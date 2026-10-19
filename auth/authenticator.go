package auth

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/shapedthought/owlctl/models"
)

// Authenticator handles API authentication
type Authenticator struct {
	client *http.Client
	debug  bool
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(insecure bool, debug bool) *Authenticator {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}

	return &Authenticator{
		client: &http.Client{Transport: tr},
		debug:  debug,
	}
}

// AuthResult contains authentication response
type AuthResult struct {
	Token       string
	ExpiresIn   int
	TokenType   string
	IsBasicAuth bool
	// RefreshToken is the single-use OAuth refresh token (empty for Basic Auth).
	RefreshToken string
	// RefreshExpiresAt is the refresh token's expiry from its JWT exp claim;
	// zero when the token is opaque or carries no exp.
	RefreshExpiresAt time.Time
}

// ErrRefreshRejected is returned by Refresh when the server refuses the refresh
// token (expired, already used or revoked). The caller should fall back to a
// password login.
var ErrRefreshRejected = errors.New("refresh token rejected")

// logoutPath is VBR's session revocation endpoint.
const logoutPath = "/api/oauth2/logout"

// jwtExpiry reads the exp claim of a JWT without verifying the signature. It is
// used only to schedule renewal, never to trust the token.
func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp json.Number `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == "" {
		return time.Time{}, false
	}
	f, err := claims.Exp.Float64()
	if err != nil || f <= 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(f), 0), true
}

// Authenticate performs OAuth or Basic Auth login
func (a *Authenticator) Authenticate(profile models.Profile, username, password, apiURL string) (*AuthResult, error) {
	// Build connection string using new Endpoints structure
	connstring := fmt.Sprintf("https://%s:%d%s", apiURL, profile.Port, profile.Endpoints.Auth)

	if a.debug {
		fmt.Printf("DEBUG: Authenticating to %s\n", connstring)
	}

	var r *http.Request
	var err error

	if profile.AuthType == "basic" {
		// Enterprise Manager uses Basic Auth
		r, err = http.NewRequest("POST", connstring, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		r.Header.Add("accept", profile.Headers.Accept)
		r.SetBasicAuth(username, password)
	} else {
		// Other products use OAuth
		data := url.Values{}
		data.Add("grant_type", "password")
		data.Add("username", username)
		data.Add("password", password)

		r, err = http.NewRequest("POST", connstring, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		r.Header.Add("accept", profile.Headers.Accept)
		r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
		r.Header.Add("Content-Type", profile.Headers.ContentType)
	}

	res, err := a.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("authentication request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid credentials (401)")
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("authentication failed with status %d: %s", res.StatusCode, string(body))
	}

	if profile.AuthType == "basic" {
		// Extract session token from header
		token := res.Header.Get("X-RestSvcSessionId")
		if token == "" {
			return nil, fmt.Errorf("no session token in response")
		}

		return &AuthResult{
			Token:       token,
			ExpiresIn:   3600, // Enterprise Manager doesn't provide expiry, default to 1 hour
			TokenType:   "session",
			IsBasicAuth: true,
		}, nil
	}
	return parseOAuthResponse(res)
}

// parseOAuthResponse converts a successful OAuth token response into an AuthResult.
func parseOAuthResponse(res *http.Response) (*AuthResult, error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var oauthResp models.SendHeader
	if err := json.Unmarshal(body, &oauthResp); err != nil {
		return nil, fmt.Errorf("failed to parse OAuth response: %w", err)
	}

	if oauthResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}

	result := &AuthResult{
		Token:        oauthResp.AccessToken,
		ExpiresIn:    oauthResp.ExpiresIn,
		TokenType:    oauthResp.TokenType,
		RefreshToken: oauthResp.RefreshToken,
	}
	if exp, ok := jwtExpiry(oauthResp.RefreshToken); ok {
		result.RefreshExpiresAt = exp
	}
	return result, nil
}

// Refresh exchanges a refresh token for a new access/refresh token pair
// (grant_type=refresh_token). VBR refresh tokens are single-use, so the caller
// must persist the returned pair immediately. A 400/401 response yields
// ErrRefreshRejected.
func (a *Authenticator) Refresh(profile models.Profile, refreshToken, apiURL string) (*AuthResult, error) {
	connstring := fmt.Sprintf("https://%s:%d%s", apiURL, profile.Port, profile.Endpoints.Auth)

	if a.debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Refreshing token at %s\n", connstring)
	}

	data := url.Values{}
	data.Add("grant_type", "refresh_token")
	data.Add("refresh_token", refreshToken)

	r, err := http.NewRequest("POST", connstring, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	r.Header.Add("accept", profile.Headers.Accept)
	r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
	r.Header.Add("Content-Type", profile.Headers.ContentType)

	res, err := a.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w (%d)", ErrRefreshRejected, res.StatusCode)
	}
	if res.StatusCode != 200 && res.StatusCode != 201 {
		return nil, fmt.Errorf("token refresh failed with status %d", res.StatusCode)
	}
	return parseOAuthResponse(res)
}

// Logout revokes the session on the server (POST /api/oauth2/logout with the
// access token). VBR also invalidates the session's refresh token.
func (a *Authenticator) Logout(profile models.Profile, token, apiURL string) error {
	connstring := fmt.Sprintf("https://%s:%d%s", apiURL, profile.Port, logoutPath)

	r, err := http.NewRequest("POST", connstring, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	r.Header.Add("accept", profile.Headers.Accept)
	r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
	r.Header.Add("Authorization", "Bearer "+token)

	res, err := a.client.Do(r)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("logout failed with status %d", res.StatusCode)
	}
	return nil
}

// ExtractTokenInfo parses token from OAuth response body (for legacy compatibility)
func ExtractTokenInfo(responseBody []byte, isBasicAuth bool) (string, int, error) {
	if isBasicAuth {
		var authModel models.BasicAuthModel
		if err := json.Unmarshal(responseBody, &authModel); err != nil {
			return "", 0, fmt.Errorf("failed to parse basic auth response: %w", err)
		}
		return authModel.Token, 3600, nil
	}

	var oauthResp models.SendHeader
	if err := json.Unmarshal(responseBody, &oauthResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse OAuth response: %w", err)
	}

	return oauthResp.AccessToken, oauthResp.ExpiresIn, nil
}

// FormatTokenForHeaders formats token for headers.json (legacy format)
func FormatTokenForHeaders(token string, expiresIn int, tokenType string, isBasicAuth bool) []byte {
	if isBasicAuth {
		aum := models.BasicAuthModel{
			Token:       token,
			ContentType: "application/json",
		}
		tokenBytes := new(bytes.Buffer)
		json.NewEncoder(tokenBytes).Encode(aum)
		return tokenBytes.Bytes()
	}

	header := models.SendHeader{
		AccessToken: token,
		TokenType:   tokenType,
		ExpiresIn:   expiresIn,
	}
	tokenBytes := new(bytes.Buffer)
	json.NewEncoder(tokenBytes).Encode(header)
	return tokenBytes.Bytes()
}
