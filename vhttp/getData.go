package vhttp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/utils"
)

type ReadHeaders interface {
	models.BasicAuthModel |  models.SendHeader
}

func GetData[T any](url string, profile models.Profile) T {
	
	settings := utils.ReadSettings()

	// With v1.0 profiles, credentials are always from environment variables
	api_url := os.Getenv("OWLCTL_URL")
	if api_url == "" {
		log.Fatal("OWLCTL_URL environment variable not set")
	}

	// creates a new client instance
	client := Client(settings.ApiNotSecure)

	// Use APIPrefix from endpoints structure
	connstring := fmt.Sprintf("https://%v:%v%v/%v", api_url, profile.Port, profile.Endpoints.APIPrefix, url)

	r, err := http.NewRequest("GET", connstring, nil)
	utils.IsErr(err)
	r.Header.Add("accept", profile.Headers.Accept)

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
	utils.IsErr(err)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		log.Fatalf("HTTP %d: GET %s\nResponse: %s", res.StatusCode, connstring, string(body))
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	utils.IsErr(err)

	var udata T

	if err := json.Unmarshal(body, &udata); err != nil {
		log.Fatalf("Could not unmarshal response from GET %s: %v", connstring, err)
	}

	return udata

}

// GetDataWithError is like GetData but returns errors instead of calling log.Fatal(),
// allowing callers to handle failures gracefully.
func GetDataWithError[T any](url string, profile models.Profile) (T, error) {
	var udata T

	settings := utils.ReadSettings()

	api_url := os.Getenv("OWLCTL_URL")
	if api_url == "" {
		return udata, fmt.Errorf("OWLCTL_URL environment variable not set")
	}

	client := Client(settings.ApiNotSecure)

	connstring := fmt.Sprintf("https://%v:%v%v/%v", api_url, profile.Port, profile.Endpoints.APIPrefix, url)

	r, err := http.NewRequest("GET", connstring, nil)
	if err != nil {
		return udata, fmt.Errorf("failed to create request: %w", err)
	}
	r.Header.Add("accept", profile.Headers.Accept)

	token, err := auth.GetTokenForRequest(settings.SelectedProfile, profile, settings)
	if err != nil {
		return udata, fmt.Errorf("authentication failed: %w", err)
	}

	if profile.AuthType == "basic" {
		r.Header.Add("x-RestSvcSessionId", token)
	} else {
		r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
		r.Header.Add("Authorization", "Bearer "+token)
	}

	res, err := client.Do(r)
	if err != nil {
		return udata, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return udata, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return udata, fmt.Errorf("HTTP %d: GET %s\nResponse: %s", res.StatusCode, connstring, string(body))
	}

	if err := json.Unmarshal(body, &udata); err != nil {
		return udata, fmt.Errorf("could not unmarshal response from GET %s: %w", connstring, err)
	}

	return udata, nil
}
