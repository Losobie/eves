package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"losobie.com/eves/eveapi"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {

	args := os.Args

	// Detect and strip --verbose
	clean := []string{args[0]}
	for _, a := range args[1:] {
		if a == "--verbose" || a == "-v" {
			Verbose = true
		} else {
			clean = append(clean, a)
		}
	}
	args = clean

	for i, a := range args {
		vlog("args[%d]: %q", i, a)
	}

	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config file: %v", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("Error getting cache dir: %v", err)
	}

	directory := filepath.Join(cacheDir, "\\CCP\\EVE\\", config.EveEnv, "\\", config.SettingsFolder)

	if len(args) == 1 {
		lookupLocal(directory, config.Server, config.ServerSuffix)
		os.Exit(0)
	}

	switch args[1] {
	case "copy":
		if len(args) < 4 {
			fmt.Println("Copy a characters settings over another")
			//all := flag.Bool("all", false, "Copy settings over all characters")
			flag.Parse()
			flagArgs := flag.Args()
			fmt.Println(flagArgs)
		} else if len(args) == 4 {

			charMap := getLocalChars(directory, config.Server, config.ServerSuffix)
			sourceChar := args[2]

			sourceId, ok := charMap[sourceChar]
			if !ok {
				log.Fatalf("Source character not found: %s", sourceChar)
			}

			target := args[3]

			members, err := GroupMembers(target)
			if err != nil {
				log.Fatalf("Error getting group members: %v", err)
			}
			if members == nil || len(members) == 0 {

			}

			var targetIds []string
			for _, charName := range members {
				if v, ok := charMap[charName]; ok {
					targetIds = append(targetIds, v)
				}
			}

			for _, targetId := range targetIds {
				if sourceId == targetId {
					continue
				}
				err := copySettings(directory, sourceId, targetId)
				if err != nil {
					log.Fatalf("Error copying settings from %s to %s: %v", sourceId, targetId, err)
				}
			}
		}
	case "group":
		if len(args) < 3 {
			dieGroupUsage()
		}
		switch args[2] {
		case "add":
			// eves group add "My Name" group1
			if len(args) < 5 {
				dieGroupUsage()
			}
			name := args[3]
			group := args[4]
			if err := AddToGroup(name, group); err != nil {
				fail(err)
			}
			fmt.Printf("Added %q to group %q\n", name, group)
			return

		case "remove":
			// eves group remove "My Name" group1
			if len(args) < 5 {
				dieGroupUsage()
			}
			name := args[3]
			group := args[4]
			removed, err := RemoveFromGroup(name, group)
			if err != nil {
				fail(err)
			}
			if removed {
				fmt.Printf("Removed %q from group %q\n", name, group)
			} else {
				fmt.Printf("%q was not in group %q (no changes)\n", name, group)
			}
			return

		case "list":
			// eves group list            -> all groups w/ counts
			// eves group list group1     -> members of group1
			if len(args) == 3 {
				g, err := LoadAllGroupsSorted()
				if err != nil {
					fail(err)
				}
				if len(g) == 0 {
					fmt.Println("No groups found.")
					return
				}
				fmt.Println("Groups:")
				for _, kv := range g {
					fmt.Printf("  %s (%d)\n", kv.Key, len(kv.Value))
				}
				return
			}
			group := args[3]
			members, err := GroupMembers(group)
			if err != nil {
				fail(err)
			}
			if len(members) == 0 {
				fmt.Printf("Group %q has no members.\n", group)
				return
			}
			fmt.Printf("Members of %q:\n", group)
			for _, m := range members {
				fmt.Printf("  %s\n", m)
			}
			return
		case "delete":
			// eves group delete group1
			if len(args) != 4 {
				dieGroupUsage()
			}
			group := args[3]
			err := DeleteGroup(group)
			if err != nil {
				fail(err)
			}
		default:
			dieGroupUsage()
		}
	case "profile":
		if len(args) < 3 {
			dieConfigUsage()
		}
		switch args[2] {
		case "get":
			fmt.Println(config.SettingsFolder[9:])
			return
		case "set":
			if len(args) < 4 {
				dieConfigUsage()
			}
			profile := args[3]

			profileId, err := strconv.Atoi(profile)
			if err == nil {
				dirs, err := os.ReadDir(filepath.Dir(directory))
				if err != nil {
					fail(err)
				}
				count := 0
				for _, e := range dirs {
					if e.IsDir() && strings.HasPrefix(e.Name(), "settings_") {
						if count == profileId {
							fmt.Println(e.Name()[9:])
							config.SettingsFolder = e.Name()
							break
						}
						count++
					}
				}
			} else {
				// Append settings_ to front of profile if missing
				if len(profile) < 9 || "settings_" != profile[:9] {
					profile = "settings_" + profile
				}

				config.SettingsFolder = profile
			}

			err = SaveConfig(config)
			if err != nil {
				fail(err)
			}
			return
		case "list":
			fmt.Printf("Profiles from %s\n", filepath.Dir(directory))

			dirs, err := os.ReadDir(filepath.Dir(directory))
			if err != nil {
				fail(err)
			}

			count := 0
			for _, e := range dirs {
				if e.IsDir() && strings.HasPrefix(e.Name(), "settings_") {
					var isCurrent string
					if config.SettingsFolder == e.Name() {
						isCurrent = "*"
					} else {
						isCurrent = ""
					}
					fmt.Printf("%s[%d] %s\n", isCurrent, count, e.Name()[9:])
					count++
				}
			}
			return
		default:
			dieConfigUsage()
		}
	default:
		fmt.Printf("Unrecognized command: %q\n", args[1])
	}
}

func copySettings(directory, sourceId, targetId string) error {
	sourcePath := directory + "\\core_char_" + sourceId + ".dat"
	targetPath := directory + "\\core_char_" + targetId + ".dat"
	vlog("Copying settings from %s to %s", sourcePath, targetPath)

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create (or overwrite) the target file
	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	// Copy the contents
	if _, err := io.Copy(target, source); err != nil {
		return err
	}

	return nil
}

func getLocalChars(directory, server, suffix string) map[string]string {
	rexChar := regexp.MustCompile(`^core_char_(\d+)\.dat$`)

	var charIds []string

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
				id := matches[1]
				charIds = append(charIds, id)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", directory, err)
	}

	for _, id := range charIds {
		vlog("Found character id: %s", id)
	}

	api := eveapi.NewApi(server, suffix)

	charMap := make(map[string]string)
	for _, id := range charIds {

		charId, err := strconv.Atoi(id)
		if err != nil {
			log.Fatalf("Error converting account id %s to int: %v", id, err)
		}
		character, err := api.LookupCharacter(charId)

		if err != nil {
			log.Printf("Error looking up character for ID %s: %v", id, err)
			continue
		}
		if len(character.Name) == 0 {
			continue
		}
		vlog("Character Name: %s (%s)", character.Name, id)
		charMap[character.Name] = id
	}
	return charMap
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
			log.Printf("Error looking up character for ID %d: %v", id, err)
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

	for key := range alliances {
		if key == 0 {
			continue
		}
		alliance, err := api.LookupAlliance(key)
		if err != nil {
			log.Printf("Error looking up alliance for ID %d: %v", key, err)
			continue
		}
		fmt.Printf("Alliance Name: %s\n", alliance.Name)
	}

	for key := range corporations {
		corporation, err := api.LookupCorporation(key)
		if err != nil {
			log.Printf("Error looking up corporation for ID %d: %v", key, err)
			continue
		}
		fmt.Printf("Corporation Name: %s\n", corporation.Name)
	}
}

func dieGroupUsage() {
	fmt.Fprintf(os.Stderr, strings.TrimSpace(`
usage:
  eves group add "My Name" <group>
  eves group remove "My Name" <group>
  eves group list
  eves group list <group>
  eves group delete <group>
`)+"\n")
	os.Exit(2)
}

func dieConfigUsage() {
	fmt.Fprintf(os.Stderr, strings.TrimSpace(`
usage:
  eves profile list
  eves profile get
  eves profile set 1
  eves profile set Default
`)+"\n")
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
