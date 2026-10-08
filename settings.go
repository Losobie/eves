package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"losobie.com/eves/internal/bluemarshal"
)

const settingsUsage = "usage: eves export <file-path|id-or-name[@profile]> [--account|--character] [--plain]"

type settingsExportOptions struct {
	source string
	kind   string
	plain  bool
}

func parseSettingsExport(args []string) (settingsExportOptions, error) {
	var opts settingsExportOptions
	positional := false
	for _, arg := range args {
		if !positional && arg == "--" {
			positional = true
			continue
		}
		if !positional && arg == "--plain" {
			opts.plain = true
			continue
		}
		if !positional && (arg == "--account" || arg == "--character") {
			if opts.kind != "" {
				return opts, fmt.Errorf("select only one settings type; %s", settingsUsage)
			}
			opts.kind = strings.TrimPrefix(arg, "--")
			continue
		}
		if !positional && strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("unknown settings option %q; %s", arg, settingsUsage)
		}
		if opts.source != "" || strings.TrimSpace(arg) == "" {
			return opts, errors.New(settingsUsage)
		}
		opts.source = arg
	}
	if opts.source == "" {
		return opts, errors.New(settingsUsage)
	}
	return opts, nil
}

// An existing path always wins, including paths containing @. Missing path-like
// arguments remain paths, so a typo does not accidentally trigger an ESI lookup.
func isSettingsPath(source string) (bool, error) {
	_, err := os.Stat(source)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return filepath.IsAbs(source) || strings.ContainsAny(source, `/\`) ||
		strings.EqualFold(filepath.Ext(source), ".dat"), nil
}

func runExport(args []string, output io.Writer) error {
	opts, err := parseSettingsExport(args)
	if err != nil {
		return err
	}
	isPath, err := isSettingsPath(opts.source)
	if err != nil {
		return fmt.Errorf("check settings source: %w", err)
	}
	if isPath {
		if opts.kind != "" {
			return fmt.Errorf("--%s applies to references, not file paths", opts.kind)
		}
		return exportSettingsFile(opts.source, output, opts.plain)
	}
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	baseDir := filepath.Join(cacheDir, "CCP", "EVE", config.EveEnv)
	accounts := Accounts{}
	if opts.kind != "character" && !accountIDPattern.MatchString(parseCharRef(opts.source, "Default").Name) {
		accounts, err = loadAccounts()
		if err != nil {
			return err
		}
	}
	path, err := resolveSettingsSource(baseDir, accounts, newLookupService(config.Server, config.ServerSuffix), opts)
	if err != nil {
		return err
	}
	return exportSettingsFile(path, output, opts.plain)
}

func resolveSettingsSource(baseDir string, accounts Accounts, service *lookupService, opts settingsExportOptions) (string, error) {
	ref := parseCharRef(strings.TrimSpace(opts.source), "Default")
	if ref.Name == "" || ref.Profile == "" || strings.ContainsAny(ref.Profile, `/\`) || strings.HasSuffix(opts.source, "@") {
		return "", fmt.Errorf("invalid settings reference %q", opts.source)
	}
	dir, err := resolveProfileDir(baseDir, "settings_Default", ref.Profile)
	if err != nil {
		return "", err
	}
	file := func(kind, id string) string {
		prefix := "core_char_"
		if kind == "account" {
			prefix = "core_user_"
		}
		return filepath.Join(dir, prefix+id+".dat")
	}
	if accountIDPattern.MatchString(ref.Name) {
		if opts.kind != "" {
			return file(opts.kind, ref.Name), nil
		}
		var matches []string
		for _, kind := range []string{"account", "character"} {
			path := file(kind, ref.Name)
			info, err := os.Stat(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			if err == nil && !info.IsDir() {
				matches = append(matches, path)
			}
		}
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return "", fmt.Errorf("no account or character settings for ID %s in profile %s", ref.Name, ref.Profile)
		default:
			return "", fmt.Errorf("ID %s has both account and character settings; use --account or --character", ref.Name)
		}
	}
	// Account names are local aliases, so they work offline and take precedence
	// over EVE character names. --character explicitly bypasses aliases.
	if opts.kind != "character" {
		hasAlias := false
		for _, name := range accounts {
			if strings.EqualFold(name, ref.Name) {
				hasAlias = true
				break
			}
		}
		if opts.kind == "account" || hasAlias {
			id, err := resolveAccountID(accounts, ref.Name)
			if err != nil {
				return "", err
			}
			if !accountIDPattern.MatchString(id) {
				return "", fmt.Errorf("account alias %q has invalid ID %q", ref.Name, id)
			}
			return file("account", id), nil
		}
	}
	entry, err := service.lookupName(ref.Name)
	if err != nil {
		return "", fmt.Errorf("resolve character name: %w", err)
	}
	if entry.Category != "Character" || entry.ID <= 0 {
		return "", fmt.Errorf("%q is not a character", ref.Name)
	}
	return file("character", strconv.Itoa(entry.ID)), nil
}

func exportSettingsFile(path string, output io.Writer, plain bool) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open settings %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, bluemarshal.MaxFileSize+1))
	if err != nil {
		return fmt.Errorf("read settings %s: %w", path, err)
	}
	root, err := bluemarshal.Decode(data)
	if err != nil {
		return fmt.Errorf("decode settings %s: %w", path, err)
	}
	convert := bluemarshal.ToJSON
	if plain {
		convert = bluemarshal.ToPlainJSON
	}
	value, err := convert(root)
	if err != nil {
		return fmt.Errorf("convert settings %s: %w", path, err)
	}
	enc := json.NewEncoder(output)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("write settings JSON: %w", err)
	}
	return nil
}
