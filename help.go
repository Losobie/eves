package main

import (
	"fmt"
	"io"
	"strings"
)

const generalHelp = `EVE settings manager

Usage: eves <command> [arguments]

Commands:
  lookup   Resolve character, corporation, and alliance names and IDs
  account  List, name, detect, and copy account settings
  copy     Copy character settings to a character or group
  group    Manage groups of characters
  profile  List or select EVE settings profiles

Options:
  --help         Show help for the command given as the first argument
  --verbose, -v  Enable diagnostic output

Run eves <command> --help for command details.
Running eves without arguments shows this help.`

var commandHelp = map[string]string{
	"lookup": `Look up characters, corporations, and alliances.

Usage: eves lookup [--refresh] [--all|-a|--characters|-c|--corporations|--alliances|<name>]

  No filter, --all, -a  List local characters and their corporations/alliances
  --characters, -c      List characters in the selected profile
  --corporations       List corporations of local characters
  --alliances          List alliances of local characters
  <name>               Resolve one exact name, case-insensitively
  --refresh            Bypass cached data and save fresh results

Results include type, name, and ID. Names cache for seven days; details and
affiliations cache for six hours.

Examples:
  eves lookup
  eves lookup --characters --refresh
  eves lookup "Character Name"`,
	"account": `Manage account names and settings.

Usage:
  eves account list
  eves account set <account-id> <name>
  eves account copy <source-id-or-name>[@profile] <target-id-or-name>[@profile]
  eves account detect

list    List accounts across settings profiles with their assigned names.
set     Assign or replace an account name. Names apply across profiles.
copy    Overwrite an existing destination account settings file.
detect  Watch all profiles for changes and prompt for names; Ctrl+C stops.

Copy references accept a profile name or zero-based index after @.
Omitted profiles use the selected profile. Copying the same file is skipped.

Example:
  eves account copy "Primary@Default" "Secondary@PvP"`,
	"copy": `Copy character settings.

Usage:
  eves copy <source-character>[@profile] <target-character>[@profile]
  eves copy <source-character>[@profile] <group>

The destination settings are overwritten. Profiles accept names or zero-based
indices from eves profile list. Omitted profiles use the selected profile.
Group members use their stored profiles. Copying the same file is skipped.

Examples:
  eves copy "Source Character" "Target Character"
  eves copy "Source Character@Default" "Target Character@PvP"
  eves copy "Source Character@0" MyGroup`,
	"group": `Manage groups of characters for copying settings.

Usage:
  eves group add <character>[@profile] <group>
  eves group remove <character>[@profile] <group>
  eves group list
  eves group list <group>
  eves group delete <group>

Character references retain their profile. Omitted profiles use the selected
profile. List without a group shows all groups and member counts.

Example:
  eves group add "Character Name@Default" MyGroup`,
	"profile": `View or select the EVE settings profile.

Usage:
  eves profile list
  eves profile get
  eves profile set <name-or-index>

list  Show profile names and zero-based indices; * marks the selected profile.
get   Show the selected profile name.
set   Select a profile by name (with or without settings_) or index.

Examples:
  eves profile set Default
  eves profile set 0`,
}

// showHelp handles help before configuration, filesystem access, or command work.
// args excludes the executable and the global verbose option.
func showHelp(args []string, output io.Writer) bool {
	requested := len(args) == 0
	for _, arg := range args {
		if arg == "--help" {
			requested = true
		}
	}
	if !requested {
		return false
	}
	text := generalHelp
	if len(args) > 0 {
		if help, ok := commandHelp[args[0]]; ok {
			text = help
		}
	}
	fmt.Fprintln(output, strings.TrimSpace(text))
	return true
}
