package eveapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type EveApi struct {
	url        string
	datasource string
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
	resp, err := http.Get(url)
	if err != nil {
		return character, fmt.Errorf("failed to resolve name for id: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return character, fmt.Errorf("failed to read response body: %w", err)
	}
	err = json.Unmarshal(body, &character)
	if err != nil {
		return character, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return character, err
}

func (receiver EveApi) LookupAlliance(id int) (Alliance, error) {
	url := fmt.Sprintf("%s%s%d%s", receiver.url, "alliances/", id, receiver.datasource)
	var alliance Alliance
	resp, err := http.Get(url)
	if err != nil {
		return alliance, fmt.Errorf("failed to resolve name for id: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return alliance, fmt.Errorf("failed to read response body: %w", err)
	}
	err = json.Unmarshal(body, &alliance)
	if err != nil {
		return alliance, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return alliance, err
}

func (receiver EveApi) LookupCorporation(id int) (Corporation, error) {
	var corporation Corporation
	url := fmt.Sprintf("%s%s%d%s", receiver.url, "corporations/", id, receiver.datasource)
	resp, err := http.Get(url)
	if err != nil {
		return corporation, fmt.Errorf("failed to resolve name for id: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return corporation, fmt.Errorf("failed to read response body: %w", err)
	}
	err = json.Unmarshal(body, &corporation)
	if err != nil {
		return corporation, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return corporation, err
}
