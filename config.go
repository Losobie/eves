package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Server         string `json:"server"`
	ServerSuffix   string `json:"server_suffix"`
	EveEnv         string `json:"eve_env"`
	SettingsFolder string `json:"settings_folder"`
}

func configFilePath(file string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			return "", errors.New("cannot determine user config dir")
		}
		switch runtime.GOOS {
		case "windows":
			dir = filepath.Join(home, "AppData", "Roaming")
		case "darwin":
			dir = filepath.Join(home, "Library", "Application Support")
		default:
			dir = filepath.Join(home, ".config")
		}
	}
	appDir := filepath.Join(dir, "eves")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(appDir, file), nil
}

var defaultConfig = `{
	"server": "https://esi.evetech.net/latest/",
	"server_suffix": "/?datasource=tranquility",
	"eve_env": "c_ccp_eve_tq_tranquility",
	"settings_folder": "settings_Default"
}`

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
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
			return nil, err
		}
		file, err = os.Open(path)
		if err != nil {
			return nil, err
		}
		vlog("Created new config file: %s", path)
	}
	defer file.Close()

	// Read the file into a byte slice
	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into the Config struct
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	path, err := configFilePath("config.json")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)

	// Create temp in the same dir
	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
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
