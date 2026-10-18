package cmd

import (
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
)

// var usi bool

// postCmd represents the post command
var postCmd = &cobra.Command{
	Use:   "post",
	Short: "Sends a POST command to API",
	Long: `Sends a POST commands to the selected profile.

Payload needs to be in the JSON format.

Note that owlctl does not type check the payload.

On success the response body is printed to stdout (pretty-printed when it is
JSON, raw otherwise, nothing when empty) and the status lines go to stderr, so
the output can be piped, e.g. owlctl post jobs/<id>/start | jq .id

Commands:
owlctl post jobs/c69eb538-5a07-4bd7-80cb-bdf5142eadd6/start
owlctl post jobs -f job.json

	`,
	Run: func(cmd *cobra.Command, args []string) {

		profile := utils.GetCurrentProfile()
		settings := utils.ReadSettings()

		api_url := utils.GetAddress(profile, settings)

		vhttp.SendData(api_url, filename, args[0], "POST", profile, settings)

	},
}

func init() {
	postCmd.Flags().StringVarP(&filename, "file", "f", "", "payload in json format")
	// postCmd.Flags().BoolVarP(&usi, "stdin", "s", false, "read payload from stdin")
	// postCmd.MarkFlagRequired("filename")
	rootCmd.AddCommand(postCmd)

}
