package main

import (
	"flag"
	"fmt"
	"log"
	"losobie.com/eves/eveapi"
	"losobie.com/eves/eves_config"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

func main() {

	config, err := eves_config.LoadConfig("eves_config.json")
	if err != nil {
		log.Fatalf("Error loading eves_config: %v", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("Error getting cache dir: %v", err)
	}

	directory := filepath.Join(cacheDir, "\\CCP\\EVE\\", config.EveEnv, "\\", config.SettingsFolder)

	args := os.Args[1:]
	if len(args) == 0 {
		lookupLocal(directory, config.Server, config.ServerSuffix)
		os.Exit(0)
	}

	if "copy" == args[0] {
		fmt.Println("Copy a characters settings over another")
		//all := flag.Bool("all", false, "Copy settings over all characters")
		flag.Parse()
		flagArgs := flag.Args()
		fmt.Println(flagArgs)
		//copySettings(charId, all)
	}
}

func copySettings(charId int, all bool) {

}

func lookupLocal(directory, server, suffix string) {
	rexChar := regexp.MustCompile(`^core_char_(\d+)\.dat$`)
	rexUser := regexp.MustCompile(`^core_user_(\d+)\.dat$`)

	var charIds []int
	var accountIds []int
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the current file matches the pattern
		if info.IsDir() {
			return nil
		}

		if rexChar.MatchString(info.Name()) {
			matches := rexChar.FindStringSubmatch(info.Name())
			if len(matches) > 1 {
				id, err := strconv.Atoi(matches[1])
				if err != nil {
					log.Fatalf("Error converting character id %s to int: %v", matches[1], err)
				}
				charIds = append(charIds, id)
			}
		}

		if rexUser.MatchString(info.Name()) {
			matches := rexUser.FindStringSubmatch(info.Name())
			if len(matches) > 1 {
				id, err := strconv.Atoi(matches[1])
				if err != nil {
					log.Fatalf("Error converting account id %s to int: %v", matches[1], err)
				}
				accountIds = append(accountIds, id)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", directory, err)
	}

	for _, id := range charIds {
		fmt.Println(id)
	}

	alliances := make(map[int]*eveapi.Alliance)
	corporations := make(map[int]*eveapi.Corporation)
	api := eveapi.NewApi(server, suffix)

	for _, id := range charIds {
		character, err := api.LookupCharacter(id)
		if err != nil {
			log.Printf("Error looking up character for ID %s: %v", id, err)
			continue
		}
		if len(character.Name) == 0 {
			continue
		}
		fmt.Printf("Character Name: %s (%d)\n", character.Name, id)
		if character.AllianceID != nil {
			alliances[*character.AllianceID] = nil
		}
		corporations[character.CorporationID] = nil
	}

	for key, _ := range alliances {
		if key == 0 {
			continue
		}
		alliance, err := api.LookupAlliance(key)
		if err != nil {
			log.Printf("Error looking up alliance for ID %s: %v", key, err)
			continue
		}
		fmt.Printf("Alliance Name: %s\n", alliance.Name)
	}

	for key, _ := range corporations {
		corporation, err := api.LookupCorporation(key)
		if err != nil {
			log.Printf("Error looking up corporation for ID %s: %v", key, err)
			continue
		}
		fmt.Printf("Corporation Name: %s\n", corporation.Name)
	}
}
