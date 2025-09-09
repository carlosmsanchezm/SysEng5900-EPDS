package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"example.com/epds-service/internal/config" // Assuming this is your module path
)

// AuthResponse represents the successful JSON response from the Oystehr auth endpoint.
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"` // Oystehr returns expires_in in seconds
}

// AuthErrorResponse represents a potential error JSON response from Oystehr auth.
type AuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Authenticator handles fetching and caching of Oystehr M2M tokens.
type Authenticator struct {
	config      *config.Config
	httpClient  *http.Client
	token       string
	expiry      time.Time
	mutex       sync.RWMutex
	tokenBuffer time.Duration // Buffer before actual expiry to refresh token
}

// NewAuthenticator creates a new Authenticator instance.
func NewAuthenticator(cfg *config.Config, client *http.Client) *Authenticator {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second} // Default client with timeout
	}
	return &Authenticator{
		config:      cfg,
		httpClient:  client,
		tokenBuffer: 5 * time.Minute, // Refresh token 5 minutes before it expires
	}
}

// GetAuthToken retrieves a valid Oystehr access token, fetching a new one if necessary.
func (a *Authenticator) GetAuthToken() (string, error) {
	a.mutex.RLock()
	// Check if the current token is valid and not nearing expiry
	if a.token != "" && time.Now().Before(a.expiry.Add(-a.tokenBuffer)) {
		token := a.token
		a.mutex.RUnlock()
		log.Println("Using cached Oystehr token")
		return token, nil
	}
	a.mutex.RUnlock()

	// If token is invalid or nearing expiry, acquire write lock and fetch new token
	return a.fetchNewToken()
}

// fetchNewToken performs the POST request to get a new token.
func (a *Authenticator) fetchNewToken() (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Double-check if another goroutine fetched the token while waiting for the lock
	if a.token != "" && time.Now().Before(a.expiry.Add(-a.tokenBuffer)) {
		log.Println("Another routine refreshed the token while waiting for lock")
		return a.token, nil
	}

	log.Println("Fetching new Oystehr token...")

	// Prepare request body according to Appendix A.4
	reqBodyMap := map[string]string{
		"client_id":     a.config.OystehrM2MClientID,
		"client_secret": a.config.OystehrM2MClientSecret,
		"grant_type":    "client_credentials",
		"audience":      "https://api.zapehr.com", // As specified in Appendix A.4
	}
	reqBodyBytes, err := json.Marshal(reqBodyMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth request body: %w", err)
	}

	// Create POST request
	req, err := http.NewRequest("POST", a.config.OystehrAuthURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute auth request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read auth response body: %w", err)
	}

	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		var errResp AuthErrorResponse
		if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error != "" {
			// Try to parse Oystehr error format
			return "", fmt.Errorf("oystehr auth API error (%d): %s - %s", resp.StatusCode, errResp.Error, errResp.ErrorDescription)
		}
		// Fallback error message
		return "", fmt.Errorf("oystehr auth API request failed with status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse successful response
	var authResp AuthResponse
	if err := json.Unmarshal(bodyBytes, &authResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal auth response JSON: %w", err)
	}

	if authResp.AccessToken == "" {
		return "", fmt.Errorf("received empty access token from Oystehr auth API")
	}

	// Store the new token and expiry time
	a.token = authResp.AccessToken
	a.expiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)
	log.Printf("Successfully fetched new Oystehr token. Expires in: %d seconds", authResp.ExpiresIn)

	return a.token, nil
}
