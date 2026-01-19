package main

import (
	"fmt"
	"os"
)

var Verbose bool

func vlog(format string, args ...any) {
	if !Verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[verbose] ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}
