package vhttp

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/models"
)

// errAuth marks failures to obtain or renew an authentication token, as
// opposed to transport errors.
var errAuth = errors.New("authentication failed")

// doAuthed sends the request built by newReq with a bearer (or session) token.
// If a bearer-token request is answered 401 it renews the token once and sends
// it once more; it never loops. newReq must return a fresh request each call
// (a request body cannot be re-read) and must not set the auth header itself.
// Authentication failures are wrapped with errAuth.
func doAuthed(client *http.Client, profile models.Profile, settings models.Settings, newReq func() (*http.Request, error)) (*http.Response, error) {
	token, err := auth.GetTokenForRequest(settings.SelectedProfile, profile, settings)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAuth, err)
	}

	res, err := sendWithToken(client, profile, newReq, token)
	if err != nil || res.StatusCode != http.StatusUnauthorized || profile.AuthType == "basic" {
		// Enterprise Manager sessions have no refresh token; its 401 is reported as before.
		return res, err
	}

	// 401: the token was revoked or expired early. Renew once and retry once.
	_, _ = io.Copy(io.Discard, res.Body)
	res.Body.Close()
	renewed, err := auth.RenewTokenAfter401(settings.SelectedProfile, profile, settings, token)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAuth, err)
	}
	return sendWithToken(client, profile, newReq, renewed)
}

func sendWithToken(client *http.Client, profile models.Profile, newReq func() (*http.Request, error), token string) (*http.Response, error) {
	r, err := newReq()
	if err != nil {
		return nil, err
	}
	if profile.AuthType == "basic" {
		r.Header.Add("x-RestSvcSessionId", token)
	} else {
		r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
		r.Header.Add("Authorization", "Bearer "+token)
	}
	return client.Do(r)
}

// requestError turns an error from doAuthed into the form callers report:
// authentication errors are already descriptive, transport errors get a prefix.
func requestError(err error) error {
	if errors.Is(err, errAuth) {
		return err
	}
	return fmt.Errorf("request failed: %w", err)
}
