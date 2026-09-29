package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadProbeFormations(t *testing.T) {
	formations, err := readProbeFormations("internal/bluemarshal/testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	want := []probeFormation{
		{ID: 2, Name: "Pinpoint", Probes: []formationProbe{{Y: 500000, Range: 37399467675}}},
		{ID: 7, Name: "Drifter: α 🚀", Probes: []formationProbe{{X: 250000, Range: 37399467675}, {X: -250000, Range: 37399467675}}},
	}
	if !reflect.DeepEqual(formations, want) {
		t.Fatalf("formations = %+v, want %+v", formations, want)
	}
	formations, err = readProbeFormations("internal/bluemarshal/testdata/empty.dat")
	if err != nil || len(formations) != 0 {
		t.Fatalf("empty = %+v, %v", formations, err)
	}
}

func TestFormationListAcrossAccountsAndProfiles(t *testing.T) {
	isolateLookupCache(t)
	if err := saveAccounts(Accounts{"123": "Primary"}); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	fixture, err := os.ReadFile("internal/bluemarshal/testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, relative := range []string{"settings_Default/core_user_123.dat", "settings_PvP/core_user_456.dat", "settings_PvP/core_user_123.dat", "backup/core_user_999.dat", "settings_Default/core_char_999.dat"} {
		path := filepath.Join(base, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, fixture, 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, path)
	}
	for _, test := range []struct {
		args []string
		rows int
	}{
		{[]string{"list"}, 6}, {[]string{"list", "Primary@PvP"}, 2}, {[]string{"list", "123@1"}, 2}, {[]string{"list", "123"}, 2},
	} {
		var out bytes.Buffer
		if err := runFormation(base, test.args, &out); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != test.rows+1 || strings.Count(out.String(), "Pinpoint") != test.rows/2 || strings.Contains(out.String(), "tempFormation") || strings.Contains(out.String(), "999") {
			t.Fatalf("%v output = %q", test.args, out.String())
		}
		if !strings.Contains(out.String(), "Primary") {
			t.Fatalf("account alias missing: %q", out.String())
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, fixture) {
			t.Fatalf("settings modified: %s", path)
		}
	}
	// A corrupt file does not prevent valid accounts from being listed.
	bad := filepath.Join(base, "settings_Default/core_user_789.dat")
	if err := os.WriteFile(bad, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = runFormation(base, []string{"list"}, &out)
	if err == nil || !strings.Contains(err.Error(), "core_user_789.dat") || strings.Count(out.String(), "Pinpoint") != 3 {
		t.Fatalf("partial listing = %q, %v", out.String(), err)
	}
}

func TestFormationListEmptyAndInvalidArguments(t *testing.T) {
	isolateLookupCache(t)
	base := t.TempDir()
	var out bytes.Buffer
	if err := runFormation(base, []string{"list"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "No custom probe formations found.\n" {
		t.Fatalf("empty output = %q", out.String())
	}
	for _, args := range [][]string{nil, {"delete"}, {"list", "1", "2"}, {"list", "Unknown"}, {"list", "123"}, {"list", "123@99"}} {
		out.Reset()
		if err := runFormation(base, args, &out); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestFormationCoordinateSelector(t *testing.T) {
	isolateLookupCache(t)
	if err := saveAccounts(Accounts{"123": "Alpha"}); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	data, err := os.ReadFile("internal/bluemarshal/testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"Default", "PvP"} {
		dir := filepath.Join(base, "settings_"+profile)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "core_user_123.dat"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, ref := range []string{"Alpha@Default", "123@1", "Alpha"} {
		var out bytes.Buffer
		if err := runFormation(base, []string{"list", ref, "drifter: α 🚀"}, &out); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 6 || strings.Contains(out.String(), "Pinpoint") {
			t.Fatalf("unexpected output: %q", out.String())
		}
		if !strings.Contains(lines[3], "X (km)") || !strings.Contains(lines[3], "Y (km)") || !strings.Contains(lines[3], "Z (km)") || !strings.Contains(lines[3], "RANGE (AU)") {
			t.Fatalf("missing units: %s", lines[3])
		}
		if got := strings.Fields(lines[4]); !reflect.DeepEqual(got, []string{"1", "250", "0", "0", "0.25"}) {
			t.Fatalf("first probe = %v", got)
		}
		if got := strings.Fields(lines[5]); !reflect.DeepEqual(got, []string{"2", "-250", "0", "0", "0.25"}) {
			t.Fatalf("second probe = %v", got)
		}
	}
	for _, args := range [][]string{
		{"list", "Alpha@Default", "missing"},
		{"list", "Alpha", ""},
		{"list", "", "Pinpoint"},
		{"list", "Alpha", "tempFormation"},
		{"list", "Alpha@Default:Drifter: α 🚀"},
		{"list", "Alpha@Default", ":Drifter: α 🚀"},
		{"list", "Alpha", "Pinpoint", "extra"},
	} {
		var out bytes.Buffer
		if err := runFormation(base, args, &out); err == nil || out.Len() != 0 {
			t.Fatalf("arguments %q returned %q, %v", args, out.String(), err)
		}
	}
	for _, profile := range []string{"Default", "PvP"} {
		after, err := os.ReadFile(filepath.Join(base, "settings_"+profile, "core_user_123.dat"))
		if err != nil || !bytes.Equal(data, after) {
			t.Fatalf("settings modified: %s, %v", profile, err)
		}
	}
}

func TestFormationAmbiguousNameAndEmptyProbes(t *testing.T) {
	_, err := selectProbeFormation([]probeFormation{{ID: 1, Name: "temp"}, {ID: 2, Name: "TEMP"}}, "temp")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguity error = %v", err)
	}
	var out bytes.Buffer
	if err := printFormationProbes(&out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No probes") {
		t.Fatalf("empty probes = %q", out.String())
	}
	out.Reset()
	if err := printFormationProbes(&out, []formationProbe{{X: 125.5, Y: -2500, Z: 3750, Range: 149597870700}}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if got := strings.Fields(lines[1]); !reflect.DeepEqual(got, []string{"1", "0.1255", "-2.5", "3.75", "1"}) {
		t.Fatalf("converted coordinates = %v", got)
	}
}

func TestFormationJSONOutput(t *testing.T) {
	isolateLookupCache(t)
	if err := saveAccounts(Accounts{"123": "Alpha"}); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	dir := filepath.Join(base, "settings_Default")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("internal/bluemarshal/testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "core_user_123.dat")
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"list", "Alpha@Default", "Drifter: α 🚀", "-o", "json"},
		{"list", "-o", "json", "Alpha", "drifter: α 🚀"},
		{"list", "123", "Drifter: α 🚀", "--output", "json"},
	} {
		var out bytes.Buffer
		if err := runFormation(base, args, &out); err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("output isn't standalone JSON: %v\n%s", err, out.String())
		}
		want := map[string]any{
			"version": float64(1), "name": "Drifter: α 🚀",
			"probes": []any{
				[]any{float64(250), float64(0), float64(0), 0.25},
				[]any{float64(-250), float64(0), float64(0), 0.25},
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("export = %#v, want %#v", got, want)
		}
		if !strings.HasPrefix(out.String(), "{\n  ") || !strings.HasSuffix(out.String(), "\n") {
			t.Fatalf("output not pretty printed: %q", out.String())
		}
	}
	for _, args := range [][]string{
		{"list", "-o", "json"}, {"list", "Alpha", "-o", "json"},
		{"list", "Alpha", "Pinpoint", "-o"}, {"list", "Alpha", "Pinpoint", "-o", "yaml"},
		{"list", "Alpha", "Pinpoint", "-o", "json", "-o", "json"},
		{"list", "Alpha", "Missing", "-o", "json"}, {"list", "Alpha", "", "-o", "json"},
	} {
		var out bytes.Buffer
		if err := runFormation(base, args, &out); err == nil || out.Len() != 0 {
			t.Fatalf("%v: error %v, output %q", args, err, out.String())
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, fixture) {
		t.Fatalf("JSON export changed settings: %v", err)
	}
}

func TestFormationJSONEmptyAndFractionalCoordinates(t *testing.T) {
	for _, probes := range [][]formationProbe{nil, {{X: 125.5, Y: -2500, Z: 3750, Range: 149597870700}}} {
		var out bytes.Buffer
		if err := printFormationJSON(&out, probeFormation{Name: "Name \"quoted\"\n<&>", Probes: probes}); err != nil {
			t.Fatal(err)
		}
		var got struct {
			Name   string      `json:"name"`
			Probes [][]float64 `json:"probes"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Name != "Name \"quoted\"\n<&>" || got.Probes == nil || len(got.Probes) != len(probes) {
			t.Fatalf("unexpected export: %s", out.String())
		}
		if len(probes) > 0 && !reflect.DeepEqual(got.Probes[0], []float64{0.1255, -2.5, 3.75, 1}) {
			t.Fatalf("coordinates = %s", out.String())
		}
	}
}
