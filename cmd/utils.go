package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
)

var utilsCmd = &cobra.Command{
	Use:   "utils",
	Short: "Utilities",
	Long: `Utilities that help with specific API related tasks.
	
Current utilities:

VBR Job JSON GET to POST converter - Converts a VBR Job GET JSON file to a VBR Job POST JSON file 

	`,
	Run: func(cmd *cobra.Command, args []string) {
		result, _ := pterm.DefaultInteractiveSelect.
		WithOptions([]string{"VBR Job JSON Converter", "Check Version"}).Show()

		if result == "VBR Job JSON Converter" {
			js, _ := pterm.DefaultInteractiveTextInput.Show("Enter source file path, e.g. C:\\temp\\job.json")
			jt, _ := pterm.DefaultInteractiveTextInput.Show("Enter target file path, e.g. C:\\temp\\job_updated.json (leave blank to overwrite source file)")

			if (len(jt) == 0) {
				jt = js
			}

			var varJobs models.VbrJobGet

			getJson, err := os.Open(js)
			utils.IsErr(err)
			
			b, err := io.ReadAll(getJson)
			utils.IsErr(err)

			err = json.Unmarshal(b, &varJobs)
			utils.IsErr(err)

			var inc []models.Includes

			// iterate through the source job vm includes and create a new includes struct
			for _, v := range varJobs.VirtualMachines.Includes {
				ti := models.Includes{
					Type: v.Type,
					HostName: v.HostName,
					Name: v.Name,
					ObjectID: v.ObjectID,
				}

				inc = append(inc, ti)
			}

			// create a new Virtual Machines struct instance
			varVMs := models.VirtualMachines{}
			varVMs.Includes = inc
			varVMs.Excludes = varJobs.VirtualMachines.Excludes

			// create a new post job struct instance
			varJobsPost := models.VbrJobPost{}

			varJobsPost.IsHighPriority = varJobs.IsHighPriority
			varJobsPost.VirtualMachines = varVMs
			varJobsPost.Storage = varJobs.Storage
			varJobsPost.Schedule = varJobs.Schedule
			varJobsPost.GuestProcessing = varJobs.GuestProcessing
			varJobsPost.Type = varJobs.Type
			varJobsPost.Name = varJobs.Name
			varJobsPost.Description = varJobs.Description
			varJobsPost.IsDisabled = varJobs.IsDisabled

			if (strings.Contains(jt, ".json")) {
				jt = strings.TrimSuffix(jt, filepath.Ext(jt))
			}

			utils.SaveJson(&varJobsPost, jt)

			fmt.Println("Job JSON POST file created.")
		} else if (result == "Check Version") {
			res, err := http.Get("https://api.github.com/repos/shapedthought/owlctl/releases/latest");
			utils.IsErr(err)

			body, err := io.ReadAll(res.Body)
			defer res.Body.Close()
			utils.IsErr(err)

			var result map[string]interface{}

			json.Unmarshal(body, &result)

			latestVersion, err := latestTag(result)
			utils.IsErr(err)

			fmt.Println(compareVersion(version, latestVersion))
		}
	},
}

func init() {
	rootCmd.AddCommand(utilsCmd)
}