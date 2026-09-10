package main

import (
	"fmt"
	"io"
	"log"
	"losobie.com/eves/eveapi"
	"losobie.com/eves/kvcache"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

	baseDir := filepath.Join(cacheDir, "CCP", "EVE", config.EveEnv)
	directory := filepath.Join(baseDir, config.SettingsFolder)

	if len(args) == 1 {
		lookupLocal(directory, config.Server, config.ServerSuffix)
		os.Exit(0)
	}

	dirs, err := os.ReadDir(baseDir)
	if err != nil {
		log.Fatalf("Error getting base dir: %v", err)
	}

	var profiles []string
	for _, e := range dirs {
		if e.IsDir() && strings.HasPrefix(e.Name(), "settings_") {
			profiles = append(profiles, e.Name()[9:])
		}
	}

	switch args[1] {
	case "account":
		if len(args) < 3 {
			dieAccountUsage()
		}
		switch args[2] {
		case "list":
			if len(args) != 3 {
				dieAccountUsage()
			}
			if err := listAccounts(baseDir, os.Stdout); err != nil {
				fail(err)
			}
		case "detect":
			if len(args) != 3 {
				dieAccountUsage()
			}
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			defer signal.Stop(stop)
			if err := detectAccounts(baseDir, os.Stdin, os.Stdout, stop); err != nil {
				fail(err)
			}
		case "set":
			if len(args) < 5 {
				dieAccountUsage()
			}
			id := args[3]
			name := strings.Join(args[4:], " ")
			if err := setAccountName(id, name); err != nil {
				fail(err)
			}
			fmt.Printf("Set account %s name to %q.\n", id, strings.TrimSpace(name))
		case "copy":
			if len(args) != 5 {
				dieAccountUsage()
			}
			if err := copyAccountSettings(baseDir, config.SettingsFolder, args[3], args[4]); err != nil {
				fail(err)
			}
		default:
			dieAccountUsage()
		}
		return
	case "copy":
		if len(args) != 4 {
			dieCopyUsage()
		}

		currentProfile := config.SettingsFolder[9:]
		sourceChar := parseCharRef(args[2], currentProfile)
		sourceDir, err := resolveProfileDir(baseDir, config.SettingsFolder, sourceChar.Profile)
		if err != nil {
			fail(err)
		}

		sourceId, err := resolveCharID(sourceDir, config.Server, config.ServerSuffix, sourceChar.Name)
		if err != nil {
			fail(err)
		}

		targetToken := args[3]

		members, err := GroupMembers(targetToken)
		if err != nil {
			fail(err)
		}

		if len(members) > 0 {
			for _, memberRef := range members {
				memberDir, err := resolveProfileDir(baseDir, config.SettingsFolder, memberRef.Profile)
				if err != nil {
					// you can choose fail-fast or skip; this is skip w/ verbose logging
					vlog("Skipping %q: %v", memberRef, err)
					continue
				}

				memberId, err := resolveCharID(memberDir, config.Server, config.ServerSuffix, memberRef.Name)
				if err != nil {
					vlog("Skipping %q: %v", memberRef, err)
					continue
				}

				// Avoid self-copy when source and dest resolve to same file
				if sourceDir == memberDir && sourceId == memberId {
					continue
				}

				if err := copySettings(sourceDir, sourceId, memberDir, memberId); err != nil {
					// choose fail-fast here because partial copies can be surprising
					fail(fmt.Errorf("copy to %q failed: %w", memberRef, err))
				}
			}
			return
		}

		// Otherwise treat it as a character ref:
		destRef := parseCharRef(targetToken, currentProfile)
		destDir, err := resolveProfileDir(baseDir, config.SettingsFolder, destRef.Profile)
		if err != nil {
			fail(err)
		}

		destId, err := resolveCharID(destDir, config.Server, config.ServerSuffix, destRef.Name)
		if err != nil {
			fail(err)
		}

		if sourceDir == destDir && sourceId == destId {
			return
		}
		if err := copySettings(sourceDir, sourceId, destDir, destId); err != nil {
			fail(err)
		}
		return
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
			currentProfile := config.SettingsFolder[9:]
			charRef := parseCharRef(name, currentProfile)
			if n, err := strconv.Atoi(charRef.Profile); err == nil && n < len(profiles) {
				charRef.Profile = profiles[n]
			}
			if err := AddToGroup(charRef, group); err != nil {
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
			currentProfile := config.SettingsFolder[9:]
			charRef := parseCharRef(name, currentProfile)
			if n, err := strconv.Atoi(charRef.Profile); err == nil && n < len(profiles) {
				charRef.Profile = profiles[n]
			}
			removed, err := RemoveFromGroup(charRef, group)
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
			vlog("Profiles from %s", filepath.Dir(directory))

			count := 0
			for _, e := range profiles {
				var isCurrent string
				if config.SettingsFolder[9:] == e {
					isCurrent = "*"
				} else {
					isCurrent = ""
				}
				fmt.Printf("%s[%d] %s\n", isCurrent, count, e)
				count++
			}
			return
		default:
			dieConfigUsage()
		}
	default:
		fmt.Printf("Unrecognized command: %q\n", args[1])
	}
}

func copySettings(sourceDir, sourceId, targetDir, targetId string) error {
	sourcePath := filepath.Join(sourceDir, fmt.Sprintf("core_char_%s.dat", sourceId))
	targetPath := filepath.Join(targetDir, fmt.Sprintf("core_char_%s.dat", targetId))

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

	cache := kvcache.New("cache/chars.json", 6*time.Hour)
	api := eveapi.NewApi(server, suffix)

	charMap := make(map[string]string)
	for _, id := range charIds {

		charId, err := strconv.Atoi(id)
		if err != nil {
			log.Fatalf("Error converting account id %s to int: %v", id, err)
		}

		character, err := kvcache.GetOrLoad(cache, id, func() (eveapi.Character, error) {
			return api.LookupCharacter(charId)
		})

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

func resolveCharID(directory, server, suffix, name string) (string, error) {
	charMap := getLocalChars(directory, server, suffix)
	if id, ok := charMap[name]; ok {
		return id, nil
	}
	return "", fmt.Errorf("character not found in profile directory %s: %q", directory, name)
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
		vlog("Found character id: %d", id)
	}

	alliances := make(map[int]*eveapi.Alliance)
	corporations := make(map[int]*eveapi.Corporation)

	charCache := kvcache.New("cache/chars.json", 6*time.Hour)
	api := eveapi.NewApi(server, suffix)

	for _, id := range charIds {

		character, err := kvcache.GetOrLoad(charCache, strconv.Itoa(id), func() (eveapi.Character, error) {
			return api.LookupCharacter(id)
		})

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

	allianceCache := kvcache.New("cache/alliances.json", 6*time.Hour)
	for key := range alliances {
		if key == 0 {
			continue
		}
		alliance, err := kvcache.GetOrLoad(allianceCache, strconv.Itoa(key), func() (eveapi.Alliance, error) {
			return api.LookupAlliance(key)
		})
		if err != nil {
			log.Printf("Error looking up alliance for ID %d: %v", key, err)
			continue
		}
		fmt.Printf("Alliance Name: %s\n", alliance.Name)
	}

	corpCache := kvcache.New("cache/corporations.json", 6*time.Hour)
	for key := range corporations {
		corporation, err := kvcache.GetOrLoad(corpCache, strconv.Itoa(key), func() (eveapi.Corporation, error) {
			return api.LookupCorporation(key)
		})
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
  eves group add "Char Name" <group>
  eves group remove "Char Name" <group>
  eves group list
  eves group list <group>
  eves group delete <group>
`)+"\n")
	os.Exit(2)
}

func dieAccountUsage() {
	fmt.Fprintf(os.Stderr, strings.TrimSpace(`
usage:
  eves account list
  eves account set <account-id> <name>
  eves account copy <source-id-or-name>[@profile] <target-id-or-name>[@profile]
  eves account detect
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

func dieCopyUsage() {
	fmt.Fprintf(os.Stderr, strings.TrimSpace(`
usage:
  eves copy "Source Char Name" "Destination Char Name"
  eves copy "Source Char Name@profile1" "Destination Char Name"
  eves copy "Source Char Name@1" "Destination Char Name"
  eves copy "Source Char Name" "Destination Char Name@profile2"
  eves copy "Source Char Name" <group>
`)+"\n")
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
