package bluemarshal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ToJSON preserves decoded types using the typed JSON convention of
// TrueBrain/blue-marshal-rs/src/json.rs. It never executes object callbacks.
// Shared objects are expanded; expansion is bounded independently of decoding.
func ToJSON(v *Value) (any, error) {
	w := jsonWalker{}
	return w.value(v, 0)
}

type jsonWalker struct{ nodes, size int }

func (w *jsonWalker) visit(v *Value, depth int) error {
	if v == nil {
		return fmt.Errorf("missing marshal value")
	}
	if depth > 256 || w.nodes >= maxNodes {
		return fmt.Errorf("JSON nesting or expansion limit exceeded")
	}
	w.nodes++
	// Bound repeated strings as well as repeated nodes. Account for JSON escaping
	// and wrapper overhead before json.Encoder allocates the complete document.
	w.size += 128 + 6*len(v.Text)
	if w.size > 256<<20 {
		return fmt.Errorf("JSON expansion size limit exceeded")
	}
	return nil
}

func floatText(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	default:
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
}

func bytesText(s string) string {
	if utf8.ValidString(s) && !strings.HasPrefix(s, "b64:") {
		return "bytes:" + s
	}
	return "bytes:b64:" + base64.StdEncoding.EncodeToString([]byte(s))
}

func (w *jsonWalker) items(items []*Value, depth int) ([]any, error) {
	result := make([]any, 0, len(items))
	for _, item := range items {
		value, err := w.value(item, depth)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (w *jsonWalker) value(v *Value, depth int) (any, error) {
	if err := w.visit(v, depth); err != nil {
		return nil, err
	}
	switch v.Kind {
	case TY_NONE:
		return nil, nil
	case TY_TRUE, TY_FALSE:
		return v.Kind == TY_TRUE, nil
	case TY_INT64:
		return v.Int, nil
	case TY_FLOAT:
		s := floatText(v.Float)
		if math.IsNaN(v.Float) || math.IsInf(v.Float, 0) {
			return "float:" + s, nil
		}
		if !strings.ContainsAny(s, ".eE") {
			s += ".0" // Keep integral floats distinguishable from integers.
		}
		return json.Number(s), nil
	case TY_LONG:
		return "long:" + v.Text, nil
	case TY_BUFFER:
		return bytesText(v.Text), nil
	case TY_UTF8:
		return "utf8:" + v.Text, nil
	case TY_GLOBAL:
		return "global:" + v.Text, nil
	case TY_LIST, TY_TUPLE:
		items, err := w.items(v.Items, depth+1)
		if err != nil {
			return nil, err
		}
		if v.Kind == TY_TUPLE {
			return map[string]any{"tuple": items}, nil
		}
		return items, nil
	case TY_DICT:
		result := make(map[string]any, len(v.Pairs))
		for _, pair := range v.Pairs {
			key, err := w.key(pair.Key, depth+1)
			if err != nil {
				return nil, err
			}
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("duplicate JSON dictionary key %q", key)
			}
			value, err := w.value(pair.Value, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		return result, nil
	case TY_INSTANCE:
		if len(v.Items) != 2 {
			return nil, fmt.Errorf("invalid instance")
		}
		if err := w.visit(v.Items[0], depth+1); err != nil {
			return nil, err
		}
		class, ok := v.Items[0].StringValue()
		if !ok {
			return nil, fmt.Errorf("invalid instance class")
		}
		if !utf8.ValidString(class) {
			return nil, fmt.Errorf("instance class is not valid UTF-8")
		}
		state, err := w.value(v.Items[1], depth+1)
		return map[string]any{"instance": map[string]any{"class": class, "state": state}}, err
	case TY_CALLBACK:
		if len(v.Items) != 1 {
			return nil, fmt.Errorf("invalid callback")
		}
		inner, err := w.value(v.Items[0], depth+1)
		return map[string]any{"callback": inner}, err
	case TY_REDUCE, TY_NEWOBJ:
		return w.reduce(v, depth)
	default:
		return nil, fmt.Errorf("unsupported JSON value type %d", v.Kind)
	}
}

func (w *jsonWalker) key(v *Value, depth int) (string, error) {
	if err := w.visit(v, depth); err != nil {
		return "", err
	}
	switch v.Kind {
	case TY_NONE:
		return "none", nil
	case TY_TRUE, TY_FALSE:
		return "bool:" + strconv.FormatBool(v.Kind == TY_TRUE), nil
	case TY_INT64:
		return "int:" + strconv.FormatInt(v.Int, 10), nil
	case TY_FLOAT:
		return "float:" + floatText(v.Float), nil
	case TY_LONG:
		return "long:" + v.Text, nil
	case TY_BUFFER:
		return bytesText(v.Text), nil
	case TY_UTF8:
		return "utf8:" + v.Text, nil
	case TY_GLOBAL:
		return "global:" + v.Text, nil
	default:
		value, err := w.value(v, depth)
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(value)
		return "json:" + string(data), err
	}
}

func (w *jsonWalker) reduce(v *Value, depth int) (any, error) {
	if len(v.Items) == 0 || v.Items[0] == nil || v.Items[0].Kind != TY_TUPLE {
		return nil, fmt.Errorf("invalid object payload")
	}
	payload := v.Items[0].Items
	argsIndex := 1
	if v.Kind == TY_NEWOBJ {
		argsIndex = 0
	}
	if len(payload) < argsIndex+1 || len(payload) > argsIndex+2 {
		return nil, fmt.Errorf("unsupported object payload length %d", len(payload))
	}
	inner := map[string]any{"newobj": v.Kind == TY_NEWOBJ, "callable": nil, "state": nil}
	var err error
	if argsIndex == 1 {
		inner["callable"], err = w.value(payload[0], depth+1)
		if err != nil {
			return nil, err
		}
	}
	inner["args"], err = w.value(payload[argsIndex], depth+1)
	if err != nil {
		return nil, err
	}
	if len(payload) == argsIndex+2 {
		inner["state"], err = w.value(payload[argsIndex+1], depth+1)
		if err != nil {
			return nil, err
		}
	}
	inner["list_items"], err = w.items(v.Items[1:], depth+1)
	if err != nil {
		return nil, err
	}
	pairs := make([]any, 0, len(v.Pairs))
	for _, pair := range v.Pairs {
		items, err := w.items([]*Value{pair.Key, pair.Value}, depth+1)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, items)
	}
	inner["dict_items"] = pairs
	return map[string]any{"reduce": inner}, nil
}
