package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"losobie.com/eves/eveapi"
	"losobie.com/eves/kvcache"
)

func TestSettingsExportSources(t *testing.T) {
	isolateLookupCache(t)
	base := t.TempDir()
	for _, profile := range []string{"Default", "PvP"} {
		dir := filepath.Join(base, "settings_"+profile)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"core_user_12.dat", "core_char_34.dat", "core_char_78.dat", "core_user_56.dat", "core_char_56.dat"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte{125, 1, 1}, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	accounts := Accounts{"12": "Alpha", "56": "Shared"}
	service := newLookupService("http://unused.invalid/", "/")
	service.rememberName("Character", "Pilot", 34)
	service.rememberName("Character", "Alpha", 78)
	if err := kvcache.Put(service.characters, "56", eveapi.Character{Name: "Detailed Pilot"}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		source, kind, profile, filename string
	}{
		{"12", "", "Default", "core_user_12.dat"},
		{"34", "", "Default", "core_char_34.dat"},
		{"alpha", "", "Default", "core_user_12.dat"},
		{"Alpha", "account", "Default", "core_user_12.dat"},
		{"Alpha", "character", "Default", "core_char_78.dat"},
		{"Pilot@PvP", "", "PvP", "core_char_34.dat"},
		{"Pilot@settings_PvP", "", "PvP", "core_char_34.dat"},
		{"Alpha@1", "", "PvP", "core_user_12.dat"},
		{"12@0", "account", "Default", "core_user_12.dat"},
		{"34@PvP", "character", "PvP", "core_char_34.dat"},
		{"56", "account", "Default", "core_user_56.dat"},
		{"56", "character", "Default", "core_char_56.dat"},
		{"Detailed Pilot", "character", "Default", "core_char_56.dat"},
	} {
		t.Run(test.source+test.kind, func(t *testing.T) {
			path, err := resolveSettingsSource(base, accounts, service, settingsExportOptions{test.source, test.kind})
			want := filepath.Join(base, "settings_"+test.profile, test.filename)
			if err != nil || path != want {
				t.Fatalf("got %s, %v; want %s", path, err, want)
			}
			var output bytes.Buffer
			if err := exportSettingsFile(path, &output); err != nil || output.String() != "null\n" {
				t.Fatalf("export = %q, %v", output.String(), err)
			}
		})
	}
	accounts["78"] = "Alpha"
	for _, test := range []struct{ source, kind, errorText string }{
		{"56", "", "both account and character"},
		{"999", "", "no account or character"},
		{"Pilot@99", "", "profile index"},
		{"Alpha", "", "ambiguous"},
		{"Missing", "account", "account not found"},
		{"Pilot@", "", "invalid settings reference"},
		{"Pilot@../PvP", "", "invalid settings reference"},
	} {
		_, err := resolveSettingsSource(base, accounts, service, settingsExportOptions{test.source, test.kind})
		if err == nil || !strings.Contains(err.Error(), test.errorText) {
			t.Errorf("%s: got %v, want %s", test.source, err, test.errorText)
		}
	}
}

func TestSettingsFreshCharacterLookup(t *testing.T) {
	isolateLookupCache(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/universe/ids") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(404)
			return
		}
		var names []string
		if err := json.NewDecoder(r.Body).Decode(&names); err != nil || !reflect.DeepEqual(names, []string{"Fresh Pilot"}) {
			t.Errorf("unexpected names %v, %v", names, err)
		}
		fmt.Fprint(w, `{"characters":[{"id":34,"name":"Fresh Pilot"}]}`)
	}))
	defer server.Close()
	service := newLookupService(server.URL+"/", "/")
	base := t.TempDir()
	for i := 0; i < 2; i++ {
		path, err := resolveSettingsSource(base, Accounts{}, service, settingsExportOptions{source: "Fresh Pilot@PvP"})
		if err != nil || path != filepath.Join(base, "settings_PvP", "core_char_34.dat") {
			t.Fatalf("lookup = %s, %v", path, err)
		}
	}
	if calls != 1 {
		t.Fatalf("expected cached second lookup, calls = %d", calls)
	}
	service.rememberName("Corporation", "A Corp", 55)
	if _, err := resolveSettingsSource(base, Accounts{}, service, settingsExportOptions{source: "A Corp"}); err == nil || !strings.Contains(err.Error(), "not a character") {
		t.Fatalf("corporation accepted: %v", err)
	}
}

func TestSettingsReferenceExport(t *testing.T) {
	isolateLookupCache(t)
	cacheDir := t.TempDir()
	t.Setenv("LocalAppData", cacheDir)
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	if err := SaveConfig(&Config{Server: "http://unused.invalid/", ServerSuffix: "/", EveEnv: "test-env"}); err != nil {
		t.Fatal(err)
	}
	if err := saveAccounts(Accounts{"12": "Alpha"}); err != nil {
		t.Fatal(err)
	}
	service := newLookupService("http://unused.invalid/", "/")
	service.rememberName("Character", "Pilot", 34)
	data, err := os.ReadFile("internal/bluemarshal/testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"Default", "PvP"} {
		dir := filepath.Join(cacheDir, "CCP", "EVE", "test-env", "settings_"+profile)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"core_user_12.dat", "core_char_34.dat"} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, source := range []string{"12", "34", "Alpha", "pilot", "Alpha@PvP", "34@1"} {
		var output bytes.Buffer
		if err := runExport([]string{source}, &output); err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if !json.Valid(output.Bytes()) || !strings.Contains(output.String(), "bytes:unrelated") {
			t.Fatalf("%s: incomplete export %s", source, output.String())
		}
	}
	// Character IDs work without reading account aliases, even with a broken map.
	accountsPath, err := configFilePath("accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accountsPath, []byte("invalid JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runExport([]string{"34"}, &output); err != nil {
		t.Fatal(err)
	}
	if err := runExport([]string{"Pilot", "--character"}, &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := runExport([]string{"34@Missing"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("missing profile: output %q, error %v", output.String(), err)
	}
}

func TestSettingsArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"--account"}, {"a", "b"}, {"a", "--bogus"}, {"--account", "--character", "a"}, {" "}} {
		if _, err := parseSettingsExport(args); err == nil {
			t.Errorf("accepted invalid arguments %v", args)
		}
	}
	for _, args := range [][]string{{"--account", "Alpha@PvP"}, {"Alpha@PvP", "--account"}} {
		if opts, err := parseSettingsExport(args); err != nil || opts != (settingsExportOptions{"Alpha@PvP", "account"}) {
			t.Errorf("%v: got %+v, %v", args, opts, err)
		}
	}
	if opts, err := parseSettingsExport([]string{"--", "-file.dat"}); err != nil || opts.source != "-file.dat" {
		t.Fatalf("end of options = %+v, %v", opts, err)
	}
}

func TestSettingsFileExportBeforeConfig(t *testing.T) {
	if os.Getenv("EVES_TEST_SETTINGS_PROCESS") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"eves"}, os.Args[i+1:]...)
				main()
				return
			}
		}
		t.Fatal("missing helper args")
	}
	isolateLookupCache(t)
	configPath, err := configFilePath("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("invalid JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("internal/bluemarshal/testdata/formations.dat")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "settings @file.dat")
	if err := os.WriteFile(source, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVES_TEST_SETTINGS_PROCESS", "1")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSettingsFileExportBeforeConfig$", "--", "export", source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("export: %v, %s", err, stderr.String())
	}
	// The helper test binary prints PASS after main returns; isolate the document.
	data := bytes.TrimSuffix(stdout.Bytes(), []byte("PASS\n"))
	want, err := os.ReadFile("internal/bluemarshal/testdata/formations.json")
	if err != nil {
		t.Fatal(err)
	}
	var actualValue, wantValue any
	if err := json.Unmarshal(data, &actualValue); err != nil {
		t.Fatalf("invalid JSON: %v, %s", err, data)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil || !reflect.DeepEqual(actualValue, wantValue) {
		t.Fatalf("full-file export differs from fixture: %v", err)
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(after, fixture) {
		t.Fatal("source changed")
	}
	after, err = os.ReadFile(configPath)
	if err != nil || string(after) != "invalid JSON" {
		t.Fatal("config changed")
	}
}

func TestSettingsExportErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.dat")
	if err := os.WriteFile(bad, []byte("not marshal"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{bad, filepath.Join(dir, "missing.dat"), dir} {
		var output bytes.Buffer
		if err := runExport([]string{path}, &output); err == nil || output.Len() != 0 {
			t.Fatalf("bad source %s: output %q, error %v", path, output.String(), err)
		}
	}
	for _, source := range []string{bad, filepath.Join(dir, "missing.dat"), "missing.dat", "folder/missing"} {
		if ok, err := isSettingsPath(source); err != nil || !ok {
			t.Errorf("path %s: got %v, %v", source, ok, err)
		}
	}
	var output bytes.Buffer
	if err := runExport([]string{bad, "--account"}, &output); err == nil {
		t.Fatal("type option accepted with path")
	}
	if err := runExport([]string{"internal/bluemarshal/testdata/empty.dat"}, failedSettingsWriter{}); err == nil || !strings.Contains(err.Error(), "write settings JSON") {
		t.Fatalf("write failure: %v", err)
	}
}

type failedSettingsWriter struct{}

func (failedSettingsWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("test write failure") }
