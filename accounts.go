package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Accounts map[string]string

type accountFileState struct {
	id      string
	profile string
	modTime time.Time
	size    int64
}

type accountNameResult struct {
	name string
	err  error
}

var accountFilePattern = regexp.MustCompile(`^core_user_(\d+)\.dat$`)
var accountIDPattern = regexp.MustCompile(`^\d+$`)

func loadAccounts() (Accounts, error) {
	path, err := configFilePath("accounts.json")
	if err != nil {
		return nil, err
	}
	vlog("using accounts file: %s", path)

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Accounts{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read accounts file: %w", err)
	}
	if len(b) == 0 {
		return Accounts{}, nil
	}

	var accounts Accounts
	if err := json.Unmarshal(b, &accounts); err != nil {
		return nil, fmt.Errorf("parse accounts.json: %w", err)
	}
	if accounts == nil {
		accounts = Accounts{}
	}
	return accounts, nil
}

func saveAccounts(accounts Accounts) error {
	path, err := configFilePath("accounts.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("make accounts directory: %w", err)
	}

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return fmt.Errorf("encode accounts: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("create accounts temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("set accounts temp permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write accounts temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync accounts temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close accounts temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace accounts file: %w", err)
	}
	return nil
}

func setAccountName(id, name string) error {
	id = strings.TrimSpace(id)
	if !accountIDPattern.MatchString(id) {
		return errors.New("account ID must contain only digits")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("account name is required")
	}

	accounts, err := loadAccounts()
	if err != nil {
		return err
	}
	accounts[id] = name
	return saveAccounts(accounts)
}

func resolveAccountID(accounts Accounts, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("account ID or name is required")
	}
	if accountIDPattern.MatchString(ref) {
		return ref, nil
	}

	var matches []string
	for id, name := range accounts {
		if strings.EqualFold(name, ref) {
			matches = append(matches, id)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("account not found: %q", ref)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("account name %q is ambiguous; it is assigned to IDs %s", ref, strings.Join(matches, ", "))
	}
}

func copyAccountSettings(baseDir, currentSettingsFolder, sourceRef, targetRef string) error {
	accounts, err := loadAccounts()
	if err != nil {
		return err
	}
	sourceAccount := parseCharRef(sourceRef, currentSettingsFolder)
	targetAccount := parseCharRef(targetRef, currentSettingsFolder)
	sourceDir, err := resolveProfileDir(baseDir, currentSettingsFolder, sourceAccount.Profile)
	if err != nil {
		return fmt.Errorf("resolve source profile: %w", err)
	}
	targetDir, err := resolveProfileDir(baseDir, currentSettingsFolder, targetAccount.Profile)
	if err != nil {
		return fmt.Errorf("resolve target profile: %w", err)
	}
	sourceID, err := resolveAccountID(accounts, sourceAccount.Name)
	if err != nil {
		return fmt.Errorf("resolve source account: %w", err)
	}
	targetID, err := resolveAccountID(accounts, targetAccount.Name)
	if err != nil {
		return fmt.Errorf("resolve target account: %w", err)
	}
	sourcePath := filepath.Join(sourceDir, fmt.Sprintf("core_user_%s.dat", sourceID))
	targetPath := filepath.Join(targetDir, fmt.Sprintf("core_user_%s.dat", targetID))
	if sourcePath == targetPath {
		return nil
	}

	vlog("Copying account settings from %s to %s", sourcePath, targetPath)

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source account settings %s: %w", sourcePath, err)
	}
	defer source.Close()

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("inspect target account settings %s: %w", targetPath, err)
	}
	sourceInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect source account settings %s: %w", sourcePath, err)
	}
	// Paths with different casing or aliases can still refer to the same file.
	if os.SameFile(sourceInfo, targetInfo) {
		return nil
	}
	target, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("open target account settings %s: %w", targetPath, err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy account settings: %w", err)
	}
	return nil
}

func scanAccountFiles(baseDir string, associated Accounts) (map[string]accountFileState, error) {
	profiles, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("read EVE environment directory %s: %w", baseDir, err)
	}

	states := make(map[string]accountFileState)
	for _, profile := range profiles {
		if !profile.IsDir() || !strings.HasPrefix(profile.Name(), "settings_") {
			continue
		}

		profileDir := filepath.Join(baseDir, profile.Name())
		entries, err := os.ReadDir(profileDir)
		if err != nil {
			return nil, fmt.Errorf("read settings profile %s: %w", profileDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matches := accountFilePattern.FindStringSubmatch(entry.Name())
			if len(matches) != 2 {
				continue
			}
			id := matches[1]
			if _, ok := associated[id]; ok {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("inspect %s: %w", filepath.Join(profileDir, entry.Name()), err)
			}
			path := filepath.Join(profile.Name(), entry.Name())
			states[path] = accountFileState{
				id:      id,
				profile: profile.Name(),
				modTime: info.ModTime(),
				size:    info.Size(),
			}
		}
	}
	return states, nil
}

func changedAccountFiles(previous, current map[string]accountFileState) []accountFileState {
	changedByID := make(map[string]accountFileState)
	for path, state := range current {
		old, existed := previous[path]
		if !existed || !state.modTime.Equal(old.modTime) || state.size != old.size {
			changedByID[state.id] = state
		}
	}
	ids := make([]string, 0, len(changedByID))
	for id := range changedByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	changed := make([]accountFileState, 0, len(ids))
	for _, id := range ids {
		changed = append(changed, changedByID[id])
	}
	return changed
}

func listAccounts(baseDir string, output io.Writer) error {
	accounts, err := loadAccounts()
	if err != nil {
		return err
	}
	states, err := scanAccountFiles(baseDir, Accounts{})
	if err != nil {
		return err
	}

	accountIDs := make(map[string]struct{})
	for _, state := range states {
		accountIDs[state.id] = struct{}{}
	}

	ids := make([]string, 0, len(accountIDs))
	for id := range accountIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if name := accounts[id]; name != "" {
			fmt.Fprintf(output, "%s (%s)\n", id, name)
			continue
		}
		fmt.Fprintln(output, id)
	}
	return nil
}

func detectAccounts(baseDir string, input io.Reader, output io.Writer, stop <-chan os.Signal) error {
	accounts, err := loadAccounts()
	if err != nil {
		return err
	}
	states, err := scanAccountFiles(baseDir, accounts)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "Watching all settings profiles under %s for account settings changes. Press Ctrl+C to stop.\n", baseDir)
	reader := bufio.NewReader(input)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			fmt.Fprintln(output, "Stopped account detection.")
			return nil
		case <-ticker.C:
			current, err := scanAccountFiles(baseDir, accounts)
			if err != nil {
				return err
			}
			changed := changedAccountFiles(states, current)
			states = current

			for _, changedFile := range changed {
				id := changedFile.id
				if _, ok := accounts[id]; ok {
					continue
				}
				fmt.Fprintf(output, "Detected a change to %s/core_user_%s.dat. Enter the account name: ", changedFile.profile, id)
				nameResult := make(chan accountNameResult, 1)
				go func() {
					name, err := reader.ReadString('\n')
					nameResult <- accountNameResult{name: name, err: err}
				}()

				var result accountNameResult
				select {
				case <-stop:
					fmt.Fprintln(output, "\nStopped account detection.")
					return nil
				case result = <-nameResult:
				}
				if result.err != nil && !errors.Is(result.err, io.EOF) {
					return fmt.Errorf("read account name: %w", result.err)
				}
				name := strings.TrimSpace(result.name)
				if name == "" {
					fmt.Fprintln(output, "No account name entered; continuing to watch.")
					continue
				}

				accounts[id] = name
				if err := saveAccounts(accounts); err != nil {
					delete(accounts, id)
					return err
				}
				fmt.Fprintf(output, "Associated account %q with core_user_%s.dat.\n", name, id)
			}
		}
	}
}
