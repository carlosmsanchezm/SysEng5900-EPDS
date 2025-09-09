package main

import (
	"encoding/json" // Import for JSON error responses
	"fmt"
	"log"
	"net/http"
	"strconv" // Import for string conversion
	"strings" // Import for string manipulation (optional, could be useful)

	"example.com/epds-service/internal/auth"   // Import the auth package
	"example.com/epds-service/internal/config" // Import the config package
	"example.com/epds-service/internal/fhir"   // Import the fhir package
)

// ApiHandler holds dependencies for the API handlers.
type ApiHandler struct {
	Config        *config.Config
	Authenticator *auth.Authenticator
	// TODO: Consider adding a shared HTTP client here if needed for multiple FHIR calls
}

// ErrorResponse defines the structure for JSON error responses.
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Helper function to send JSON errors
func sendJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Status: "error", Message: message})
}

func main() {
	// Load application configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Oystehr authenticator
	authenticator := auth.NewAuthenticator(cfg, nil) // Using default HTTP client for now

	// Create the API handler with dependencies
	apiHandler := &ApiHandler{
		Config:        cfg,
		Authenticator: authenticator,
	}

	// Setup HTTP routes
	http.HandleFunc("/api/v1/submit-epds", apiHandler.handleSubmitEPDS)

	// Use port from loaded config
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting EPDS service on %s", addr)

	// Start the HTTP server
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleSubmitEPDS parses, validates, scores, authenticates, creates Observation,
// and creates Flag/Communication for high-risk results.
func (h *ApiHandler) handleSubmitEPDS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request for %s from %s", r.URL.Path, r.RemoteAddr)

	// Basic validation: Ensure it's a POST request
	if r.Method != http.MethodPost {
		// Note: http.Error sets Content-Type to text/plain, override if JSON is strictly needed
		// sendJSONError(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Rejected non-POST request for %s", r.URL.Path)
		return
	}

	// --- 1. Parse request body (assuming application/x-www-form-urlencoded) ---
	if err := r.ParseForm(); err != nil {
		log.Printf("ERROR: Failed to parse form data: %v", err)
		sendJSONError(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// --- 2. Extract and Validate Input ---
	patientID := strings.TrimSpace(r.FormValue("patientId"))
	idSystem  := strings.TrimSpace(r.FormValue("patientIdentifierSystem"))
	idValue   := strings.TrimSpace(r.FormValue("patientIdentifierValue"))
	encID     := strings.TrimSpace(r.FormValue("encounterId"))
	apptID    := strings.TrimSpace(r.FormValue("appointmentId"))

	epdsScores := make([]int, 10)
	for i := 1; i <= 10; i++ {
		qKey := fmt.Sprintf("q%d", i)
		qValueStr := r.FormValue(qKey)
		if qValueStr == "" {
			log.Printf("ERROR: Validation failed - %s is missing", qKey)
			sendJSONError(w, fmt.Sprintf("Invalid input: %s is required", qKey), http.StatusBadRequest)
			return
		}

		qValueInt, err := strconv.Atoi(qValueStr)
		if err != nil {
			log.Printf("ERROR: Validation failed - %s is not a valid integer ('%s'): %v", qKey, qValueStr, err)
			sendJSONError(w, fmt.Sprintf("Invalid input: %s must be an integer", qKey), http.StatusBadRequest)
			return
		}

		if qValueInt < 0 || qValueInt > 3 {
			log.Printf("ERROR: Validation failed - %s score (%d) out of range [0, 3]", qKey, qValueInt)
			sendJSONError(w, fmt.Sprintf("Invalid input: %s score must be between 0 and 3", qKey), http.StatusBadRequest)
			return
		}
		epdsScores[i-1] = qValueInt // Store score (adjusting for 0-based index)
	}

	log.Printf("Successfully parsed and validated input for Patient ID: %s, Scores: %v", patientID, epdsScores)

	// --- 3. Calculate EPDS Score ---
	totalScore := 0
	for _, score := range epdsScores {
		totalScore += score
	}
	q10Score := epdsScores[9]
	log.Printf("Calculated EPDS score (patient?: %s / %s|%s): Total=%d, Q10=%d", patientID, idSystem, idValue, totalScore, q10Score)

	// --- 4. Resolve Patient (if needed) & Authenticate with Oystehr ---
	// Defer resolution until after we have a token (same headers)
	token, err := h.Authenticator.GetAuthToken()
	if err != nil {
		log.Printf("ERROR: Failed to get Oystehr token: %v", err)
		sendJSONError(w, "Internal server error - authentication failed", http.StatusInternalServerError)
		return
	}
	log.Printf("Successfully obtained Oystehr token.")
	// Resolve patient via identifier if patientId was not provided
	fhirClient := &http.Client{} // shared per request
	if patientID == "" {
		if idSystem == "" || idValue == "" {
			sendJSONError(w, "provide patientId OR patientIdentifierSystem+patientIdentifierValue", http.StatusBadRequest)
			return
		}
		resolvedID, err := fhir.FindPatientIDByIdentifier(fhirClient, h.Config, token, idSystem, idValue)
		if err != nil {
			log.Printf("ERROR: patient lookup failed for %s|%s: %v", idSystem, idValue, err)
			sendJSONError(w, "patient not found from identifier", http.StatusBadRequest)
			return
		}
		patientID = resolvedID
	}

	// --- 5. Create FHIR Observation ---
	observationId, err := fhir.CreateObservation(fhirClient, h.Config, token, patientID, totalScore)
	if err != nil {
		log.Printf("ERROR: Failed to create FHIR Observation: %v", err)
		sendJSONError(w, "Failed to create FHIR Observation", http.StatusInternalServerError)
		return
	}
	log.Printf("Successfully created Observation ID: %s", observationId)

	// --- 6. Create FHIR Flag & Communication if High Risk ---
	isHighRisk := totalScore >= 13 || q10Score >= 1
	if isHighRisk {
		log.Printf("High risk detected for Patient %s (Score: %d, Q10: %d). Attempting to create Flag and Communication.", patientID, totalScore, q10Score)
		
		// Cascading encounter discovery
		if encID == "" {
			// Try appointment-based discovery first (if appointmentId provided)
			if apptID != "" {
				if found, err := fhir.FindEncounterByAppointment(fhirClient, h.Config, token, apptID); err == nil {
					encID = found
					log.Printf("Found encounter %s via appointment %s", encID, apptID)
				} else {
					log.Printf("WARN: appointment→encounter lookup failed for %s: %v", apptID, err)
				}
			}
			// Fall back to patient-based discovery
			if encID == "" {
				if found, err := fhir.FindActiveEncounterID(fhirClient, h.Config, token, patientID); err == nil {
					encID = found
					log.Printf("Found encounter %s via patient search", encID)
				} else {
					log.Printf("WARN: no active Encounter found for patient %s; creating patient-scoped Flag only (banner may not show). err=%v", patientID, err)
				}
			}
		}
		
		// Create Flag (with Encounter link if we have it, patient-scoped if not)
		flagId, flagErr := fhir.CreateFlag(fhirClient, h.Config, token, patientID, encID, totalScore, q10Score)
		if flagErr != nil {
			// Log error but continue to attempt Communication creation
			log.Printf("ERROR: Failed to create FHIR Flag: %v", flagErr)
		} else {
			log.Printf("Successfully created Flag ID: %s", flagId)
		}

		// Create Communication
		commId, commErr := fhir.CreateCommunication(fhirClient, h.Config, token, patientID, h.Config.AlertProviderFHIRID, totalScore, q10Score)
		if commErr != nil {
			// Log error, but response to client is already determined by Observation success
			log.Printf("ERROR: Failed to create FHIR Communication: %v", commErr)
		} else {
			log.Printf("Successfully created Communication ID: %s", commId)
		}
	}

	// --- 7. Return Success Response ---
	// The primary outcome (Observation creation) was successful.
	// Errors in Flag/Communication creation are logged but don't cause a client-facing error.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "success", "observationId": "%s", "calculatedScore": %d}`, observationId, totalScore)
	log.Printf("Successfully processed EPDS submission for Patient %s. Observation ID: %s", patientID, observationId)
}
