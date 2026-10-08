package main

import (
	"fmt"
	"io"
	"runtime"
)

// Release builds set version to the Git tag using -ldflags.
var version = "development"

func runAbout(args []string, output io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: eves about")
	}
	_, err := fmt.Fprintf(output, `eves %s (%s/%s) - EVE Online settings manager
Copy settings, look up names, inspect probe formations, and export JSON.
Author: Losobie - Clento Loso (in-game)
Repository: https://github.com/Losobie/eves
License: MIT - https://github.com/Losobie/eves/blob/main/LICENSE
Decoder: TrueBrain/blue-marshal-rs (MIT; includes CCP Games notices)
         https://github.com/Losobie/eves/blob/main/internal/bluemarshal/LICENSE
`, version, runtime.GOOS, runtime.GOARCH)
	return err
}
