package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// deprecatedOverlayShorthandWarning is printed to stderr when -o is used for
// --overlay on 'job apply' / 'job plan'. Remove the shorthand in the next minor release.
const deprecatedOverlayShorthandWarning = "Warning: -o for --overlay is deprecated; use --overlay."

// usesOverlayShorthand reports whether args contain the -o shorthand (-o FILE,
// -o=FILE or -oFILE). Scanning stops at "--". The overlay flag is the only
// shorthand on apply/plan, so any "-o..." token is the shorthand.
func usesOverlayShorthand(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if strings.HasPrefix(a, "-o") && !strings.HasPrefix(a, "--") {
			return true
		}
	}
	return false
}

func warnDeprecatedOverlayShorthand(args []string) {
	writeOverlayShorthandWarning(os.Stderr, args)
}

func writeOverlayShorthandWarning(w io.Writer, args []string) {
	if usesOverlayShorthand(args) {
		fmt.Fprintln(w, deprecatedOverlayShorthandWarning)
	}
}
