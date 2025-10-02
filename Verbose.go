package main

import (
	"fmt"
	"os"
)

var Verbose bool

func vlog(format string, args ...any) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
	}
}
