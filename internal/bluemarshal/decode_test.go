package bluemarshal

import (
	"encoding/binary"
	"hash/adler32"
	"os"
	"strings"
	"testing"
)

func TestUpstreamFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	ui, ok := root.Field("ui")
	if !ok {
		t.Fatal("missing ui")
	}
	formations, ok := ui.Field("probescanning.customFormations")
	if !ok {
		t.Fatal("missing formations")
	}
	if formations.Kind != TY_TUPLE || len(formations.Items) != 2 || len(formations.Items[1].Pairs) != 3 {
		t.Fatalf("unexpected formations: %+v", formations)
	}
	if formations.Items[0].Text != "134280801871504062" {
		t.Fatal("timestamp lost precision")
	}
	found := false
	for _, pair := range formations.Items[1].Pairs {
		if id, ok := pair.Key.Integer(); ok && id == 7 {
			name, _ := pair.Value.Items[0].StringValue()
			if name != "Drifter: α 🚀" {
				t.Fatalf("name = %q", name)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("formation 7 missing")
	}
	data[len(data)-1] ^= 1
	if _, err := Decode(data); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corrupt checksum accepted: %v", err)
	}
}

func TestSharedReferences(t *testing.T) {
	for _, data := range [][]byte{
		{125, 1, 44, 83, 1, 'x', 27, 0},                      // Version 1 references are zero-based.
		{126, 1, 0, 0, 0, 44, 83, 1, 'x', 27, 1, 1, 0, 0, 0}, // Version 0 has a trailing map.
	} {
		v, err := Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Items) != 2 || v.Items[0] != v.Items[1] || v.Items[1].Text != "x" {
			t.Fatalf("unresolved reference: %+v", v)
		}
	}
}

func TestVersionZeroChecksumIncludesMapping(t *testing.T) {
	payload := []byte{83, 1, 'x', 1, 0, 0, 0}
	data := []byte{126, 1, 0, 0, 0, 28}
	data = binary.LittleEndian.AppendUint32(data, adler32.Checksum(payload))
	data = append(data, payload...)
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
	data[8] ^= 1
	if _, err := Decode(data); err == nil {
		t.Fatal("bad CRC accepted")
	}
}

func TestStringAndNumberEncodings(t *testing.T) {
	for _, test := range []struct {
		payload []byte
		want    string
	}{
		{[]byte{17, 1}, "*corpid"},
		{[]byte{18, 2, 0x3d, 0xd8, 0x80, 0xde}, "🚀"},
		{[]byte{41, 'A', 0}, "A"},
		{[]byte{16, 2, 'u', 'i'}, "ui"},
	} {
		v, err := Decode(append([]byte{125, 1}, test.payload...))
		if err != nil || v.Text != test.want {
			t.Fatalf("decode %v = %+v, %v", test.payload, v, err)
		}
	}
	v, err := Decode([]byte{125, 1, 47, 1, 0xff})
	if err != nil || v.Text != "-1" {
		t.Fatalf("signed long = %+v, %v", v, err)
	}
}

func TestMalformedStreams(t *testing.T) {
	for _, data := range [][]byte{
		nil, {0}, {125}, {125, 2, 1}, {126, 255, 255, 255, 255},
		{125, 1, 22, 255, 255, 255, 255, 127},     // Huge dictionary.
		{125, 1, 21, 255, 255, 255, 255, 255},     // Negative length.
		{125, 1, 27, 0},                           // Dangling reference.
		{125, 1, 101, 27, 0},                      // Cyclic tuple.
		{125, 1, 17, 0}, {125, 1, 18, 1, 0, 0xd8}, // Invalid strings.
		{125, 1, 26}, {125, 1, 42}, {125, 1, 43}, // Unsupported types.
		{125, 1, 1, 1}, // Trailing bytes.
	} {
		if _, err := Decode(data); err == nil {
			t.Errorf("accepted malformed stream %v", data)
		}
	}
	deep := []byte{125, 1}
	for i := 0; i < 300; i++ {
		deep = append(deep, 37)
	}
	deep = append(deep, 1)
	if _, err := Decode(deep); err == nil {
		t.Fatal("deep nesting accepted")
	}
}

func FuzzDecode(f *testing.F) {
	data, err := os.ReadFile("testdata/formations.dat")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte{125, 1, 44, 83, 1, 'x', 27, 0})
	f.Add([]byte{126, 0, 0, 0, 0, 1})
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = Decode(data) })
}
