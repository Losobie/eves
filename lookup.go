package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"losobie.com/eves/eveapi"
	"losobie.com/eves/kvcache"
)

const lookupUsage = "usage: eves lookup [--refresh] [--all|-a|--characters|-c|--corporations|--alliances|<name>]"

type lookupService struct {
	api                                        *eveapi.EveApi
	refresh                                    bool
	characters, corporations, alliances, names *kvcache.Cache
}

func newLookupService(server, suffix string) *lookupService {
	return &lookupService{
		api:          eveapi.NewApi(server, suffix),
		characters:   kvcache.New("cache/chars.json", 6*time.Hour),
		corporations: kvcache.New("cache/corporations.json", 6*time.Hour),
		alliances:    kvcache.New("cache/alliances.json", 6*time.Hour),
		names:        kvcache.New("cache/names.json", 7*24*time.Hour),
	}
}

func runLookup(directory string, service *lookupService, args []string, output io.Writer) error {
	// Keep refresh local to this invocation, even when the service is reused.
	invocation := *service
	service = &invocation
	service.refresh = false
	var filtered []string
	for _, arg := range args {
		if arg == "--refresh" {
			service.refresh = true
		} else {
			filtered = append(filtered, arg)
		}
	}
	args = filtered
	category := ""
	if len(args) > 0 {
		switch args[0] {
		case "-a", "--all":
		case "-c", "--characters":
			category = "Character"
		case "--corporations":
			category = "Corporation"
		case "--alliances":
			category = "Alliance"
		default:
			for _, arg := range args {
				if strings.HasPrefix(arg, "-") {
					return fmt.Errorf("unknown lookup option %q; %s", arg, lookupUsage)
				}
			}
			name := strings.TrimSpace(strings.Join(args, " "))
			if name == "" {
				return fmt.Errorf("name is required; %s", lookupUsage)
			}
			entry, err := service.lookupName(name)
			if err != nil {
				return err
			}
			return printLookupEntries(output, []eveapi.NamedEntity{entry})
		}
		if len(args) != 1 {
			return fmt.Errorf("lookup accepts one filter or a name; %s", lookupUsage)
		}
	}
	entries, err := service.lookupLocal(directory, category)
	if err != nil {
		return err
	}
	return printLookupEntries(output, entries)
}

func printLookupEntries(output io.Writer, entries []eveapi.NamedEntity) error {
	order := map[string]int{"Character": 0, "Corporation": 1, "Alliance": 2}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Category != b.Category {
			return order[a.Category] < order[b.Category]
		}
		if strings.ToLower(a.Name) != strings.ToLower(b.Name) {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.ID < b.ID
	})
	for _, entry := range entries {
		if _, err := fmt.Fprintf(output, "%s Name: %s (%d)\n", entry.Category, entry.Name, entry.ID); err != nil {
			return err
		}
	}
	return nil
}

var characterFilePattern = regexp.MustCompile(`^core_char_(\d+)\.dat$`)

func localCharacterIDs(directory string) ([]int, error) {
	ids := make(map[int]bool)
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if match := characterFilePattern.FindStringSubmatch(entry.Name()); match != nil {
			id, err := strconv.Atoi(match[1])
			if err != nil {
				return fmt.Errorf("invalid character ID in %s: %w", path, err)
			}
			if id > 0 {
				ids[id] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read local characters: %w", err)
	}
	return sortedLookupIDs(ids), nil
}

func sortedLookupIDs(ids map[int]bool) []int {
	result := make([]int, 0, len(ids))
	for id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	sort.Ints(result)
	return result
}

func (s *lookupService) lookupLocal(directory, category string) ([]eveapi.NamedEntity, error) {
	ids, err := localCharacterIDs(directory)
	if err != nil {
		return nil, err
	}
	var entries []eveapi.NamedEntity
	corporations, alliances := make(map[int]bool), make(map[int]bool)
	add := func(kind, name string, id int) error {
		if name == "" {
			return fmt.Errorf("lookup %s %d returned an empty name", strings.ToLower(kind), id)
		}
		if category == "" || category == kind {
			entries = append(entries, eveapi.NamedEntity{ID: id, Name: name, Category: kind})
		}
		return nil
	}
	for _, id := range ids {
		if category == "Character" && !s.refresh {
			if entry, err := kvcache.Get[eveapi.NamedEntity](s.names, nameCacheKey("Character", id)); err == nil {
				if err := add("Character", entry.Name, id); err != nil {
					return nil, err
				}
				continue
			}
		}
		character, err := lookupCached(s.characters, strconv.Itoa(id), s.refresh, func() (eveapi.Character, error) {
			value, err := s.api.LookupCharacter(id)
			if err == nil {
				s.rememberName("Character", value.Name, id)
			}
			return value, err
		})
		if err != nil {
			return nil, fmt.Errorf("lookup character %d: %w", id, err)
		}
		if err := add("Character", character.Name, id); err != nil {
			return nil, err
		}
		corporations[character.CorporationID] = true
		if character.AllianceID != nil {
			alliances[*character.AllianceID] = true
		}
	}
	if category == "Character" {
		return entries, nil
	}
	for _, id := range sortedLookupIDs(corporations) {
		if category == "Corporation" && !s.refresh {
			if entry, err := kvcache.Get[eveapi.NamedEntity](s.names, nameCacheKey("Corporation", id)); err == nil {
				if err := add("Corporation", entry.Name, id); err != nil {
					return nil, err
				}
				continue
			}
		}
		corporation, err := lookupCached(s.corporations, strconv.Itoa(id), s.refresh, func() (eveapi.Corporation, error) {
			value, err := s.api.LookupCorporation(id)
			if err == nil {
				s.rememberName("Corporation", value.Name, id)
			}
			return value, err
		})
		if err != nil {
			return nil, fmt.Errorf("lookup corporation %d: %w", id, err)
		}
		if err := add("Corporation", corporation.Name, id); err != nil {
			return nil, err
		}
		alliances[corporation.AllianceID] = true
	}
	if category == "Corporation" {
		return entries, nil
	}
	for _, id := range sortedLookupIDs(alliances) {
		if !s.refresh {
			if entry, err := kvcache.Get[eveapi.NamedEntity](s.names, nameCacheKey("Alliance", id)); err == nil {
				if err := add("Alliance", entry.Name, id); err != nil {
					return nil, err
				}
				continue
			}
		}
		alliance, err := lookupCached(s.alliances, strconv.Itoa(id), s.refresh, func() (eveapi.Alliance, error) {
			value, err := s.api.LookupAlliance(id)
			if err == nil {
				s.rememberName("Alliance", value.Name, id)
			}
			return value, err
		})
		if err != nil {
			return nil, fmt.Errorf("lookup alliance %d: %w", id, err)
		}
		if err := add("Alliance", alliance.Name, id); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func (s *lookupService) lookupName(name string) (eveapi.NamedEntity, error) {
	if s.refresh {
		return s.fetchName(name)
	}
	// Read the existing ID caches as well, so names discovered by other commands
	// can be resolved without a network request.
	var matches []eveapi.NamedEntity
	names, _ := kvcache.Entries[eveapi.NamedEntity](s.names)
	for _, entry := range names {
		if strings.EqualFold(entry.Name, name) {
			matches = append(matches, entry)
		}
	}
	for _, source := range []struct {
		cache    *kvcache.Cache
		category string
	}{{s.characters, "Character"}, {s.corporations, "Corporation"}, {s.alliances, "Alliance"}} {
		values, err := kvcache.Entries[struct {
			Name string `json:"name"`
		}](source.cache)
		if err != nil {
			continue // An unreadable cache can still be resolved through ESI.
		}
		for key, value := range values {
			if strings.EqualFold(value.Name, name) {
				id, err := strconv.Atoi(key)
				if err == nil && id > 0 {
					if _, exists := names[nameCacheKey(source.category, id)]; exists {
						continue
					}
					matches = append(matches, eveapi.NamedEntity{ID: id, Name: value.Name, Category: source.category})
				}
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return eveapi.NamedEntity{}, fmt.Errorf("name %q is ambiguous", name)
	}
	return s.fetchName(name)
}

func nameCacheKey(category string, id int) string {
	return category + ":" + strconv.Itoa(id)
}

func (s *lookupService) rememberName(category, name string, id int) {
	// Key by identity so a refreshed name replaces the old spelling or alias.
	_ = kvcache.Put(s.names, nameCacheKey(category, id), eveapi.NamedEntity{ID: id, Name: name, Category: category})
}

func (s *lookupService) fetchName(name string) (eveapi.NamedEntity, error) {
	entry, err := s.api.LookupName(name)
	if err == nil {
		s.rememberName(entry.Category, entry.Name, entry.ID)
	}
	return entry, err
}

func lookupCached[V any](cache *kvcache.Cache, key string, refresh bool, loader func() (V, error)) (V, error) {
	if !refresh {
		return kvcache.GetOrLoad(cache, key, loader)
	}
	value, err := loader()
	if err == nil {
		_ = kvcache.Put(cache, key, value)
	}
	return value, err
}
