package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	OystehrFHIRBaseURL     string
	OystehrAuthURL         string
	OystehrProjectID       string
	OystehrM2MClientID     string
	OystehrM2MClientSecret string
	AlertProviderFHIRID    string
	Port                   string // Optional port from environment
}

// LoadConfig reads required environment variables and returns a Config struct.
// It returns an error if any required variable is missing.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		OystehrFHIRBaseURL:     os.Getenv("OYSTEHR_FHIR_BASE_URL"),
		OystehrAuthURL:         os.Getenv("OYSTEHR_AUTH_URL"),
		OystehrProjectID:       os.Getenv("OYSTEHR_PROJECT_ID"),
		OystehrM2MClientID:     os.Getenv("OYSTEHR_M2M_CLIENT_ID"),
		OystehrM2MClientSecret: os.Getenv("OYSTEHR_M2M_CLIENT_SECRET"),
		AlertProviderFHIRID:    os.Getenv("ALERT_PROVIDER_FHIR_ID"),
		Port:                   os.Getenv("PORT"),
	}

	// Validate required fields
	if cfg.OystehrFHIRBaseURL == "" {
		return nil, fmt.Errorf("required environment variable OYSTEHR_FHIR_BASE_URL is not set")
	}
	if cfg.OystehrAuthURL == "" {
		return nil, fmt.Errorf("required environment variable OYSTEHR_AUTH_URL is not set")
	}
	if cfg.OystehrProjectID == "" {
		return nil, fmt.Errorf("required environment variable OYSTEHR_PROJECT_ID is not set")
	}
	if cfg.OystehrM2MClientID == "" {
		return nil, fmt.Errorf("required environment variable OYSTEHR_M2M_CLIENT_ID is not set")
	}
	if cfg.OystehrM2MClientSecret == "" {
		return nil, fmt.Errorf("required environment variable OYSTEHR_M2M_CLIENT_SECRET is not set")
	}
	if cfg.AlertProviderFHIRID == "" {
		return nil, fmt.Errorf("required environment variable ALERT_PROVIDER_FHIR_ID is not set")
	}

	// Set default port if not provided
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}
