package eves_config

import (
	"encoding/json"
	"io"
	"os"
)

type Config struct {
	Server         string `json:"server"`
	ServerSuffix   string `json:"server_suffix"`
	EvePath        string `json:"eve_path"`
	EveEnv         string `json:"eve_env"`
	SettingsFolder string `json:"settings_folder"`
}

func LoadConfig(filename string) (Config, error) {
	var config Config

	// Open and read the file
	file, err := os.Open(filename)
	if err != nil {
		return config, err
	}
	defer file.Close()

	// Read the file into a byte slice
	bytes, err := io.ReadAll(file)
	if err != nil {
		return config, err
	}

	// Unmarshal the JSON data into the Config struct
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
