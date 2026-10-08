# Usage

## License

This project is licensed under the [MIT License](LICENSE).
The blue.Marshal decoder also retains the upstream TrueBrain and CCP Games
copyright and license notices in [internal/bluemarshal/LICENSE](internal/bluemarshal/LICENSE).

## Releases

Pushing a stable SemVer tag publishes a GitHub release through
[the release workflow](.github/workflows/release.yml). Tags may use `v1.2.3` or
`1.2.3`, including optional SemVer build metadata such as `v1.2.3+build.4`.
Prerelease tags (`v1.2.3-rc.1`), malformed versions, and tags whose commits are
not in `main`'s history are skipped.

The workflow tests on Windows and Linux before building AMD64 and ARM64
executables and archives for Windows, Linux, and macOS. Standalone executable
downloads are named `eves_<tag>_<os>_<arch>`, with `.exe` for Windows. For example,
`eves_v1.2.3_windows_amd64.exe` can be downloaded and run directly. On Linux and
macOS, make standalone downloads executable with `chmod +x <filename>`.

Windows ZIP archives contain `eves.exe`; Linux and macOS tar.gz archives contain
`eves`. Each archive also includes this README, the project's `LICENSE`, and
the decoder's license notices. Both `LICENSE` and `bluemarshal-LICENSE.txt`
are also available alongside standalone executables. `SHA256SUMS.txt` covers
both executables and archives, plus both license files. Published releases are
left unchanged on reruns;
interrupted draft releases can be completed by rerunning the workflow.

Merge the workflow into `main`, then tag and push the desired commit:

```text
git switch main
git pull --ff-only
git tag v1.2.3
git push origin v1.2.3
```

Publishing uses the repository's built-in `GITHUB_TOKEN`; no extra secret is
required. The tagged commit must contain the workflow for the tag push to run it.

## Help

Running `eves` without arguments, `eves help`, or `eves --help` shows general help.
Use `--help` anywhere
after a command to show help for that first command, without running it:

```text
eves
eves --help
eves help
eves lookup --help
eves account copy --help
eves group --help
```

Help for an unknown command falls back to general help. Help does not require
configuration or a local EVE installation.

There is no active-profile setting. Copy and group
references without `@profile` use `Default`; lookup lists scan all profiles.

## About

```text
eves about
```

Shows the version and platform, author Losobie (in-game character **Clento Loso**),
GitHub repository, and project/decoder license links. It works without
configuration or an EVE installation. Release builds show their version tag;
local builds default to `development`.

## List Profiles

```text
eves profile list
```

Lists profile names and zero-based indices in alphabetical order for use in
`@profile` references. Only listing is supported; there is no `profile get` or
`profile set` command.

## List Accounts

See also `eves formation list` to inspect account probe formations.

List the account IDs found in the current EVE settings profiles. Accounts with an assigned name include it in parentheses; unassigned accounts are shown by ID only.

```text
eves account list
```

## Set an Account Name

Assign or replace a name for an account ID manually.

```text
eves account set 12345 "Name"
```

## Copy Account Settings

Copy account settings within or across profiles. The source and target can each be an account ID or an assigned name, optionally followed by `@profile`. Profiles accept a name (with or without `settings_`) or a zero-based index of alphabetically sorted `settings_*` directories. When omitted, each side uses `Default`.

```text
eves account copy 12345 67890
eves account copy "Primary Account" "Secondary Account"
eves account copy "Primary Account" 67890
eves account copy "Primary Account@Default" "Secondary Account@PvP"
eves account copy "12345@Default" "12345@PvP"
```

The destination settings file must already exist and is overwritten in full. Copying is skipped only when both references resolve to the same file.

## Detect Account Settings

Watch all EVE settings profiles for the next modified account settings file and associate it with an account name. Associations apply across every profile, and detection continues until Ctrl+C.

```text
eves account detect
```

## List Probe Formations

List custom probe formations across all accounts and `settings_*` profiles in
the configured EVE environment:

```text
eves formation list
```

Optionally limit the list to an account ID or assigned account name. References
accept `@profile` names or zero-based indices from `eves profile list`; an
omitted profile means `Default`:

```text
eves formation list "Primary@PvP"
eves formation list "12345@1"
eves formation list 12345
eves formation --help
```

Output includes account ID/name, profile, formation ID/name, and probe count.
The same formation in different account/profile files is listed separately.
Internal formations with negative IDs are omitted. If there are no user
formations, the command prints `No custom probe formations found.`

Supply a formation name as a separate argument after the account reference to display the coordinates
and scan range of each probe in one formation:

```text
eves formation list "Alpha@Default" "temp"
```

Formation names match case-insensitively. Missing or ambiguous names return an
error. Coordinates are shown as `NORTH/SOUTH`, `EAST/WEST`, and `UP/DOWN` in
kilometres. Positive values mean north, east, and up; negative values mean south,
west, and down. Zero means no displacement on that axis. Ranges are in AU, and
probes are numbered from 1 in their stored order. Omitted profiles still use
`Default`.

The conversion uses EVE's stored axes: north/south = Z, east/west = -X, and
up/down = Y, following the compass mapping in
[EVE Wrench's formation editor](https://github.com/eve-wrench/eve-wrench-app/blob/main/crates/eve-wrench/src/formation_editor/model.rs).

Use `-o json` (also `--output json`) for a selected formation to print an
indented JSON object suitable for copying, editing, or saving:

```text
eves formation list "Alpha@Default" "temp" -o json
eves formation list "Alpha@Default" "temp" -o json > temp.json
```

The version 2 format contains `name` and a `probes` array. Each probe is
`[north/south, east/west, up/down, au]`: signed coordinates in kilometres
followed by scan range in AU, using the same signs as text output.
For example: `[250,0,0,0.25]` is 250 km north with a 0.25 AU scan range.
Probe order is preserved. The version changes from 1 to 2 because the coordinate
order and east/west sign differ from the previous `[x, y, z, au]` format.
Account/profile information and the source formation ID are omitted so the
definition can be reused elsewhere. JSON mode prints only JSON to stdout;
errors go to stderr. JSON output requires both an account and formation name.
Creating or importing a formation from this JSON will be added separately.

Listing reads local account settings directly without modifying them or calling
ESI. Unreadable, corrupt, or unsupported files are reported, with a nonzero exit
status; formations from readable files are still listed.

The read-only Go decoder is adapted from
[TrueBrain's blue-marshal-rs](https://github.com/TrueBrain/blue-marshal-rs), with
its MIT notices included in [internal/bluemarshal/LICENSE](internal/bluemarshal/LICENSE).
No Rust, Node, or external decoder executable is required to run `eves`.

## Export Settings

Export an entire blue.Marshal settings file as indented typed JSON:

```text
eves export "C:\EVE\settings_Default\core_user_12345.dat"
eves export 12345
eves export "Character Name@PvP"
eves export "Alpha@Default" > settings.json
eves export "Alpha@Default" --plain > settings.json
eves export 12345@1 --account
eves export "Character Name" --character
```

Sources accept file paths, character IDs/names, and account IDs or aliases assigned
with `eves account set`. References accept `@profile` names (including `settings_`)
or zero-based indices from `eves profile list`. Omitted profiles use `Default`.
An existing file path takes precedence over a reference, including paths containing
`@`. Paths do not require configuration or an EVE installation.

IDs automatically select the matching local account or character settings file.
If both exist for that ID, specify `--account` or `--character`. Account aliases
match case-insensitively and take precedence over character names; `--character`
bypasses aliases. Character names resolve through the existing cache, with an ESI
lookup on a cache miss. The selected settings file must exist in that profile.
Quote names and paths containing spaces. Use `--` before paths beginning with `-`.

Only JSON goes to stdout; errors go to stderr. The source file is never modified.
This exports all supported decoded settings, including unrelated settings and
temporary formations. It differs from `formation list -o json`, which exports
only one formation in its compact portable format.

The typed JSON follows the decoder's upstream convention: byte strings use
`bytes:`, Unicode uses `utf8:`, arbitrary precision integers use `long:`, and
tuples use `{"tuple":[...]}`. Dictionary keys carry type prefixes such as `int:`
and `bytes:`; compound keys use `json:`. Binary byte strings use `bytes:b64:`.
Non-finite floats and object wrappers are represented explicitly as data.
Shared references expand into repeated values rather than retaining identity.
Unsupported types, invalid data, bad checksums, or excessive expansion return
an error before printing a document. This is an inspection export; JSON import
and byte-identical round trips are not supported.

Use `--plain` for ordinary JSON without type markers. String values and
dictionary keys have no type prefixes, tuples become arrays, and large integers
become JSON numbers with their exact decimal digits. JSON consumers using
floating-point numbers may round large integers. Invalid UTF-8 byte strings
become unmarked base64 strings; non-finite floats become `"nan"`, `"inf"`, or
`"-inf"`. Instance wrappers become their state and callback wrappers become
their contents. Construction records retain callable/arguments, state, and
iterator data without their type wrapper or `newobj` marker. This mode loses
original type distinctions. If different dictionary keys convert to the same
JSON key, that dictionary becomes an array of `[key, value]` pairs in stored
order. For example, integer `2` and string `"2"` keys become
`[[2,"first value"],["2","second value"]]`. All entries in that dictionary are
retained; dictionaries without collisions remain JSON objects.

## Lookup

List characters across all `settings_*` profiles in the configured EVE environment and their corporations and alliances, resolving IDs to names. These commands produce the same output:

```text
eves lookup
eves lookup -a
eves lookup --all
```

Filter the list by type:

```text
eves lookup -c
eves lookup --characters
eves lookup --corporations
eves lookup --alliances
```

Look up one character, corporation, or alliance by its exact name (case-insensitive), including entries without local settings:

```text
eves lookup "Character Name"
eves lookup "Corporation Name"
eves lookup "Alliance Name"
```

Each result includes its type, name, and ID. Lists are sorted by type and name, with shared corporations and alliances shown once. Name-to-ID mappings last seven days; full records, including corporation and alliance membership, last six hours. Lists that need affiliations refresh those records independently of the name cache. Unknown names and failed requests return an error.

Use `--refresh` to bypass cached data for the requested lookup and save fresh results:

```text
eves lookup --refresh
eves lookup --characters --refresh
eves lookup "Character Name" --refresh
```

Normal cache reads do not extend expiration. Existing cached records retain their original expiration until refreshed.

## Backup Configurations

### List Backups
eves backup list

### Create New Backup
eves backup create [--tag, -t TAG] [--message, -m MESSAGE] [--group, -g GROUP_NAME]

examples:
* eves backup
* eves backup -t 1.1
* eves backup -t 1.2 -m 'Fixed drone window position'
* eves backup -t 1.3 -m 'Moved selected target for kiki alts' -g kiki

### Delete Backup

eves backup delete {TIMESTAMP | --all, -a | --tag, -t TAG | --group, -g GROUP_NAME | {--before TIMESTAMP | --after TIMESTAMP} }


eves
    lookup [CHAR_NAME] [--all, -a]
    backup
        list
        create [--tag, -t TAG] [--message, -m MESSAGE] [--group, -g GROUP_NAME]
        delete {TIMESTAMP | --all, -a | --tag, -t TAG | --group, -g GROUP_NAME | {--before TIMESTAMP | --after TIMESTAMP} }
        info {TIMESTAMP | --tag, -t TAG}
    restore {TIMESTAMP | --tag, -t TAG} [{--backup | --nobackup}]
    copy {CHAR_NAME | STORED_ID} {CHAR_NAME | GROUP_NAME | --all, -a }
    group
        list
        add {CHAR_NAME | CHAR_ID}
        remove {CHAR_NAME | CHAR_ID}
        info GROUP_NAME
    store {list | CHAR_NAME}
    cache
        clear
        update
    env
    list
    use ENVIRONMENT_NAME
    config ???

copy settings from source to destination

sources = character name, stored
destinations = character name, group, all

list (all) - shows all characters, their corporation and alliance including id's
list (character) - shows a specific character, its corporation, alliance and id's

backup
- list - lists all backups available with a timestamp and short message if added
- create - creates a new backup
  eg.
  backup create // creates a backup with no tag or message for all characters
  backup create -t 1.1 -m 'Version 1.1 of my backup' -g group1 // creates a backup with the tag "1.1", a message for characters in group1
- delete - deletes backups
  eg.
  delete 32151251251
  delete -tag 1.1
  delete -all
  delete -before 3210123
  delete -after 32151241
  delete -before 213214124 -after 1231412541
  delete -group group1 // deletes backups where the characters exactly match those in group1
- info - shows information about a specific backup selected by timestamp or tag

restore - restores a backup by tag or timestamp
restore 312512512515 - restores by timestamp
restore -tag 1.1 - restores by tag
restore 321421412541 -backup - restores by timestamp but creates a new backup before storing
restore 312412512541 -backup=false - stores by timestamp but does not save a backup when config for backup on restore is set

group - manages groups of characters that can be used in other commands (look into seeing if this can be integrated with EVE profiles???)
list
add
remove
info

env - commands to manage which EVE environment is being managed (on first run auto-detect and notify user)
config - commands to manage where eves will store its configuration files/cache (local or user settings)
cache - clears or updates the cache of character name lookups

https://support.eveonline.com/hc/en-us/articles/8563435867804-EVE-Online-Naming-Policy

Implementation order

1. copy CHAR_NAME {CHAR_NAME | --all, -a}
2. backup create
3. backup list
4. restore by TIMESTAMP
5. character name cache along with clear and update
6. remainder of backup and restore without groups
7. group commands
8. groups in the copy command
9. groups in the backup command
10. store command
11. store in the copy command
    12a. config command
    12b. env command

Other thoughts:

config settings
warning_backup_days
warning_last_backup
warning_restore

Have a backup warning before applying copy if a character has not been included in a previous backup going back a number of days. Setting backup_warning_days to 0 should mute this warning. Default to something like one day or one week (7).

Warning if you are about to delete the last backup for a character

Have a restore warning indicating which characters a restore would modify, this should be silenced if restore_warning is set to false
