package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"losobie.com/eves/eveapi"
	"losobie.com/eves/kvcache"
)

func isolateLookupCache(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
}

func ageLookupCache(t *testing.T, file string, age time.Duration) map[string]time.Time {
	t.Helper()
	path, err := configFilePath("cache/" + file)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries map[string]struct {
		Value     json.RawMessage `json:"value"`
		ExpiresAt time.Time       `json:"expiresAt"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	expires := make(map[string]time.Time)
	for key, entry := range entries {
		entry.ExpiresAt = entry.ExpiresAt.Add(-age)
		entries[key] = entry
		expires[key] = entry.ExpiresAt
	}
	data, err = json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return expires
}

func TestLookupNameTTLAndAffiliationRefresh(t *testing.T) {
	isolateLookupCache(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "core_char_1.dat"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/characters/1/":
			fmt.Fprint(w, `{"name":"Pilot","corporation_id":11}`)
		case "/corporations/11/":
			fmt.Fprint(w, `{"name":"New Corp"}`)
		default:
			http.Error(w, "unexpected request", 500)
		}
	}))
	defer server.Close()
	s := newLookupService(server.URL+"/", "/")
	s.rememberName("Character", "Pilot", 1)
	s.rememberName("Corporation", "Old Corp", 10)
	if err := kvcache.Put(s.characters, "1", eveapi.Character{Name: "Pilot", CorporationID: 10}); err != nil {
		t.Fatal(err)
	}
	if err := kvcache.Put(s.corporations, "10", eveapi.Corporation{Name: "Old Corp"}); err != nil {
		t.Fatal(err)
	}
	expires := ageLookupCache(t, "names.json", 24*time.Hour)
	if remaining := time.Until(expires["Character:1"]); remaining < 143*time.Hour || remaining > 145*time.Hour {
		t.Fatalf("name TTL after one day = %v, want six days", remaining)
	}
	for _, file := range []string{"chars.json", "corporations.json"} {
		for _, expiry := range ageLookupCache(t, file, 24*time.Hour) {
			if time.Until(expiry) > -17*time.Hour {
				t.Fatal("details did not expire after six hours")
			}
		}
	}
	for _, args := range [][]string{{"-c"}, {"Pilot"}, {"Old Corp"}} {
		var output bytes.Buffer
		// No server available: these must use the day-old name mappings.
		if err := runLookup(dir, newLookupService("http://unused.invalid/", "/"), args, &output); err != nil {
			t.Fatal(err)
		}
	}
	if after := ageLookupCache(t, "names.json", 0); !after["Character:1"].Equal(expires["Character:1"]) {
		t.Fatal("cache reads extended name expiry")
	}
	var output bytes.Buffer
	if err := runLookup(dir, newLookupService(server.URL+"/", "/"), []string{"--corporations"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "Corporation Name: New Corp (11)\n" {
		t.Fatalf("stale affiliation: %q", output.String())
	}
}

func TestLookupRefreshBypassesAndUpdatesCaches(t *testing.T) {
	for _, args := range [][]string{{"--refresh"}, {"--characters", "--refresh"}, {"--refresh", "Pilot"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			isolateLookupCache(t)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "core_char_1.dat"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch r.URL.Path {
				case "/universe/ids/":
					fmt.Fprint(w, `{"characters":[{"id":1,"name":"Pilot"}]}`)
				case "/characters/1/":
					fmt.Fprint(w, `{"name":"Pilot","corporation_id":10,"alliance_id":20}`)
				case "/corporations/10/":
					fmt.Fprint(w, `{"name":"Corp","alliance_id":20}`)
				case "/alliances/20/":
					fmt.Fprint(w, `{"name":"Alliance"}`)
				default:
					http.Error(w, "unexpected request", 500)
				}
			}))
			defer server.Close()
			s := newLookupService(server.URL+"/", "/")
			s.rememberName("Character", "Old Name", 1)
			if err := kvcache.Put(s.characters, "1", eveapi.Character{Name: "Old Name", CorporationID: 10, AllianceID: intPointer(20)}); err != nil {
				t.Fatal(err)
			}
			if err := kvcache.Put(s.corporations, "10", eveapi.Corporation{Name: "Old Corp", AllianceID: 20}); err != nil {
				t.Fatal(err)
			}
			if err := kvcache.Put(s.alliances, "20", eveapi.Alliance{Name: "Old Alliance"}); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := runLookup(dir, s, args, &output); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "Pilot (1)") || strings.Contains(output.String(), "Old") {
				t.Fatalf("stale result: %q", output.String())
			}
			server.Close()
			wantCalls := 1
			if len(args) == 1 {
				wantCalls = 3
			}
			if calls != wantCalls {
				t.Fatalf("API calls = %d, want %d", calls, wantCalls)
			}
			output.Reset()
			if err := runLookup(dir, s, []string{"Pilot"}, &output); err != nil {
				t.Fatal(err)
			}
			if output.String() != "Character Name: Pilot (1)\n" {
				t.Fatalf("refreshed name not cached: %q", output.String())
			}
			entry, err := kvcache.Get[eveapi.NamedEntity](newLookupService("", "").names, "Character:1")
			if err != nil || entry.Name != "Pilot" {
				t.Fatalf("persisted mapping = %+v, %v", entry, err)
			}
		})
	}
}

func intPointer(value int) *int { return &value }

func TestLookupLocalModesAndCache(t *testing.T) {
	isolateLookupCache(t)
	dir := t.TempDir()
	for _, name := range []string{"core_char_1.dat", "core_char_2.dat", "core_user_99.dat", "core_char_bad.dat"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "core_char_1.dat"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	calls := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		responses := map[string]string{
			"/characters/1/":    `{"name":"Zeta","corporation_id":10,"alliance_id":20}`,
			"/characters/2/":    `{"name":"Alpha","corporation_id":10}`,
			"/corporations/10/": `{"name":"Example Corp","alliance_id":20}`,
			"/alliances/20/":    `{"name":"Example Alliance"}`,
		}
		if body, ok := responses[r.URL.Path]; ok {
			fmt.Fprint(w, body)
		} else {
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	characters := "Character Name: Alpha (2)\nCharacter Name: Zeta (1)\n"
	corporations := "Corporation Name: Example Corp (10)\n"
	alliances := "Alliance Name: Example Alliance (20)\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, characters + corporations + alliances},
		{[]string{"-a"}, characters + corporations + alliances},
		{[]string{"--all"}, characters + corporations + alliances},
		{[]string{"-c"}, characters},
		{[]string{"--characters"}, characters},
		{[]string{"--corporations"}, corporations},
		{[]string{"--alliances"}, alliances},
		{[]string{"alpha"}, "Character Name: Alpha (2)\n"},
		{[]string{"Example", "Corp"}, corporations},
		{[]string{"Example Alliance"}, alliances},
	} {
		t.Run(fmt.Sprint(test.args), func(t *testing.T) {
			var output bytes.Buffer
			// New service each time verifies persisted cache reuse across runs.
			err := runLookup(dir, newLookupService(server.URL+"/", "/?datasource=tranquility"), test.args, &output)
			if err != nil {
				t.Fatal(err)
			}
			if output.String() != test.want {
				t.Fatalf("output = %q, want %q", output.String(), test.want)
			}
		})
	}
	server.Close()
	if len(calls) != 4 {
		t.Fatalf("unexpected API calls: %v", calls)
	}
	for path, count := range calls {
		if count != 1 {
			t.Errorf("%s requested %d times, want 1", path, count)
		}
	}
}

func TestLookupNamedEntryWithoutLocalProfile(t *testing.T) {
	for _, category := range []string{"Character", "Corporation", "Alliance"} {
		t.Run(category, func(t *testing.T) {
			isolateLookupCache(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				fmt.Fprintf(w, `{"%ss":[{"id":42,"name":"Remote Name"}]}`, strings.ToLower(category))
			}))
			defer server.Close()
			for _, name := range []string{"Remote Name", "remote name"} {
				var output bytes.Buffer
				err := runLookup(filepath.Join(t.TempDir(), "missing"), newLookupService(server.URL+"/", "/"), []string{name}, &output)
				if err != nil {
					t.Fatal(err)
				}
				if want := category + " Name: Remote Name (42)\n"; output.String() != want {
					t.Fatalf("output = %q, want %q", output.String(), want)
				}
			}
			server.Close()
			if calls != 1 {
				t.Fatalf("API called %d times, want 1", calls)
			}
		})
	}
}

func TestLookupExpiredCacheAndAPIFailure(t *testing.T) {
	isolateLookupCache(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "core_char_1.dat"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cache := kvcache.New("cache/chars.json", time.Nanosecond)
	if err := kvcache.Put(cache, "1", eveapi.Character{Name: "Expired"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, `{"error":"unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"name":"Fresh"}`)
	}))
	defer server.Close()
	var output bytes.Buffer
	err := runLookup(dir, newLookupService(server.URL+"/", "/"), []string{"-c"}, &output)
	if err == nil || !strings.Contains(err.Error(), "503") || output.Len() != 0 {
		t.Fatalf("first lookup = %q, %v; want HTTP 503 and no output", output.String(), err)
	}
	err = runLookup(dir, newLookupService(server.URL+"/", "/"), []string{"-c"}, &output)
	if err != nil || output.String() != "Character Name: Fresh (1)\n" {
		t.Fatalf("retry lookup = %q, %v", output.String(), err)
	}
}

func TestLookupInvalidArgumentsAndMissingDirectory(t *testing.T) {
	isolateLookupCache(t)
	service := newLookupService("http://unused.invalid/", "/")
	for _, args := range [][]string{{"--unknown"}, {"-a", "-c"}, {"--characters", "Name"}, {"Name", "--all"}, {" "}, nil} {
		var output bytes.Buffer
		if err := runLookup(filepath.Join(t.TempDir(), "missing"), service, args, &output); err == nil {
			t.Errorf("runLookup(%v) succeeded, want error", args)
		}
	}
}

func TestLookupFiltersFetchOnlyNeededEntities(t *testing.T) {
	for _, test := range []struct {
		flag, character, corporation, want string
		wantCalls                          int
	}{
		{"--characters", `{"name":"Pilot","corporation_id":10,"alliance_id":20}`, "", "Character Name: Pilot (1)\n", 1},
		{"--corporations", `{"name":"Pilot","corporation_id":10,"alliance_id":20}`, `{"name":"Corp","alliance_id":20}`, "Corporation Name: Corp (10)\n", 2},
		{"--alliances", `{"name":"Pilot","corporation_id":10}`, `{"name":"Corp","alliance_id":20}`, "Alliance Name: Alliance (20)\n", 3},
		{"--alliances", `{"name":"Pilot","corporation_id":10}`, `{"name":"Corp"}`, "", 2},
	} {
		t.Run(test.flag+test.want, func(t *testing.T) {
			isolateLookupCache(t)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "core_char_1.dat"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch r.URL.Path {
				case "/characters/1/":
					fmt.Fprint(w, test.character)
				case "/corporations/10/":
					fmt.Fprint(w, test.corporation)
				case "/alliances/20/":
					fmt.Fprint(w, `{"name":"Alliance"}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			var output bytes.Buffer
			if err := runLookup(dir, newLookupService(server.URL+"/", "/"), []string{test.flag}, &output); err != nil {
				t.Fatal(err)
			}
			server.Close()
			if output.String() != test.want || calls != test.wantCalls {
				t.Fatalf("output = %q, calls = %d; want %q, %d", output.String(), calls, test.want, test.wantCalls)
			}
		})
	}
}
