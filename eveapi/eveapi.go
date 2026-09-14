package eveapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type EveApi struct {
	url        string
	datasource string
}

var client = &http.Client{Timeout: 30 * time.Second}

type NamedEntity struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// LookupName resolves an exact character, corporation, or alliance name.
func (receiver EveApi) LookupName(name string) (NamedEntity, error) {
	data, err := json.Marshal([]string{name})
	if err != nil {
		return NamedEntity{}, err
	}
	url := receiver.url + "universe/ids" + receiver.datasource
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return NamedEntity{}, fmt.Errorf("resolve name %q: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NamedEntity{}, fmt.Errorf("resolve name %q: HTTP %s", name, resp.Status)
	}
	var result struct {
		Characters   []NamedEntity `json:"characters"`
		Corporations []NamedEntity `json:"corporations"`
		Alliances    []NamedEntity `json:"alliances"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return NamedEntity{}, fmt.Errorf("decode name lookup: %w", err)
	}
	var matches []NamedEntity
	for category, entries := range map[string][]NamedEntity{
		"Character": result.Characters, "Corporation": result.Corporations, "Alliance": result.Alliances,
	} {
		for _, entry := range entries {
			if entry.ID > 0 && strings.EqualFold(entry.Name, name) {
				entry.Category = category
				matches = append(matches, entry)
			}
		}
	}
	if len(matches) == 0 {
		return NamedEntity{}, fmt.Errorf("character, corporation, or alliance not found: %q", name)
	}
	if len(matches) != 1 {
		return NamedEntity{}, fmt.Errorf("name %q is ambiguous", name)
	}
	return matches[0], nil
}

func NewApi(url string, datasource string) *EveApi {
	return &EveApi{
		url:        url,
		datasource: datasource,
	}
}

func (receiver EveApi) LookupCharacter(id int) (Character, error) {
	url := fmt.Sprintf("%s%s%d%s", receiver.url, "characters/", id, receiver.datasource)
	var character Character
	resp, err := client.Get(url)
	if err != nil {
		return character, fmt.Errorf("failed to resolve name for id: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return character, fmt.Errorf("lookup character %d: HTTP %s", id, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return character, fmt.Errorf("failed to read response body: %w", err)
	}
	err = json.Unmarshal(body, &character)
	if err != nil {
		return character, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	if character.Name == "" {
		return character, fmt.Errorf("lookup character %d returned an empty name", id)
	}
	return character, err
}

func (receiver EveApi) LookupAlliance(id int) (Alliance, error) {
	url := fmt.Sprintf("%s%s%d%s", receiver.url, "alliances/", id, receiver.datasource)
	var alliance Alliance
	resp, err := client.Get(url)
	if err != nil {
		return alliance, fmt.Errorf("failed to resolve name for id: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return alliance, fmt.Errorf("lookup alliance %d: HTTP %s", id, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return alliance, fmt.Errorf("failed to read response body: %w", err)
	}
	err = json.Unmarshal(body, &alliance)
	if err != nil {
		return alliance, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	if alliance.Name == "" {
		return alliance, fmt.Errorf("lookup alliance %d returned an empty name", id)
	}
	return alliance, err
}

func (receiver EveApi) LookupCorporation(id int) (Corporation, error) {
	var corporation Corporation
	url := fmt.Sprintf("%s%s%d%s", receiver.url, "corporations/", id, receiver.datasource)
	resp, err := client.Get(url)
	if err != nil {
		return corporation, fmt.Errorf("failed to resolve name for id: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return corporation, fmt.Errorf("lookup corporation %d: HTTP %s", id, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return corporation, fmt.Errorf("failed to read response body: %w", err)
	}
	err = json.Unmarshal(body, &corporation)
	if err != nil {
		return corporation, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	if corporation.Name == "" {
		return corporation, fmt.Errorf("lookup corporation %d returned an empty name", id)
	}
	return corporation, err
}
