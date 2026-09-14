# Usage

## Help

Running `eves` without arguments shows general help. Use `--help` anywhere
after a command to show help for that first command, without running it:

```text
eves
eves --help
eves lookup --help
eves account copy --help
eves group --help
```

Help for an unknown command falls back to general help. Help does not require
configuration or a local EVE installation.

There is no active-profile setting. Copy and group
references without `@profile` use `Default`; lookup lists scan all profiles.

## List Profiles

```text
eves profile list
```

Lists profile names and zero-based indices in alphabetical order for use in
`@profile` references. Only listing is supported; there is no `profile get` or
`profile set` command.

## List Accounts

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
