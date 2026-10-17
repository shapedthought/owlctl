package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build metadata, injected at build time with -ldflags -X, e.g.
//
//	-X github.com/shapedthought/owlctl/cmd.version=v1.3.0
//	-X github.com/shapedthought/owlctl/cmd.commit=<sha>
//	-X github.com/shapedthought/owlctl/cmd.date=<rfc3339>
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build information",
	Args:  cobra.NoArgs,
	// Skip the root pre-run (instance activation): version needs no config.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "owlctl version %s\n", version)
		fmt.Fprintf(out, "commit: %s\n", commit)
		fmt.Fprintf(out, "built: %s\n", date)
		fmt.Fprintf(out, "go: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

// compareVersion describes how the running version relates to the latest
// release tag. It is pure so it can be tested without network access.
func compareVersion(current, latest string) string {
	if current == "dev" {
		return fmt.Sprintf("You are running a development build, the latest release is %s", latest)
	}
	if current != latest {
		return fmt.Sprintf("You are running %s, the latest version is %s", current, latest)
	}
	return "You are running the latest version."
}

// latestTag extracts tag_name from a GitHub releases/latest response,
// returning an error (including GitHub's message, e.g. rate limiting)
// when it is missing.
func latestTag(result map[string]interface{}) (string, error) {
	if tag, ok := result["tag_name"].(string); ok && tag != "" {
		return tag, nil
	}
	if msg, ok := result["message"].(string); ok && msg != "" {
		return "", fmt.Errorf("could not determine latest release: %s", msg)
	}
	return "", fmt.Errorf("could not determine latest release: unexpected response")
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(versionCmd)
}
