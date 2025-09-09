package fhir

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "example.com/epds-service/internal/config"
)

type bundle struct {
    Entry []struct {
        Resource json.RawMessage `json:"resource"`
    } `json:"entry"`
}
type fhirID struct{ ID string `json:"id"` }

// GET /Patient?identifier={system}|{value}
func FindPatientIDByIdentifier(httpClient *http.Client, cfg *config.Config, token, system, value string) (string, error) {
    if httpClient == nil { httpClient = &http.Client{Timeout: 10 * time.Second} }
    u := fmt.Sprintf("%s/Patient?identifier=%s|%s", cfg.OystehrFHIRBaseURL, system, value)
    req, _ := http.NewRequest(http.MethodGet, u, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("x-zapehr-project-id", cfg.OystehrProjectID)
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := httpClient.Do(req)
    if err != nil { return "", fmt.Errorf("patient search failed: %w", err) }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("patient search status %d", resp.StatusCode) }

    var b bundle
    if err := json.NewDecoder(resp.Body).Decode(&b); err != nil { return "", fmt.Errorf("patient bundle decode: %w", err) }
    if len(b.Entry) == 0 { return "", fmt.Errorf("patient not found for %s|%s", system, value) }

    var p fhirID
    if err := json.Unmarshal(b.Entry[0].Resource, &p); err != nil { return "", fmt.Errorf("patient id parse: %w", err) }
    if p.ID == "" { return "", fmt.Errorf("patient id missing") }
    return p.ID, nil
}

// GET /Encounter?subject=Patient/{id}&status=arrived,in-progress&_sort=-date&_count=1
func FindActiveEncounterID(httpClient *http.Client, cfg *config.Config, token, patientID string) (string, error) {
    if httpClient == nil { httpClient = &http.Client{Timeout: 10 * time.Second} }
    u := fmt.Sprintf("%s/Encounter?subject=Patient/%s&status=arrived,in-progress&_sort=-date&_count=1",
        cfg.OystehrFHIRBaseURL, patientID)
    req, _ := http.NewRequest(http.MethodGet, u, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("x-zapehr-project-id", cfg.OystehrProjectID)
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := httpClient.Do(req)
    if err != nil { return "", fmt.Errorf("encounter search failed: %w", err) }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("encounter search status %d", resp.StatusCode) }

    var b bundle
    if err := json.NewDecoder(resp.Body).Decode(&b); err != nil { return "", fmt.Errorf("encounter bundle decode: %w", err) }
    if len(b.Entry) == 0 { return "", fmt.Errorf("no active encounter found for patient %s", patientID) }

    var e fhirID
    if err := json.Unmarshal(b.Entry[0].Resource, &e); err != nil { return "", fmt.Errorf("encounter id parse: %w", err) }
    if e.ID == "" { return "", fmt.Errorf("encounter id missing") }
    return e.ID, nil
}