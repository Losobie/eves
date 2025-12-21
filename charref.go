package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CharRef struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
}

func (r CharRef) String() string {
	return r.Name + "@" + r.Profile
}

func (r *CharRef) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type raw CharRef
	var tmp raw

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	tmp.Name = strings.TrimSpace(tmp.Name)
	tmp.Profile = strings.TrimSpace(tmp.Profile)

	if tmp.Name == "" {
		return fmt.Errorf("charref.name cannot be empty")
	}
	if tmp.Profile == "" {
		return fmt.Errorf("charref.profile cannot be empty")
	}

	*r = CharRef(tmp)
	return nil
}

func (r CharRef) MarshalJSON() ([]byte, error) {
	type raw CharRef
	tmp := raw{
		Name:    strings.TrimSpace(r.Name),
		Profile: strings.TrimSpace(r.Profile),
	}
	return json.Marshal(tmp)
}

func parseCharRef(s, profile string) CharRef {
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		if profile == "" {
			profile = "default"
		}
		return CharRef{Name: s, Profile: profile}
	}
	return CharRef{Name: strings.TrimSpace(s[:at]), Profile: strings.TrimSpace(s[at+1:])}
}

func resolveProfileDir(baseDir string, currentSettingsFolder string, profileToken string) (string, error) {
	settingsFolder := currentSettingsFolder

	// If no profile token provided just use the settings directory
	if profileToken == "" {
		return filepath.Join(baseDir, settingsFolder), nil
	}

	// If the profile token is an integer look it up based on order of directories
	if n, err := strconv.Atoi(profileToken); err == nil {
		dirs, err := os.ReadDir(baseDir)
		if err != nil {
			return "", err
		}
		vlog("Profile token is: %s", profileToken)
		count := 0
		found := false
		for _, e := range dirs {
			if e.IsDir() && strings.HasPrefix(e.Name(), "settings_") {
				if count == n {
					settingsFolder = e.Name()
					found = true
					break
				}
				count++
			}
		}
		if !found {
			return "", fmt.Errorf("profile index %d not found under %s", n, baseDir)
		}
		return filepath.Join(baseDir, settingsFolder), nil
	}

	// If profile token is a string make sure the directory will start with "settings_"
	if !strings.HasPrefix(profileToken, "settings_") {
		profileToken = "settings_" + profileToken
	}
	return filepath.Join(baseDir, profileToken), nil
}
