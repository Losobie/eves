package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Groups map[string][]string

type kv struct {
	Key   string
	Value []string
}

func loadGroups() (Groups, error) {
	path, err := configFilePath("groups.json")
	if err != nil {
		return nil, err
	}
	vlog("using groups file: %s", path)

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Groups{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open groups file: %w", err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read groups file: %w", err)
	}
	if len(b) == 0 {
		return Groups{}, nil
	}

	var g Groups
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("parse groups.json: %w", err)
	}
	if g == nil {
		g = Groups{}
	}
	return g, nil
}

func saveGroups(g Groups) error {
	path, err := configFilePath("groups.json")
	if err != nil {
		return err
	}

	// Stable order and dedup members per group
	keys := make([]string, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string][]string, len(keys))
	for _, k := range keys {
		v := uniquePreserveOrder(g[k])
		ordered[k] = v
	}

	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

func memberKey(s string) string {
	// Normalize entries so dedupe/removal behave well with "Name@Profile"
	// and sloppy spacing like "Name @ Profile".
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return s
	}

	name := strings.TrimSpace(s[:at])
	profile := strings.TrimSpace(s[at+1:])
	return name + "@" + profile
}

func uniquePreserveOrder(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		key := memberKey(s)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

// AddToGroup adds name to group, avoiding duplicates (case-insensitive).
func AddToGroup(name, group string) error {
	name = strings.TrimSpace(name)
	group = strings.TrimSpace(group)
	if name == "" || group == "" {
		return errors.New("both name and group are required")
	}

	g, err := loadGroups()
	if err != nil {
		return err
	}

	list := g[group]
	kname := memberKey(name)
	for _, existing := range list {
		if memberKey(existing) == kname {
			return saveGroups(g)
		}
	}

	g[group] = append(list, kname)
	return saveGroups(g)
}

// RemoveFromGroup removes name from group (case-insensitive). Returns true if removed.
func RemoveFromGroup(name, group string) (bool, error) {
	name = strings.TrimSpace(name)
	group = strings.TrimSpace(group)
	if name == "" || group == "" {
		return false, errors.New("both name and group are required")
	}

	g, err := loadGroups()
	if err != nil {
		return false, err
	}

	list, ok := g[group]
	if !ok || len(list) == 0 {
		// group not found or empty
		return false, saveGroups(g)
	}

	kname := memberKey(name)
	newList := make([]string, 0, len(list))
	removed := false
	for _, existing := range list {
		if memberKey(existing) == kname {
			removed = true
			continue
		}
		newList = append(newList, existing)
	}

	if removed {
		if len(newList) == 0 {
			delete(g, group) // tidy up empty groups
		} else {
			g[group] = newList
		}
	}
	return removed, saveGroups(g)
}

// GroupMembers returns a group's members (deduped, original casing) sorted by insertion order.
func GroupMembers(group string) ([]string, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, errors.New("group is required")
	}
	g, err := loadGroups()
	if err != nil {
		return nil, err
	}
	return uniquePreserveOrder(g[group]), nil
}

// LoadAllGroupsSorted returns all groups and their members, with groups sorted by name.
func LoadAllGroupsSorted() ([]kv, error) {
	g, err := loadGroups()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]kv, 0, len(keys))
	for _, k := range keys {
		out = append(out, kv{Key: k, Value: uniquePreserveOrder(g[k])})
	}
	return out, nil
}

// DeleteGroup removes the specified group entirely
func DeleteGroup(group string) error {
	g, err := loadGroups()
	if err != nil {
		return err
	}
	delete(g, group)
	return saveGroups(g)
}
