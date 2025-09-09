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

// fhirCommunication represents the structure needed to create the Communication resource.
// Based on Appendix A.3 of pdr.md.
type fhirCommunication struct {
	ResourceType string          `json:"resourceType"`
	Status       string          `json:"status"`
	Category     []fhirCategory  `json:"category"`  // Reusing from observation.go
	Subject      fhirReference   `json:"subject"`   // Reusing from observation.go
	Recipient    []fhirReference `json:"recipient"` // Reusing fhirReference
	Payload      []fhirPayload   `json:"payload"`
	Sent         string          `json:"sent"`
}

type fhirPayload struct {
	ContentString string `json:"contentString"`
}

// Note: fhirCategory, fhirCoding, fhirReference, and createdResource are assumed
// to be defined in the same package (e.g., in observation.go or flag.go).

// CreateCommunication sends a POST request to the Oystehr FHIR API to create a Communication resource.
// It returns the ID of the created Communication or an error.
func CreateCommunication(httpClient *http.Client, cfg *config.Config, token string, patientID string, providerID string, totalScore int, q10Score int) (string, error) {
	// Use a default client if none is provided
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	// Construct the FHIR Communication payload
	comm := fhirCommunication{
		ResourceType: "Communication",
		Status:       "completed",
		Category: []fhirCategory{{
			Coding: []fhirCoding{{
				System:  "http://terminology.hl7.org/CodeSystem/communication-category",
				Code:    "alert",
				Display: "Alert",
			}},
		}},
		Subject:   fhirReference{Reference: fmt.Sprintf("Patient/%s", patientID)},
		Recipient: []fhirReference{{Reference: providerID}}, // Use providerID from config
		Payload: []fhirPayload{{
			ContentString: fmt.Sprintf("Alert: High EPDS score (%d) recorded for Patient %s. Q10 Score: %d. Please review patient chart.", totalScore, patientID, q10Score),
		}},
		Sent: time.Now().Format(time.RFC3339), // ISO8601 Format
	}

	commBytes, err := json.Marshal(comm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal FHIR Communication JSON: %w", err)
	}

	// Construct the request URL
	url := cfg.OystehrFHIRBaseURL + "/Communication"

	// Create the HTTP request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(commBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create FHIR Communication request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-zapehr-project-id", cfg.OystehrProjectID)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")

	// Execute the request
	log.Printf("Sending POST request to %s to create Communication for Patient %s", url, patientID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute FHIR Communication request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("Warning: failed to read response body after status %d for Communication creation: %v", resp.StatusCode, readErr)
	}

	// Check response status code
	if resp.StatusCode != http.StatusCreated { // 201 Created
		errBody := string(bodyBytes)
		if errBody == "" && readErr != nil {
			errBody = fmt.Sprintf("(could not read body: %v)", readErr)
		}
		log.Printf("ERROR: FHIR Communication creation failed. Status: %d, Body: %s", resp.StatusCode, errBody)
		return "", fmt.Errorf("FHIR API error creating Communication (status %d): %s", resp.StatusCode, errBody)
	}

	// Parse the response body to get the created resource ID
	var createdComm createdResource // Reusing the struct from observation.go
	if err := json.Unmarshal(bodyBytes, &createdComm); err != nil {
		log.Printf("ERROR: Failed to unmarshal FHIR Communication response body: %s. Error: %v", string(bodyBytes), err)
		return "", fmt.Errorf("failed to parse FHIR Communication response body: %w", err)
	}

	if createdComm.ID == "" {
		log.Printf("ERROR: FHIR Communication created (201) but response did not contain an ID. Body: %s", string(bodyBytes))
		return "", fmt.Errorf("FHIR Communication created but response missing ID")
	}

	log.Printf("Successfully created FHIR Communication with ID: %s for Patient %s", createdComm.ID, patientID)
	return createdComm.ID, nil
}
