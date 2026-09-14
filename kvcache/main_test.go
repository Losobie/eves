package kvcache

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestEntriesExcludesExpiredAndCorruptValues(t *testing.T) {
	c := &Cache{
		path:   filepath.Join(t.TempDir(), "cache.json"),
		loaded: true,
		entries: map[string]entry{
			"valid":     {Value: json.RawMessage(`{"name":"Valid"}`), ExpiresAt: time.Now().Add(time.Hour)},
			"permanent": {Value: json.RawMessage(`{"name":"Permanent"}`)},
			"expired":   {Value: json.RawMessage(`{"name":"Expired"}`), ExpiresAt: time.Now().Add(-time.Hour)},
			"corrupt":   {Value: json.RawMessage(`not json`)},
		},
	}
	values, err := Entries[struct{ Name string }](c)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values["valid"].Name != "Valid" || values["permanent"].Name != "Permanent" {
		t.Fatalf("Entries() = %v", values)
	}
}
