package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveAndLoadAccounts(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}

	want := Accounts{"123": "Main Account", "456": "Industry Account"}
	if err := saveAccounts(want); err != nil {
		t.Fatal(err)
	}
	got, err := loadAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %d accounts, want %d", len(got), len(want))
	}
	for id, name := range want {
		if got[id] != name {
			t.Errorf("account %s = %q, want %q", id, got[id], name)
		}
	}
}

func TestScanAccountFilesIgnoresAssociatedAccounts(t *testing.T) {
	dir := t.TempDir()
	for _, profile := range []string{"settings_Default", "settings_Second"} {
		if err := os.Mkdir(filepath.Join(dir, profile), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"core_user_123.dat", "core_user_456.dat", "core_char_789.dat"} {
		if err := os.WriteFile(filepath.Join(dir, "settings_Default", name), []byte("settings"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "settings_Second", "core_user_456.dat"), []byte("settings"), 0o600); err != nil {
		t.Fatal(err)
	}

	states, err := scanAccountFiles(dir, Accounts{"123": "primary"})
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range states {
		if state.id == "123" {
			t.Fatal("associated account was returned")
		}
		if state.id != "456" {
			t.Fatalf("unexpected account ID %q", state.id)
		}
	}
	if len(states) != 2 {
		t.Fatalf("got %d account files, want 2", len(states))
	}
}

func TestChangedAccountFilesDeduplicatesIDsAcrossProfiles(t *testing.T) {
	before := time.Unix(100, 0)
	after := before.Add(time.Second)
	previous := map[string]accountFileState{
		"settings_A/core_user_1.dat": {id: "1", profile: "settings_A", modTime: before, size: 10},
		"settings_A/core_user_2.dat": {id: "2", profile: "settings_A", modTime: before, size: 10},
	}
	current := map[string]accountFileState{
		"settings_A/core_user_1.dat": {id: "1", profile: "settings_A", modTime: before, size: 10},
		"settings_A/core_user_2.dat": {id: "2", profile: "settings_A", modTime: after, size: 10},
		"settings_B/core_user_2.dat": {id: "2", profile: "settings_B", modTime: after, size: 10},
		"settings_A/core_user_3.dat": {id: "3", profile: "settings_A", modTime: before, size: 10},
	}

	changed := changedAccountFiles(previous, current)
	if len(changed) != 2 || changed[0].id != "2" || changed[1].id != "3" {
		t.Fatalf("changedAccountFiles() IDs = [%s %s], want [2 3]", changed[0].id, changed[1].id)
	}
}
