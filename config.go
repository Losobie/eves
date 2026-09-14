package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Server       string `json:"server"`
	ServerSuffix string `json:"server_suffix"`
	EveEnv       string `json:"eve_env"`
}

func configFilePath(file string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "eves", file), nil
}

const defaultConfig = `{
	"server": "https://esi.evetech.net/latest/",
	"server_suffix": "/?datasource=tranquility",
	"eve_env": "c_ccp_eve_tq_tranquility"
}`

func init() {
	var v any
	if err := json.Unmarshal([]byte(defaultConfig), &v); err != nil {
		panic("defaultConfig is invalid JSON: " + err.Error())
	}
}

func LoadConfig() (*Config, error) {
	path, err := configFilePath("config.json")
	if err != nil {
		return nil, err
	}
	vlog("using config file: %s", path)

	var config Config

	// Open and read the file
	file, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("open config %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("make config path %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
			return nil, fmt.Errorf("write default config %s: %w", path, err)
		}
		file, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open default config %s: %w", path, err)
		}
		vlog("Created new config file: %s", path)
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	dec.DisallowUnknownFields() // optional but very nice for catching typos
	// Accept the retired field only to migrate older configurations.
	legacy := struct {
		*Config
		SettingsFolder json.RawMessage `json:"settings_folder"`
	}{Config: &config}
	if err := dec.Decode(&legacy); err != nil {
		return nil, err
	}
	if len(legacy.SettingsFolder) > 0 {
		if err := file.Close(); err != nil {
			return nil, err
		}
		if err := SaveConfig(&config); err != nil {
			return nil, fmt.Errorf("remove retired settings_folder: %w", err)
		}
	}
	return &config, nil
}

func SaveConfig(config *Config) error {
	path, err := configFilePath("config.json")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("make config dir %s: %w", dir, err)
	}

	// Create temp in the same dir
	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	_ = tmp.Chmod(0o600)
	tmpPath := tmp.Name()

	// Ensure cleanup on any failure.
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	// On success, we'll Close() without removing.

	// Encode JSON with indentation
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(config); err != nil {
		cleanup()
		return fmt.Errorf("encode config: %w", err)
	}

	// fsync file to push bytes to disk
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename to final path (same filesystem)
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp to final: %w", err)
	}

	// 5) best-effort fsync the directory (POSIX durability for the rename)
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" { // macOS dirfsync is often a no-op, harmless to try
		if d, err := os.Open(dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
	return nil
}
