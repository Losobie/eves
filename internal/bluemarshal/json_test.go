package bluemarshal

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func jsonDocument(t *testing.T, data []byte) any {
	t.Helper()
	var value any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestJSONMatchesUpstreamFixtures(t *testing.T) {
	for _, name := range []string{"formations", "empty"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + name + ".dat")
			if err != nil {
				t.Fatal(err)
			}
			root, err := Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			value, err := ToJSON(root)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile("testdata/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(jsonDocument(t, actual), jsonDocument(t, want)) {
				t.Fatalf("JSON differs from upstream:\n%s", actual)
			}
		})
	}
}

func TestJSONTypes(t *testing.T) {
	for _, test := range []struct {
		value *Value
		want  string
	}{
		{&Value{Kind: TY_NONE}, `null`},
		{&Value{Kind: TY_TRUE}, `true`},
		{&Value{Kind: TY_FALSE}, `false`},
		{&Value{Kind: TY_INT64, Int: math.MaxInt64}, `9223372036854775807`},
		{&Value{Kind: TY_FLOAT, Float: 1}, `1.0`},
		{&Value{Kind: TY_FLOAT, Float: math.Copysign(0, -1)}, `-0.0`},
		{&Value{Kind: TY_FLOAT, Float: math.NaN()}, `"float:nan"`},
		{&Value{Kind: TY_FLOAT, Float: math.Inf(1)}, `"float:inf"`},
		{&Value{Kind: TY_FLOAT, Float: math.Inf(-1)}, `"float:-inf"`},
		{&Value{Kind: TY_LONG, Text: "999999999999999999999999"}, `"long:999999999999999999999999"`},
		{&Value{Kind: TY_BUFFER, Text: "hello"}, `"bytes:hello"`},
		{&Value{Kind: TY_BUFFER, Text: "\xff\x00"}, `"bytes:b64:/wA="`},
		{&Value{Kind: TY_BUFFER, Text: "b64:x"}, `"bytes:b64:YjY0Ong="`},
		{&Value{Kind: TY_UTF8, Text: "🚀"}, `"utf8:🚀"`},
		{&Value{Kind: TY_GLOBAL, Text: "util.KeyVal"}, `"global:util.KeyVal"`},
		{&Value{Kind: TY_LIST}, `[]`},
		{&Value{Kind: TY_TUPLE}, `{"tuple":[]}`},
		{&Value{Kind: TY_DICT}, `{}`},
		{&Value{Kind: TY_CALLBACK, Items: []*Value{{Kind: TY_INT64, Int: 7}}}, `{"callback":7}`},
	} {
		value, err := ToJSON(test.value)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil || string(data) != test.want {
			t.Errorf("type %d: got %s, %v; want %s", test.value.Kind, data, err, test.want)
		}
	}
}

func TestJSONDictionaryKeys(t *testing.T) {
	keys := []*Value{
		{Kind: TY_NONE}, {Kind: TY_TRUE}, {Kind: TY_FALSE},
		{Kind: TY_INT64, Int: 1}, {Kind: TY_FLOAT, Float: 1},
		{Kind: TY_LONG, Text: "1"}, {Kind: TY_BUFFER, Text: "1"},
		{Kind: TY_UTF8, Text: "1"}, {Kind: TY_GLOBAL, Text: "1"},
		{Kind: TY_TUPLE, Items: []*Value{{Kind: TY_INT64, Int: 1}}},
	}
	dict := &Value{Kind: TY_DICT}
	for _, key := range keys {
		dict.Pairs = append(dict.Pairs, Pair{key, &Value{Kind: TY_NONE}})
	}
	value, err := ToJSON(dict)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"none", "bool:true", "bool:false", "int:1", "float:1", "long:1", "bytes:1", "utf8:1", "global:1", `json:{"tuple":[1]}`} {
		if _, ok := value.(map[string]any)[key]; !ok {
			t.Errorf("missing key %s", key)
		}
	}
	dict.Pairs = append(dict.Pairs, dict.Pairs[0])
	if _, err := ToJSON(dict); err == nil {
		t.Fatal("duplicate dictionary key silently discarded")
	}
}

func TestJSONObjectWrappers(t *testing.T) {
	for _, kind := range []byte{TY_REDUCE, TY_NEWOBJ} {
		payload := []*Value{{Kind: TY_TUPLE}, {Kind: TY_DICT}}
		if kind == TY_REDUCE {
			payload = append([]*Value{{Kind: TY_GLOBAL, Text: "util.KeyVal"}}, payload...)
		}
		v := &Value{Kind: kind, Items: []*Value{{Kind: TY_TUPLE, Items: payload}, {Kind: TY_INT64, Int: 7}}, Pairs: []Pair{{&Value{Kind: TY_BUFFER, Text: "key"}, &Value{Kind: TY_TRUE}}}}
		value, err := ToJSON(v)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(value)
		callable := `"global:util.KeyVal"`
		newobj := "false"
		if kind == TY_NEWOBJ {
			callable, newobj = "null", "true"
		}
		want := `{"reduce":{"args":{"tuple":[]},"callable":` + callable + `,"dict_items":[["bytes:key",true]],"list_items":[7],"newobj":` + newobj + `,"state":{}}}`
		if string(data) != want {
			t.Fatalf("got %s, want %s", data, want)
		}
		v.Items[0].Items = append(payload, &Value{Kind: TY_NONE})
		if _, err := ToJSON(v); err == nil {
			t.Fatal("extra payload fields silently discarded")
		}
	}
}

func TestJSONExpansionLimits(t *testing.T) {
	v := &Value{Kind: TY_NONE}
	for i := 0; i < 20; i++ {
		v = &Value{Kind: TY_LIST, Items: []*Value{v, v}}
	}
	if _, err := ToJSON(v); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("unbounded reference expansion: %v", err)
	}
	v = &Value{Kind: TY_LIST}
	v.Items = []*Value{v}
	if _, err := ToJSON(v); err == nil {
		t.Fatal("cyclic value accepted")
	}
	v = &Value{Kind: TY_BUFFER, Text: strings.Repeat("x", 1<<20)}
	for i := 0; i < 7; i++ {
		v = &Value{Kind: TY_LIST, Items: []*Value{v, v}}
	}
	if _, err := ToJSON(v); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("unbounded string expansion: %v", err)
	}
}

func TestPlainJSONFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	value, err := ToPlainJSON(root)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ui":{
		"probescanning.customFormations":[134280801871504062,{
			"7":["Drifter: α 🚀",[[[250000,0,0],37399467675],[[-250000,0,0],37399467675]]],
			"2":["Pinpoint",[[[0,500000,0],37399467675]]],
			"-4":["tempFormation",[]]
		}],
		"probescanning.selectedFormationID":[134280801871504062,7],
		"unrelated":[9999999999999999999999999999,{
			"bool":true,"float":1.5,"negative":-40000,
			"long":-99999999999999999999999999,"instance":{"test":null}
		}]
	}}`
	if !reflect.DeepEqual(jsonDocument(t, actual), jsonDocument(t, []byte(want))) {
		t.Fatalf("plain fixture:\n%s", actual)
	}
}

func TestPlainJSONTypes(t *testing.T) {
	for _, test := range []struct {
		value *Value
		want  string
	}{
		{&Value{Kind: TY_LONG, Text: "999999999999999999999999"}, `999999999999999999999999`},
		{&Value{Kind: TY_BUFFER, Text: "bytes:literal"}, `"bytes:literal"`},
		{&Value{Kind: TY_BUFFER, Text: "b64:literal"}, `"b64:literal"`},
		{&Value{Kind: TY_BUFFER, Text: "\xff\x00"}, `"/wA="`},
		{&Value{Kind: TY_UTF8, Text: "utf8:🚀"}, `"utf8:🚀"`},
		{&Value{Kind: TY_GLOBAL, Text: "util.KeyVal"}, `"util.KeyVal"`},
		{&Value{Kind: TY_FLOAT, Float: math.NaN()}, `"nan"`},
		{&Value{Kind: TY_FLOAT, Float: math.Inf(1)}, `"inf"`},
		{&Value{Kind: TY_FLOAT, Float: math.Inf(-1)}, `"-inf"`},
		{&Value{Kind: TY_TUPLE}, `[]`},
		{&Value{Kind: TY_LIST}, `[]`},
		{&Value{Kind: TY_DICT}, `{}`},
		{&Value{Kind: TY_INSTANCE, Items: []*Value{{Kind: TY_BUFFER, Text: "Class"}, {Kind: TY_TRUE}}}, `true`},
		{&Value{Kind: TY_CALLBACK, Items: []*Value{{Kind: TY_INT64, Int: 7}}}, `7`},
		{&Value{Kind: TY_NEWOBJ, Items: []*Value{{Kind: TY_TUPLE, Items: []*Value{{Kind: TY_TUPLE}}}}}, `{"args":[],"callable":null,"dict_items":[],"list_items":[],"state":null}`},
		{&Value{Kind: TY_REDUCE, Items: []*Value{{Kind: TY_TUPLE, Items: []*Value{{Kind: TY_GLOBAL, Text: "Class"}, {Kind: TY_TUPLE}, {Kind: TY_DICT}}}}}, `{"args":[],"callable":"Class","dict_items":[],"list_items":[],"state":{}}`},
	} {
		value, err := ToPlainJSON(test.value)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil || string(data) != test.want {
			t.Errorf("type %d: got %s, %v; want %s", test.value.Kind, data, err, test.want)
		}
	}
}

func TestPlainJSONKeys(t *testing.T) {
	for _, keys := range [][]*Value{
		{{Kind: TY_INT64, Int: 1}, {Kind: TY_UTF8, Text: "1"}},
		{{Kind: TY_BUFFER, Text: "same"}, {Kind: TY_UTF8, Text: "same"}},
		{{Kind: TY_NONE}, {Kind: TY_BUFFER, Text: "null"}},
	} {
		v := &Value{Kind: TY_DICT}
		for _, key := range keys {
			v.Pairs = append(v.Pairs, Pair{key, &Value{Kind: TY_TRUE}})
		}
		if _, err := ToJSON(v); err != nil {
			t.Fatalf("distinct typed keys rejected: %v", err)
		}
		if _, err := ToPlainJSON(v); err == nil || !strings.Contains(err.Error(), "collide") {
			t.Fatalf("plain key collision accepted: %v", err)
		}
	}
	v := &Value{Kind: TY_DICT, Pairs: []Pair{
		{&Value{Kind: TY_TUPLE, Items: []*Value{{Kind: TY_INT64, Int: 1}, {Kind: TY_BUFFER, Text: "x"}}}, &Value{Kind: TY_TRUE}},
		{&Value{Kind: TY_FALSE}, &Value{Kind: TY_NONE}},
	}}
	value, err := ToPlainJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) != `{"[1,\"x\"]":true,"false":null}` {
		t.Fatalf("plain keys = %s, %v", data, err)
	}
}
