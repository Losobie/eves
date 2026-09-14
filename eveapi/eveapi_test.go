package eveapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookupName(t *testing.T) {
	for _, test := range []struct {
		name, body, wantCategory, wantError string
		status                              int
	}{
		{"character", `{"characters":[{"id":1,"name":"Example Name"}]}`, "Character", "", 200},
		{"corporation", `{"corporations":[{"id":1,"name":"Example Name"}]}`, "Corporation", "", 200},
		{"alliance", `{"alliances":[{"id":1,"name":"Example Name"}]}`, "Alliance", "", 200},
		{"not found", `{}`, "", "not found", 200},
		{"other category", `{"inventory_types":[{"id":1,"name":"Example Name"}]}`, "", "not found", 200},
		{"nonexact", `{"characters":[{"id":1,"name":"Example Name Extra"}]}`, "", "not found", 200},
		{"ambiguous", `{"characters":[{"id":1,"name":"Example Name"}],"alliances":[{"id":2,"name":"Example Name"}]}`, "", "ambiguous", 200},
		{"malformed", `not json`, "", "decode", 200},
		{"API error", `{"error":"unavailable"}`, "", "503", 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/universe/ids/" || r.URL.Query().Get("datasource") != "tranquility" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing JSON content type")
				}
				var names []string
				if err := json.NewDecoder(r.Body).Decode(&names); err != nil || len(names) != 1 || names[0] != "example name" {
					t.Errorf("request names = %v, error = %v", names, err)
				}
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			entry, err := NewApi(server.URL+"/", "/?datasource=tranquility").LookupName("example name")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil || entry.ID != 1 || entry.Name != "Example Name" || entry.Category != test.wantCategory {
				t.Fatalf("entry = %+v, error = %v", entry, err)
			}
		})
	}
}

func TestEntityLookupsRejectHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()
	api := NewApi(server.URL+"/", "/")
	_, characterErr := api.LookupCharacter(1)
	_, corporationErr := api.LookupCorporation(1)
	_, allianceErr := api.LookupAlliance(1)
	for _, err := range []error{characterErr, corporationErr, allianceErr} {
		if err == nil || !strings.Contains(err.Error(), "404") {
			t.Errorf("error = %v, want HTTP 404", err)
		}
	}
}
