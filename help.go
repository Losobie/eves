package main

import (
	"fmt"
	"io"
	"strings"
)

const generalHelp = `EVE settings manager

Usage: eves <command> [arguments]

Commands:
  help     Show general help
  about    Show version, author, repository, and licenses
  lookup   Resolve character, corporation, and alliance names and IDs
  account  List, name, detect, and copy account settings
  copy     Copy character settings to a character or group
  group    Manage groups of characters
  profile  List EVE settings profiles
  formation List custom probe formations
  export   Export a complete settings file as typed JSON

Options:
  --help         Show help for the command given as the first argument
  --verbose, -v  Enable diagnostic output

Run eves <command> --help for command details.
Running eves without arguments or using eves help shows this help.`

var commandHelp = map[string]string{
	"about": `Show project information.

Usage: eves about

Shows the build version/platform, author and in-game character, GitHub
repository, MIT license, and decoder credits. No configuration or EVE
installation is required.`,
	"export": `Export a complete EVE settings file as typed JSON.

Usage: eves export <file-path|id-or-name[@profile]> [--account|--character] [--plain]

Accepts a file path, character ID/name, or account ID/assigned name.
References use Default unless @profile supplies a name or zero-based index.
Account aliases match case-insensitively and take precedence over character names.
Character names use cached lookups, or ESI when not cached.
Use --character to bypass an account alias, or --account to select account settings.
An ID with both settings types requires --account or --character.

Prints only indented JSON to stdout; redirect it to save a document.
Typed markers preserve byte/Unicode strings, dictionary keys, tuples, large
integers, and object wrappers. Shared references expand to repeated values.
Use --plain to omit type markers: strings/keys have no prefixes, tuples become
arrays, and large integers become JSON numbers. Instances export their state.
Binary strings become base64; non-finite floats become "nan", "inf", or "-inf".
Plain output loses original types. If dictionary keys collide after conversion,
that dictionary becomes a list of [key, value] pairs, preserving every entry.
Unsupported or corrupt files return an error. Source files are never modified.
This is an inspection export; settings import is not implemented.
File-path exports do not require configuration or a local EVE installation.

Examples:
  eves export "C:\EVE\settings_Default\core_user_12345.dat"
  eves export "Character Name@PvP" > settings.json
  eves export "Alpha@Default"
  eves export "Alpha@Default" --plain > settings.json
  eves export 12345@1 --account`,
	"formation": `List custom probe formations stored in account settings.

Usage: eves formation list [<account-id-or-name>[@profile] [<formation-name>]] [-o json|text]

Without an account reference, scans all accounts in every settings profile
of the configured EVE environment. With a reference, reads only that account;
omitting @profile uses Default. Profile names and zero-based indices work.

Shows account ID/name, profile, formation ID/name, and probe count.
Supply a formation name after the account reference to display its coordinates in km
and scan ranges in AU. Coordinates use north/south, east/west, and up/down:
positive means north, east, or up; negative means south, west, or down.
Names match case-insensitively; ambiguous names error.
Internal temporary formations are omitted. Settings files are never modified.
Unreadable files are reported as errors; readable files are still listed.

Use -o json (or --output json) with one selected formation to print portable
JSON version 2 containing its name and probes, each as
[north/south, east/west, up/down, au] (coordinates in km; same signs as text).
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
	requested := len(args) == 0 || args[0] == "help"
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
