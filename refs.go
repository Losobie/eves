package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CharRef struct {
	Name    string
	Profile string
}

func parseCharRef(s string) CharRef {
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return CharRef{Name: s}
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
