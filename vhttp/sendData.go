package vhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/utils"
	"gopkg.in/yaml.v2"
)

func SendData(api_url string, filename string, endPoint string, method string, profile models.Profile, settings models.Settings) {
		var data interface{}

		client := Client(settings.ApiNotSecure)

		// Use APIPrefix from endpoints structure
		connstring := fmt.Sprintf("https://%v:%v%v/%v", api_url, profile.Port, profile.Endpoints.APIPrefix, endPoint)

		// var sendData []byte
		var j []byte
		var err error

		// if usi {
		// 	st, err := io.ReadAll(os.Stdin)
		// 	utils.IsErr(err)
		// 	str := string(st)
		// 	fmt.Printf("%#v", str)
		// 	str = strings.TrimSuffix(str, "\n")
		// 	j = []byte(str)
		// 	err = json.Unmarshal(j, &data)
		// 	utils.IsErr(err)
		// } else if strings.Contains(filename, ".") {
		// 	j, err = os.ReadFile(filename)
		// 	fmt.Printf("%#v", string(j))
		// 	utils.IsErr(err)
		// 	err = json.Unmarshal(j, &data)
		// 	utils.IsErr(err)
		// }
		if strings.Contains(filename, ".") {
			j, err = os.ReadFile(filename)
			utils.IsErr(err)
			if strings.Contains(filename, "yaml") || strings.Contains(filename, "yml") {
				err = yaml.Unmarshal(j, &data)
				utils.IsErr(err)
				// sendData, err = json.Marshal(data)
				// utils.IsErr(err)
			} else {
				err = json.Unmarshal(j, &data)
				utils.IsErr(err)
			}
		}

		sendData, err := json.Marshal(data)
		utils.IsErr(err)

		var r *http.Request
		// var err error

		if strings.Contains(filename, ".") {
			r, err = http.NewRequest(method, connstring, bytes.NewReader(sendData))
			utils.IsErr(err)
		} else {
			r, err = http.NewRequest(method, connstring, nil)
			utils.IsErr(err)
		}
		
		utils.IsErr(err)

		r.Header.Add("accept", profile.Headers.Accept)
		r.Header.Add("Content-Type", "application/json")

		// Get authentication token using TokenManager
		token, err := auth.GetTokenForRequest(settings.SelectedProfile, profile, settings)
		if err != nil {
			log.Fatalf("Authentication failed: %v", err)
		}

		if profile.AuthType == "basic" {
			r.Header.Add("x-RestSvcSessionId", token)
		} else {
			r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
			r.Header.Add("Authorization", "Bearer "+token)
		}

		res, err := client.Do(r)
		if err != nil {
			fmt.Printf("Error sending HTTP request %v\n", err)
			return
		}

		defer res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			body, _ := io.ReadAll(res.Body)
			log.Fatalf("HTTP %d: %s %s\nResponse: %s", res.StatusCode, method, connstring, string(body))
		}

		utils.IsErr(writeSuccessResponse(res, os.Stdout, os.Stderr))
}

// writeSuccessResponse writes the body of a 2xx response to stdout (indented
// when it is JSON, raw otherwise, nothing when empty) and the status lines to
// stderr, so that stdout can be piped into tools such as jq.
func writeSuccessResponse(res *http.Response, stdout, stderr io.Writer) error {
	fmt.Fprintln(stderr, "Status Code:", res.StatusCode)
	fmt.Fprintln(stderr, "Status:", res.Status)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}

	var pretty bytes.Buffer
	if json.Indent(&pretty, body, "", "  ") == nil {
		body = pretty.Bytes()
	}
	_, err = stdout.Write(append(body, '\n'))
	return err
}
