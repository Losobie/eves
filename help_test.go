package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowHelpContext(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, generalHelp},
		{[]string{"--help"}, generalHelp},
		{[]string{"--help", "lookup"}, generalHelp},
		{[]string{"unknown", "--help"}, generalHelp},
		{[]string{"lookup", "--refresh", "--help"}, commandHelp["lookup"]},
		{[]string{"account", "copy", "--help", "Primary", "Secondary"}, commandHelp["account"]},
		{[]string{"copy", "Source", "Target", "--help"}, commandHelp["copy"]},
		{[]string{"group", "delete", "MyGroup", "--help"}, commandHelp["group"]},
		{[]string{"profile", "list", "--help"}, commandHelp["profile"]},
		{[]string{"formation", "list", "--help"}, commandHelp["formation"]},
		{[]string{"export", "--help"}, commandHelp["export"]},
	} {
		var output bytes.Buffer
		if !showHelp(test.args, &output) || output.String() != test.want+"\n" {
			t.Errorf("help for %v = %q, want %q", test.args, output.String(), test.want)
		}
	}
	for _, args := range [][]string{{"lookup"}, {"account", "list"}, {"lookup", "contains--help"}} {
		var output bytes.Buffer
		if showHelp(args, &output) || output.Len() != 0 {
			t.Errorf("unexpected help for %v", args)
		}
	}
}

func TestHelpBeforeConfiguration(t *testing.T) {
	if os.Getenv("EVES_TEST_HELP_PROCESS") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"eves"}, os.Args[i+1:]...)
				main()
				return
			}
		}
		t.Fatal("missing helper arguments")
	}
	for _, args := range [][]string{nil, {"--help"}, {"profile", "set", "Default", "--help"}, {"--verbose", "account", "detect", "--help"}, {"export", "--help"}} {
		configDir := t.TempDir()
		// Invalid config proves help is dispatched before reading configuration.
		evesDir := filepath.Join(configDir, "eves")
		if err := os.Mkdir(evesDir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(evesDir, "config.json")
		if err := os.WriteFile(path, []byte("invalid JSON"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("AppData", configDir)
		t.Setenv("XDG_CONFIG_HOME", configDir)
		t.Setenv("EVES_TEST_HELP_PROCESS", "1")
		commandArgs := append([]string{"-test.run=^TestHelpBeforeConfiguration$", "--"}, args...)
		cmd := exec.Command(os.Args[0], commandArgs...)
		output, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(output), "Usage:") {
			t.Fatalf("eves %v: %v, output: %s", args, err, output)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "invalid JSON" {
			t.Fatalf("help modified configuration: %q, %v", data, err)
		}
	}
}
