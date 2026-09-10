package main

import (
	"bytes"
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

func TestListAccountsIncludesNamedAndUnnamedAccounts(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}

	dir := t.TempDir()
	for _, profile := range []string{"settings_Default", "settings_Second"} {
		if err := os.Mkdir(filepath.Join(dir, profile), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(dir, "settings_Default", "core_user_12345.dat"),
		filepath.Join(dir, "settings_Default", "core_user_67890.dat"),
		filepath.Join(dir, "settings_Second", "core_user_12345.dat"),
	} {
		if err := os.WriteFile(path, []byte("settings"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveAccounts(Accounts{"12345": "Name"}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := listAccounts(dir, &output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "12345 (Name)\n67890\n"; got != want {
		t.Fatalf("listAccounts() output = %q, want %q", got, want)
	}
}

func TestSetAccountNameCreatesAndReplacesAssignment(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}

	if err := setAccountName("12345", " First Name "); err != nil {
		t.Fatal(err)
	}
	if err := setAccountName("12345", "Second Name"); err != nil {
		t.Fatal(err)
	}

	accounts, err := loadAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := accounts["12345"], "Second Name"; got != want {
		t.Fatalf("account name = %q, want %q", got, want)
	}
}

func TestSetAccountNameRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "non-numeric ID", id: "abc", want: "Name"},
		{name: "empty name", id: "12345", want: "  "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := setAccountName(test.id, test.want); err == nil {
				t.Fatal("setAccountName() succeeded, want error")
			}
		})
	}
}

func TestResolveAccountIDByIDOrName(t *testing.T) {
	accounts := Accounts{"12345": "Primary Account", "67890": "Secondary Account"}
	tests := []struct {
		ref  string
		want string
	}{
		{ref: "12345", want: "12345"},
		{ref: "Primary Account", want: "12345"},
		{ref: "secondary account", want: "67890"},
	}
	for _, test := range tests {
		t.Run(test.ref, func(t *testing.T) {
			got, err := resolveAccountID(accounts, test.ref)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("resolveAccountID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveAccountIDRejectsAmbiguousName(t *testing.T) {
	accounts := Accounts{"12345": "Shared", "67890": "shared"}
	if _, err := resolveAccountID(accounts, "Shared"); err == nil {
		t.Fatal("resolveAccountID() succeeded, want ambiguity error")
	}
}

func TestCopyAccountSettingsUsingNamesAndIDs(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}
	if err := saveAccounts(Accounts{"12345": "Primary", "67890": "Secondary"}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	profileDir := filepath.Join(dir, "settings_Default")
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(profileDir, "core_user_12345.dat")
	targetPath := filepath.Join(profileDir, "core_user_67890.dat")
	if err := os.WriteFile(sourcePath, []byte("source settings"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("target settings"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyAccountSettings(dir, "settings_Default", "Primary", "67890"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "source settings"; string(got) != want {
		t.Fatalf("target settings = %q, want %q", got, want)
	}
}

func TestCopyAccountSettingsProfiles(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}
	if err := saveAccounts(Accounts{"12345": "Primary", "67890": "Secondary"}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, source, target, targetProfile, targetID string
		wantError                                     bool
	}{
		{"same account across profiles", "12345@Default", "12345@PvP", "PvP", "12345", false},
		{"names across profiles", "Primary@Default", "Secondary@PvP", "PvP", "67890", false},
		{"default source profile", "Primary", "67890@PvP", "PvP", "67890", false},
		{"default target profile", "Primary@Default", "Secondary", "Default", "67890", false},
		{"profile indices", "12345@0", "67890@1", "PvP", "67890", false},
		{"same file with profile aliases", "Primary@Default", "12345@settings_Default", "Default", "12345", false},
		{"same file with index", "12345", "Primary@0", "Default", "12345", false},
		{"invalid source index", "12345@9", "67890", "Default", "67890", true},
		{"invalid target index", "12345", "67890@9", "Default", "67890", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, profile := range []string{"Default", "PvP"} {
				if err := os.Mkdir(filepath.Join(dir, "settings_"+profile), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			sourcePath := filepath.Join(dir, "settings_Default", "core_user_12345.dat")
			targetPath := filepath.Join(dir, "settings_"+test.targetProfile, "core_user_"+test.targetID+".dat")
			if err := os.WriteFile(targetPath, []byte("old target settings"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sourcePath, []byte("source settings"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := copyAccountSettings(dir, "settings_Default", test.source, test.target)
			if (err != nil) != test.wantError {
				t.Fatalf("copyAccountSettings() error = %v, wantError %v", err, test.wantError)
			}
			wantTarget := "source settings"
			if test.wantError {
				wantTarget = "old target settings"
			}
			for path, want := range map[string]string{sourcePath: "source settings", targetPath: wantTarget} {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("settings at %s = %q, want %q", path, got, want)
				}
			}
		})
	}
}

func TestCopyAccountSettingsRequiresExistingTarget(t *testing.T) {
	configHome := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", configHome)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configHome)
	}

	dir := t.TempDir()
	profileDir := filepath.Join(dir, "settings_Default")
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "core_user_12345.dat"), []byte("settings"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyAccountSettings(dir, "settings_Default", "12345", "67890"); err == nil {
		t.Fatal("copyAccountSettings() succeeded, want missing target error")
	}
	if _, err := os.Stat(filepath.Join(profileDir, "core_user_67890.dat")); !os.IsNotExist(err) {
		t.Fatalf("target was unexpectedly created: %v", err)
	}
}
