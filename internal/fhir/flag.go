package fhir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"example.com/epds-service/internal/config"
)

// fhirFlag represents the structure needed to create the Flag resource.
// Based on Appendix A.2 of pdr.md.
type fhirFlag struct {
	ResourceType string         `json:"resourceType"`
	Status       string         `json:"status"`
	Category     []fhirCategory `json:"category"`            // Reusing from observation.go (implicitly)
	Code         fhirCode       `json:"code"`                // Reusing from observation.go (implicitly)
	Subject      fhirReference  `json:"subject"`             // Reusing from observation.go (implicitly)
	Encounter    *fhirReference `json:"encounter,omitempty"` // Add Encounter field
	Meta         *fhirMeta      `json:"meta,omitempty"`      // Add Meta field
}

// fhirMeta defines the structure for the meta field, including tags.
// Assuming fhirCoding is defined elsewhere in the package.
type fhirMeta struct {
	Tag []fhirCoding `json:"tag,omitempty"`
}

// Note: fhirCategory, fhirCoding, fhirCode, fhirReference, and createdResource are assumed
// to be defined in the same package (e.g., in observation.go or a common types file).
// If they are not accessible, they would need to be redefined or imported.

// CreateFlag sends a POST request to the Oystehr FHIR API to create a Flag resource.
// It returns the ID of the created Flag or an error.
func CreateFlag(httpClient *http.Client, cfg *config.Config, token string, patientID string, encounterID string, totalScore int, q10Score int) (string, error) {
	// Use a default client if none is provided
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	// Construct the FHIR Flag payload
	flag := fhirFlag{
		ResourceType: "Flag",
		Status:       "active",
		Category: []fhirCategory{{
			Coding: []fhirCoding{{
				System:  "http://example.org/codes", // Using example from PRD
				Code:    "epds-high-risk",
				Display: "EPDS High Risk Alert",
			}},
			Text: "High EPDS Score or Self-Harm Risk Reported", // Adding text as per example
		}},
		Code: fhirCode{
			Coding: []fhirCoding{}, // Add empty coding slice
			// No specific coding provided in PRD Appendix A.2, only text
			Text: fmt.Sprintf("High EPDS Score (%d) or Q10 Risk (%d) indicated.", totalScore, q10Score),
		},
		Subject: fhirReference{Reference: fmt.Sprintf("Patient/%s", patientID)},
	}

	// Add Meta tag
	flag.Meta = &fhirMeta{
		Tag: []fhirCoding{{
			System:  "urn:cornell:epds:tags",
			Code:    "epds-high-risk",
			Display: "EPDS High Risk Indicator",
		}},
	}

	// Add Encounter if encounterID is provided
	if encounterID != "" {
		flag.Encounter = &fhirReference{Reference: fmt.Sprintf("Encounter/%s", encounterID)}
	}

	flagBytes, err := json.Marshal(flag)
	if err != nil {
		return "", fmt.Errorf("failed to marshal FHIR Flag JSON: %w", err)
	}

	// Construct the request URL
	url := cfg.OystehrFHIRBaseURL + "/Flag"

	// Create the HTTP request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(flagBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create FHIR Flag request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-zapehr-project-id", cfg.OystehrProjectID)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")

	// Execute the request
	log.Printf("Sending POST request to %s to create Flag for Patient %s", url, patientID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute FHIR Flag request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("Warning: failed to read response body after status %d for Flag creation: %v", resp.StatusCode, readErr)
	}

	// Check response status code
	if resp.StatusCode != http.StatusCreated { // 201 Created
		errBody := string(bodyBytes)
		if errBody == "" && readErr != nil {
			errBody = fmt.Sprintf("(could not read body: %v)", readErr)
		}
		log.Printf("ERROR: FHIR Flag creation failed. Status: %d, Body: %s", resp.StatusCode, errBody)
		return "", fmt.Errorf("FHIR API error creating Flag (status %d): %s", resp.StatusCode, errBody)
	}

	// Parse the response body to get the created resource ID
	var createdFlag createdResource // Reusing the struct from observation.go
	if err := json.Unmarshal(bodyBytes, &createdFlag); err != nil {
		log.Printf("ERROR: Failed to unmarshal FHIR Flag response body: %s. Error: %v", string(bodyBytes), err)
		return "", fmt.Errorf("failed to parse FHIR Flag response body: %w", err)
	}

	if createdFlag.ID == "" {
		log.Printf("ERROR: FHIR Flag created (201) but response did not contain an ID. Body: %s", string(bodyBytes))
		return "", fmt.Errorf("FHIR Flag created but response missing ID")
	}

	log.Printf("Successfully created FHIR Flag with ID: %s for Patient %s", createdFlag.ID, patientID)
	return createdFlag.ID, nil
}
