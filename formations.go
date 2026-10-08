package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"losobie.com/eves/internal/bluemarshal"
)

const formationUsage = "usage: eves formation list [<account-id-or-name>[@profile] [<formation-name>]] [-o json|text]"

// Coordinates and scan range are stored in metres, as in EVE's settings file.
type formationProbe struct {
	X, Y, Z, Range float64
}

type probeFormation struct {
	ID     int64
	Name   string
	Probes []formationProbe
}

// readProbeFormations never writes to or creates EVE settings files.
func readProbeFormations(path string) ([]probeFormation, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, bluemarshal.MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	root, err := bluemarshal.Decode(data)
	if err != nil {
		return nil, err
	}
	if root.Kind != bluemarshal.TY_DICT {
		return nil, errors.New("settings root is not a dictionary")
	}
	ui, exists := root.Field("ui")
	if !exists {
		return nil, nil
	}
	if ui.Kind != bluemarshal.TY_DICT {
		return nil, errors.New("settings ui is not a dictionary")
	}
	setting, exists := ui.Field("probescanning.customFormations")
	if !exists {
		return nil, nil
	}
	if setting.Kind != bluemarshal.TY_TUPLE || len(setting.Items) != 2 || setting.Items[1].Kind != bluemarshal.TY_DICT {
		return nil, errors.New("invalid custom formations setting")
	}
	var formations []probeFormation
	seen := make(map[int64]bool)
	for _, pair := range setting.Items[1].Pairs {
		id, ok := pair.Key.Integer()
		if !ok {
			return nil, errors.New("formation ID is not an integer")
		}
		if id < 0 {
			continue
		} // EVE's temporary/internal formations are not user entries.
		if seen[id] {
			return nil, fmt.Errorf("duplicate formation ID %d", id)
		}
		seen[id] = true
		v := pair.Value
		if v.Kind != bluemarshal.TY_TUPLE || len(v.Items) != 2 || v.Items[1].Kind != bluemarshal.TY_LIST {
			return nil, fmt.Errorf("invalid formation %d", id)
		}
		name, ok := v.Items[0].StringValue()
		if !ok || !utf8.ValidString(name) {
			return nil, fmt.Errorf("invalid formation %d name", id)
		}
		var probes []formationProbe
		for _, probe := range v.Items[1].Items {
			if probe.Kind != bluemarshal.TY_TUPLE || len(probe.Items) != 2 || probe.Items[0].Kind != bluemarshal.TY_TUPLE || len(probe.Items[0].Items) != 3 {
				return nil, fmt.Errorf("invalid probe in formation %d", id)
			}
			var values [4]float64
			for i, coordinate := range [4]*bluemarshal.Value{probe.Items[0].Items[0], probe.Items[0].Items[1], probe.Items[0].Items[2], probe.Items[1]} {
				n, ok := coordinate.Number()
				if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
					return nil, fmt.Errorf("invalid probe coordinate or range in formation %d", id)
				}
				values[i] = n
			}
			probes = append(probes, formationProbe{X: values[0], Y: values[1], Z: values[2], Range: values[3]})
		}
		formations = append(formations, probeFormation{ID: id, Name: name, Probes: probes})
	}
	sort.Slice(formations, func(i, j int) bool { return formations[i].ID < formations[j].ID })
	return formations, nil
}

func runFormation(baseDir string, args []string, output io.Writer) error {
	args, format, err := parseFormationOutput(args)
	if err != nil {
		return err
	}
	if len(args) < 1 || args[0] != "list" || len(args) > 3 {
		return errors.New(formationUsage)
	}
	if format == "json" && len(args) != 3 {
		return fmt.Errorf("JSON output requires an account reference and formation name; %s", formationUsage)
	}
	accounts, err := loadAccounts()
	if err != nil {
		return err
	}
	files := make(map[string]accountFileState)
	var formationName string
	showCoordinates := len(args) == 3
	if showCoordinates {
		formationName = strings.TrimSpace(args[2])
		if strings.TrimSpace(args[1]) == "" || formationName == "" {
			return fmt.Errorf("account reference and formation name are required; %s", formationUsage)
		}
	}
	if len(args) >= 2 {
		ref := parseCharRef(args[1], "Default")
		id, err := resolveAccountID(accounts, ref.Name)
		if err != nil {
			return err
		}
		dir, err := resolveProfileDir(baseDir, "settings_Default", ref.Profile)
		if err != nil {
			return err
		}
		files[filepath.Join(dir, "core_user_"+id+".dat")] = accountFileState{id: id, profile: filepath.Base(dir)}
	} else {
		states, err := scanAccountFiles(baseDir, Accounts{})
		if err != nil {
			return err
		}
		for path, state := range states {
			files[filepath.Join(baseDir, path)] = state
		}
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	var failures []error
	count := 0
	for _, path := range paths {
		formations, err := readProbeFormations(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("read formations from %s: %w", path, err))
			continue
		}
		state := files[path]
		if showCoordinates {
			formation, err := selectProbeFormation(formations, formationName)
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", path, err))
				continue
			}
			formations = []probeFormation{formation}
			if format == "json" {
				return printFormationJSON(output, formation)
			}
		}
		account := state.id
		if name := accounts[state.id]; name != "" {
			account += " (" + strconv.Quote(name) + ")"
		}
		for _, formation := range formations {
			if count == 0 {
				if _, err := fmt.Fprintln(writer, "ACCOUNT\tPROFILE\tID\tNAME\tPROBES"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "%s\t%q\t%d\t%q\t%d\n", account, strings.TrimPrefix(state.profile, "settings_"), formation.ID, formation.Name, len(formation.Probes)); err != nil {
				return err
			}
			if showCoordinates {
				if err := writer.Flush(); err != nil {
					return err
				}
				if err := printFormationProbes(output, formation.Probes); err != nil {
					return err
				}
			}
			count++
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	if count == 0 {
		_, err = fmt.Fprintln(output, "No custom probe formations found.")
	}
	return err
}

func parseFormationOutput(args []string) ([]string, string, error) {
	format := "text"
	var positional []string
	seen := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--":
			return append(positional, args[i+1:]...), format, nil
		case "-o", "--output":
			if seen {
				return nil, "", errors.New("output format may only be specified once")
			}
			if i+1 == len(args) {
				return nil, "", errors.New("output format is required after -o or --output")
			}
			i++
			format, seen = args[i], true
			if format != "json" && format != "text" {
				return nil, "", fmt.Errorf("unsupported output format %q; use json or text", format)
			}
		default:
			positional = append(positional, args[i])
		}
	}
	return positional, format, nil
}

// A portable formation contains no account, profile, or destination-specific ID.
// The version defines probe order and units for a future create/import command.
type formationJSON struct {
	Version int                  `json:"version"`
	Name    string               `json:"name"`
	Probes  []formationProbeJSON `json:"probes"`
}

// Version 2 probes are [north/south km, east/west km, up/down km, range AU].
// Positive values mean north, east, and up; negative values mean south, west,
// and down. EVE stores +X west, +Y up, and +Z north.
type formationProbeJSON [4]float64

func (probe formationProbe) compassCoordinates() formationProbeJSON {
	coordinates := formationProbeJSON{probe.Z / 1000, -probe.X / 1000, probe.Y / 1000, probe.Range / 149597870700}
	// Negating X must not turn a centered probe into a displayed -0.
	for i, value := range coordinates {
		if value == 0 {
			coordinates[i] = 0
		}
	}
	return coordinates
}

func printFormationJSON(output io.Writer, formation probeFormation) error {
	value := formationJSON{Version: 2, Name: formation.Name, Probes: make([]formationProbeJSON, 0, len(formation.Probes))}
	for _, probe := range formation.Probes {
		value.Probes = append(value.Probes, probe.compassCoordinates())
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func selectProbeFormation(formations []probeFormation, name string) (probeFormation, error) {
	var matches []probeFormation
	for _, formation := range formations {
		if strings.EqualFold(formation.Name, name) {
			matches = append(matches, formation)
		}
	}
	if len(matches) == 0 {
		return probeFormation{}, fmt.Errorf("formation not found: %q", name)
	}
	if len(matches) > 1 {
		return probeFormation{}, fmt.Errorf("formation name %q is ambiguous", name)
	}
	return matches[0], nil
}

func printFormationProbes(output io.Writer, probes []formationProbe) error {
	if len(probes) == 0 {
		_, err := fmt.Fprintln(output, "\nNo probes in this formation.")
		return err
	}
	if _, err := fmt.Fprintln(output); err != nil {
		return err
	}
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "PROBE\tNORTH/SOUTH (km)\tEAST/WEST (km)\tUP/DOWN (km)\tRANGE (AU)"); err != nil {
		return err
	}
	for i, probe := range probes {
		coordinates := probe.compassCoordinates()
		if _, err := fmt.Fprintf(writer, "%d\t%g\t%g\t%g\t%g\n", i+1, coordinates[0], coordinates[1], coordinates[2], coordinates[3]); err != nil {
			return err
		}
	}
	return writer.Flush()
}
