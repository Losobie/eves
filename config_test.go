package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadsCurrentFields(t *testing.T) {
	isolateLookupCache(t)
	path, err := configFilePath("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"server":"https://example.test/","server_suffix":"/","eve_env":"custom"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.EveEnv != "custom" || config.Server != "https://example.test/" || config.ServerSuffix != "/" {
		t.Fatalf("config changed: %+v", config)
	}
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("unknown config field accepted")
	}
}
