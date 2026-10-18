package cmd

import (
	"log"
	"os"
	"strings"

	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
)

var (filename string
	//  usiP bool
	)


// postCmd represents the post command
var putCmd = &cobra.Command{
	Use:   "put",
	Short: "Sends a PUT command to API",
	Long: `Sends a PUT commands to the selected profile.

Payload needs to be in the JSON format. PUTs always require a payload.

Note that owlctl does not type check the payload.

On success the response body is printed to stdout (pretty-printed when it is
JSON, raw otherwise, nothing when empty) and the status lines go to stderr, so
the output can be piped, e.g. owlctl put jobs -f job.json | jq .id

Commands:
owlctl put jobs -f job.json

	`,
	Run: func(cmd *cobra.Command, args []string) {

		// check if filename is set
		if !strings.Contains(filename, ".json") {
			log.Fatal("A JSON file is required for PUT commands")
		}
		
		profile := utils.GetCurrentProfile()
		settings := utils.ReadSettings()

		// With v1.0 profiles, credentials are always from environment variables
		api_url := os.Getenv("OWLCTL_URL")
		if api_url == "" {
			log.Fatal("OWLCTL_URL environment variable not set")
		}

		vhttp.SendData(api_url, filename, args[0], "PUT", profile, settings)

	},
}

func init() {
	putCmd.Flags().StringVarP(&filename, "file", "f", "", "payload in json format")
	// putCmd.Flags().BoolVarP(&usiP, "stdin", "s", false, "read payload from stdin")
	// putCmd.MarkFlagRequired("filename")
	rootCmd.AddCommand(putCmd)

}
