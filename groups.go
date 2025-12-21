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

type Groups map[string][]CharRef

type kv struct {
	Key   string
	Value []CharRef
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

	ordered := make(map[string][]CharRef, len(keys))
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

func uniquePreserveOrder(charList []CharRef) []CharRef {
	seen := map[CharRef]struct{}{}
	out := make([]CharRef, 0, len(charList))
	for _, char := range charList {
		key := CharRef{
			Name:    strings.ToLower(strings.TrimSpace(char.Name)),
			Profile: strings.ToLower(strings.TrimSpace(char.Profile)),
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, char)
	}
	return out
}

// AddToGroup adds a character reference to group, avoiding duplicates (case-insensitive).
func AddToGroup(char CharRef, group string) error {
	char.Name = strings.TrimSpace(char.Name)
	if char.Name == "" {
		return errors.New("character name is required")
	}
	char.Profile = strings.TrimSpace(char.Profile)
	if char.Profile == "" {
		return errors.New("character profile is required")
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return errors.New("group name is required")
	}

	g, err := loadGroups()
	if err != nil {
		return err
	}

	list := g[group]
	for _, existing := range list {
		if existing == char {
			return saveGroups(g)
		}
	}

	g[group] = append(list, char)
	return saveGroups(g)
}

// RemoveFromGroup removes name from group (case-insensitive). Returns true if removed.
func RemoveFromGroup(char CharRef, group string) (bool, error) {
	char.Name = strings.TrimSpace(char.Name)
	if char.Name == "" {
		return false, errors.New("character name is required")
	}
	char.Profile = strings.TrimSpace(char.Profile)
	if char.Profile == "" {
		return false, errors.New("character profile is required")
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return false, errors.New("group name is required")
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

	newList := make([]CharRef, 0, len(list))
	removed := false
	for _, existing := range list {
		if existing == char {
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
func GroupMembers(group string) ([]CharRef, error) {
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
