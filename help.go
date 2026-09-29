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
  profile  List EVE settings profiles
  formation List custom probe formations

Options:
  --help         Show help for the command given as the first argument
  --verbose, -v  Enable diagnostic output

Run eves <command> --help for command details.
Running eves without arguments shows this help.`

var commandHelp = map[string]string{
	"formation": `List custom probe formations stored in account settings.

Usage: eves formation list [<account-id-or-name>[@profile] [<formation-name>]] [-o json|text]

Without an account reference, scans all accounts in every settings profile
of the configured EVE environment. With a reference, reads only that account;
omitting @profile uses Default. Profile names and zero-based indices work.

Shows account ID/name, profile, formation ID/name, and probe count.
Supply a formation name after the account reference to display its coordinates in km
and scan ranges in AU. Names match case-insensitively; ambiguous names error.
Internal temporary formations are omitted. Settings files are never modified.
Unreadable files are reported as errors; readable files are still listed.

Use -o json (or --output json) with one selected formation to print portable
JSON containing its name and probes, each as [x, y, z, au] (coordinates in km).
Output defaults to text. Formation creation/import is not yet implemented.

Examples:
  eves formation list
  eves formation list "Primary@PvP"
  eves formation list "Alpha@Default" "temp"
  eves formation list "Alpha@Default" "temp" -o json
  eves formation list 12345`,
	"profile": `List EVE settings profiles in the configured environment.

Usage: eves profile list

Shows profile names and zero-based indices in alphabetical order.
Use a name or index in @profile references. There is no active profile;
references without @profile use Default.`,
	"lookup": `Look up characters, corporations, and alliances.

Usage: eves lookup [--refresh] [--all|-a|--characters|-c|--corporations|--alliances|<name>]

  No filter, --all, -a  List local characters and their corporations/alliances
  --characters, -c      List characters across all settings profiles
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
Omitted profiles use the Default profile. Copying the same file is skipped.

Example:
  eves account copy "Primary@Default" "Secondary@PvP"`,
	"copy": `Copy character settings.

Usage:
  eves copy <source-character>[@profile] <target-character>[@profile]
  eves copy <source-character>[@profile] <group>

The destination settings are overwritten. Profiles accept names or zero-based
indices of alphabetically sorted settings directories. Omitted profiles use the Default profile.
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

Character references retain their profile. Omitted profiles use Default. List without a group shows all groups and member counts.

Example:
  eves group add "Character Name@Default" MyGroup`,
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
