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

// fhirObservation represents the structure needed to create the Observation resource.
// Based on Appendix A.1 of pdr.md.
type fhirObservation struct {
	ResourceType      string         `json:"resourceType"`
	Status            string         `json:"status"`
	Category          []fhirCategory `json:"category"`
	Code              fhirCode       `json:"code"`
	Subject           fhirReference  `json:"subject"`
	EffectiveDateTime string         `json:"effectiveDateTime"`
	ValueInteger      int            `json:"valueInteger"`
}

type fhirCategory struct {
	Coding []fhirCoding `json:"coding"`
	Text   string       `json:"text,omitempty"`
}

type fhirCoding struct {
	System  string `json:"system"`
	Code    string `json:"code"`
	Display string `json:"display"`
}

type fhirCode struct {
	Coding []fhirCoding `json:"coding"`
	Text   string       `json:"text"`
}

type fhirReference struct {
	Reference string `json:"reference"`
}

// createdResource is used to unmarshal the response from FHIR server
// to extract the ID of the newly created resource.
type createdResource struct {
	ID string `json:"id"`
}

// CreateObservation sends a POST request to the Oystehr FHIR API to create an Observation resource.
// It returns the ID of the created Observation or an error.
func CreateObservation(httpClient *http.Client, cfg *config.Config, token string, patientID string, totalScore int) (string, error) {
	// Use a default client if none is provided
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	// Construct the FHIR Observation payload
	obs := fhirObservation{
		ResourceType: "Observation",
		Status:       "final",
		Category: []fhirCategory{{
			Coding: []fhirCoding{{
				System:  "http://terminology.hl7.org/CodeSystem/observation-category",
				Code:    "survey",
				Display: "Survey",
			}},
		}},
		Code: fhirCode{
			Coding: []fhirCoding{{
				System:  "http://loinc.org",
				Code:    "99046-5",
				Display: "Total score [EPDS]",
			}},
			Text: "EPDS Total Score",
		},
		Subject:           fhirReference{Reference: fmt.Sprintf("Patient/%s", patientID)},
		EffectiveDateTime: time.Now().Format(time.RFC3339), // ISO8601 Format
		ValueInteger:      totalScore,
	}

	obsBytes, err := json.Marshal(obs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal FHIR Observation JSON: %w", err)
	}

	// Construct the request URL
	url := cfg.OystehrFHIRBaseURL + "/Observation" // Assuming base URL does not end with /
	// TODO: Consider adding a check/fix for trailing slash in base URL

	// Create the HTTP request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(obsBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create FHIR Observation request: %w", err)
	}

	// Set required headers (as per Section 6.2)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-zapehr-project-id", cfg.OystehrProjectID)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")

	// Execute the request
	log.Printf("Sending POST request to %s to create Observation for Patient %s", url, patientID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute FHIR Observation request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("Warning: failed to read response body after status %d: %v", resp.StatusCode, readErr)
		// Continue processing status code, but body might be unavailable for error reporting
	}

	// Check response status code
	if resp.StatusCode != http.StatusCreated { // 201 Created
		errBody := string(bodyBytes)
		if errBody == "" && readErr != nil {
			errBody = fmt.Sprintf("(could not read body: %v)", readErr)
		}
		log.Printf("ERROR: FHIR Observation creation failed. Status: %d, Body: %s", resp.StatusCode, errBody)
		return "", fmt.Errorf("FHIR API error creating Observation (status %d): %s", resp.StatusCode, errBody)
	}

	// Parse the response body to get the created resource ID
	var createdObs createdResource
	if err := json.Unmarshal(bodyBytes, &createdObs); err != nil {
		// Log the body if unmarshalling fails, it might not be the expected format
		log.Printf("ERROR: Failed to unmarshal FHIR Observation response body: %s. Error: %v", string(bodyBytes), err)
		return "", fmt.Errorf("failed to parse FHIR Observation response body: %w", err)
	}

	if createdObs.ID == "" {
		log.Printf("ERROR: FHIR Observation created (201) but response did not contain an ID. Body: %s", string(bodyBytes))
		return "", fmt.Errorf("FHIR Observation created but response missing ID")
	}

	log.Printf("Successfully created FHIR Observation with ID: %s for Patient %s", createdObs.ID, patientID)
	return createdObs.ID, nil
}
