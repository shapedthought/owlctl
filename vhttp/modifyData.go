package vhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/utils"
)

// PostData sends a POST request with data and returns the response
func PostData[T any](url string, data interface{}, profile models.Profile) T {
	return sendRequest[T]("POST", url, data, profile)
}

// PutData sends a PUT request with data
func PutData(url string, data interface{}, profile models.Profile) {
	sendRequest[interface{}]("PUT", url, data, profile)
}

// PutDataWithError sends a PUT request with data and returns the response body and any error.
// Unlike PutData, this function returns errors instead of calling log.Fatal(),
// allowing callers to handle failures gracefully.
func PutDataWithError(url string, data interface{}, profile models.Profile) ([]byte, error) {
	return sendRequestWithError("PUT", url, data, profile)
}

// PostDataWithError sends a POST request with data and returns the response body and any error.
// Unlike PostData, this function returns errors instead of calling log.Fatal(),
// allowing callers to handle failures gracefully.
func PostDataWithError(url string, data interface{}, profile models.Profile) ([]byte, error) {
	return sendRequestWithError("POST", url, data, profile)
}

// sendRequestWithError is like sendRequest but returns errors instead of calling log.Fatal()
func sendRequestWithError(method string, url string, data interface{}, profile models.Profile) ([]byte, error) {
	settings := utils.ReadSettings()

	// With v1.0 profiles, credentials are always from environment variables
	api_url := os.Getenv("OWLCTL_URL")
	if api_url == "" {
		return nil, fmt.Errorf("OWLCTL_URL environment variable not set")
	}

	client := Client(settings.ApiNotSecure)

	// Use APIPrefix from endpoints structure
	connstring := fmt.Sprintf("https://%v:%v%v/%v", api_url, profile.Port, profile.Endpoints.APIPrefix, url)

	var jsonData []byte
	if data != nil {
		var err error
		jsonData, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	res, err := doAuthed(client, profile, settings, func() (*http.Request, error) {
		var reqBody io.Reader
		if jsonData != nil {
			reqBody = bytes.NewReader(jsonData)
		}
		r, err := http.NewRequest(method, connstring, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		r.Header.Add("accept", profile.Headers.Accept)
		r.Header.Add("Content-Type", "application/json")
		return r, nil
	})
	if err != nil {
		return nil, requestError(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return body, fmt.Errorf("HTTP %d: %s %s\nResponse: %s", res.StatusCode, method, connstring, string(body))
	}

	return body, nil
}

// DeleteData sends a DELETE request
func DeleteData(url string, profile models.Profile) {
	sendRequest[interface{}]("DELETE", url, nil, profile)
}

// sendRequest is a generic function for sending HTTP requests
func sendRequest[T any](method string, url string, data interface{}, profile models.Profile) T {
	settings := utils.ReadSettings()

	// With v1.0 profiles, credentials are always from environment variables
	api_url := os.Getenv("OWLCTL_URL")
	if api_url == "" {
		log.Fatal("OWLCTL_URL environment variable not set")
	}

	client := Client(settings.ApiNotSecure)

	// Use APIPrefix from endpoints structure
	connstring := fmt.Sprintf("https://%v:%v%v/%v", api_url, profile.Port, profile.Endpoints.APIPrefix, url)

	var jsonData []byte
	if data != nil {
		var err error
		jsonData, err = json.Marshal(data)
		utils.IsErr(err)
	}

	res, err := doAuthed(client, profile, settings, func() (*http.Request, error) {
		var reqBody io.Reader
		if jsonData != nil {
			reqBody = bytes.NewReader(jsonData)
		}
		r, err := http.NewRequest(method, connstring, reqBody)
		if err != nil {
			return nil, err
		}
		r.Header.Add("accept", profile.Headers.Accept)
		r.Header.Add("Content-Type", "application/json")
		return r, nil
	})
	utils.IsErr(err)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		log.Fatalf("HTTP %d: %s %s\nResponse: %s", res.StatusCode, method, connstring, string(body))
	}

	defer res.Body.Close()

	// For methods that expect a response body
	if method == "POST" || method == "GET" {
		body, err := io.ReadAll(res.Body)
		utils.IsErr(err)

		var result T
		if len(body) > 0 {
			if err := json.Unmarshal(body, &result); err != nil {
				log.Fatalf("Could not unmarshal response - %v", err)
			}
		}
		return result
	}

	// For PUT/DELETE, return empty result
	var result T
	return result
}
