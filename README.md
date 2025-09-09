# EPDS Service - Edinburgh Postnatal Depression Scale Integration

A microservice that integrates EPDS (Edinburgh Postnatal Depression Scale) screening results with the Oystehr FHIR platform. The service automatically creates FHIR resources (Observations, Flags, Communications) and implements intelligent encounter discovery to enable red banner alerts in the EHR interface.

## 🏗️ Architecture Overview

The service implements **service-driven discovery** to automatically link high-risk EPDS results to active patient visits:

1. **Score Collection**: Receives EPDS questionnaire responses (Q1-Q10)
2. **Risk Assessment**: Calculates total score and identifies high-risk patients (score ≥13 OR Q10 ≥1)
3. **FHIR Integration**: Creates standardized medical records:
   - **Observation**: EPDS score (LOINC 99046-5)
   - **Flag**: Safety alert linked to specific encounter (enables red banner)
   - **Communication**: Provider notification
4. **Encounter Discovery**: Automatically finds active encounters via:
   - Appointment ID → Encounter lookup (primary)
   - Patient ID → Active encounter search (fallback)
   - Manual encounter ID override (optional)

## 🚀 Quick Start

### Prerequisites

- Go 1.19+
- Access to Oystehr console: https://console.oystehr.com/
- `jq` (for JSON formatting in examples)
- `curl` (for API testing)

### 1. Get Oystehr Credentials

1. Login to [Oystehr Console](https://console.oystehr.com/)
2. Navigate to your project
3. Copy your **Project ID** (format: `596a23c5-e239-412b-bb05-55e47f41e1f8`)
4. Generate a fresh **Bearer Token** (expires in 24 hours)

### 2. Configure Environment

Create or update `env.sh` with your credentials:

```bash
#!/bin/bash
# Oystehr/Zapehr Configuration
export OYSTEHR_CLIENT_ID="your_client_id_here"
export OYSTEHR_CLIENT_SECRET="your_client_secret_here"
export OYSTEHR_FHIR_BASE_URL="https://fhir-api.zapehr.com/r4"
export OYSTEHR_PROJECT_ID="596a23c5-e239-412b-bb05-55e47f41e1f8"
export OYSTEHR_AUTH_URL="https://auth.zapehr.com/oauth/token"

# Service Configuration
export PORT="8080"
export ALERT_PROVIDER_FHIR_ID="your_provider_id_here"
```

**⚠️ Security Note**: Never commit `env.sh` to version control. Add it to `.gitignore`.

### 3. Build and Start Service

```bash
# Load environment variables
source ./env.sh

# Build the service
go build -o epds-service ./cmd/epds-service

# Start the service
./epds-service
```

You should see: `Starting EPDS service on :8080`

## 📋 Testing Guide

### Step 1: Create Test Patient & Visit

1. In Oystehr console, create a test patient
2. Create an appointment for that patient  
3. Note the **Patient ID** and **Appointment ID** from the URLs

### Step 2: Arrive the Visit

Before EPDS submission, the visit must be "arrived" to create an active encounter:

```bash
# Set your credentials
export TOKEN='your_fresh_bearer_token_here'
export PROJECT_ID='596a23c5-e239-412b-bb05-55e47f41e1f8'
export PATIENT_ID='patient-uuid-from-console'
export APPT_ID='appointment-uuid-from-console'

# Check appointment exists
curl -sS \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  -H "Accept: application/fhir+json" \
  "https://fhir-api.zapehr.com/r4/Appointment/$APPT_ID"

# Find existing encounter
curl -sS \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  -H "Accept: application/fhir+json" \
  "https://fhir-api.zapehr.com/r4/Encounter?appointment=Appointment/$APPT_ID&_count=1" \
  | jq '.entry[0].resource.id, .entry[0].resource.status'

# Update encounter status to "arrived"
ENC_ID='encounter-id-from-above'
curl -sS -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  -H "Content-Type: application/json-patch+json" \
  "https://fhir-api.zapehr.com/r4/Encounter/$ENC_ID" \
  --data '[{"op":"replace","path":"/status","value":"arrived"}]'

# Also update appointment status
curl -sS -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  -H "Content-Type: application/json-patch+json" \
  "https://fhir-api.zapehr.com/r4/Appointment/$APPT_ID" \
  --data '[{"op":"replace","path":"/status","value":"arrived"}]'
```

### Step 3: Submit EPDS Screening

#### Test Case A: High-Risk Score (Auto Discovery)
```bash
curl -sS -X POST http://localhost:8080/api/v1/submit-epds \
  -d "patientId=$PATIENT_ID" \
  -d "appointmentId=$APPT_ID" \
  -d "q1=3&q2=2&q3=1&q4=2&q5=1&q6=3&q7=1&q8=0&q9=0&q10=1" \
  | jq .
```
**Expected**: Score=14, Flag created with encounter link → red banner appears

#### Test Case B: Patient Identifier Lookup
```bash
curl -sS -X POST http://localhost:8080/api/v1/submit-epds \
  -d "patientIdentifierSystem=http://hospital.example/mrn" \
  -d "patientIdentifierValue=MRN-12345" \
  -d "q1=3&q2=2&q3=1&q4=2&q5=1&q6=3&q7=1&q8=0&q9=0&q10=1" \
  | jq .
```

#### Test Case C: Low-Risk Score 
```bash
curl -sS -X POST http://localhost:8080/api/v1/submit-epds \
  -d "patientId=$PATIENT_ID" \
  -d "q1=0&q2=0&q3=0&q4=1&q5=0&q6=1&q7=0&q8=0&q9=0&q10=0" \
  | jq .
```
**Expected**: Score=2, Observation created, no Flag (no red banner)

#### Test Case D: Explicit Encounter Override
```bash
curl -sS -X POST http://localhost:8080/api/v1/submit-epds \
  -d "patientId=$PATIENT_ID" \
  -d "encounterId=$ENC_ID" \
  -d "q1=4&q2=3&q3=2&q4=3&q5=1&q6=3&q7=1&q8=0&q9=0&q10=0" \
  | jq .
```

### Step 4: Verify Red Banner

1. Open visit page: `https://console.oystehr.com/visit/$APPT_ID`
2. Refresh the page
3. Look for red EPDS alert banner (only appears for high-risk submissions)

### Step 5: API Verification

```bash
# Check created Observation
curl -sS \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  -H "Accept: application/fhir+json" \
  "https://fhir-api.zapehr.com/r4/Observation?subject=Patient/$PATIENT_ID&code=99046-5&_sort=-date&_count=1" \
  | jq '.entry[0].resource.id, .entry[0].resource.valueInteger'

# Check created Flag (high-risk only)
curl -sS \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  -H "Accept: application/fhir+json" \
  "https://fhir-api.zapehr.com/r4/Flag?encounter=Encounter/$ENC_ID&_sort=-date&_count=1" \
  | jq '.entry[0].resource.id, .entry[0].resource.code.text'
```

## 📊 API Reference

### POST /api/v1/submit-epds

Submit EPDS questionnaire responses for scoring and FHIR integration.

#### Request Parameters (form-encoded)

**Patient Identification** (one required):
- `patientId`: Direct patient UUID
- `patientIdentifierSystem` + `patientIdentifierValue`: Patient identifier lookup

**EPDS Responses** (all required):
- `q1` through `q10`: Integer values 0-3 for each question

**Optional Parameters**:
- `appointmentId`: Appointment UUID for encounter discovery
- `encounterId`: Direct encounter UUID (bypasses discovery)

#### Response

```json
{
  "status": "success",
  "observationId": "uuid-of-created-observation",
  "calculatedScore": 14
}
```

#### Error Responses

```json
{
  "status": "error", 
  "message": "descriptive error message"
}
```

## 🏥 EPDS Scoring Rules

- **Total Score**: Sum of Q1-Q10 responses (0-30 range)
- **High Risk Criteria**: Total ≥13 OR Q10 ≥1 (self-harm indicator)
- **Low Risk**: All other scores

### High-Risk Actions
1. Creates FHIR Observation (always)
2. Creates FHIR Flag linked to encounter (triggers red banner)
3. Creates FHIR Communication to alert provider

### Low-Risk Actions
1. Creates FHIR Observation only
2. No Flag or Communication created

## 🔧 Development

### Project Structure
```
├── cmd/epds-service/           # Main application entry point
│   └── main.go
├── internal/
│   ├── auth/                   # Oystehr authentication
│   │   └── auth.go
│   ├── config/                 # Configuration management
│   │   └── config.go
│   └── fhir/                   # FHIR resource management
│       ├── communication.go    # Provider communications
│       ├── flag.go            # Safety alerts/flags
│       ├── observation.go     # EPDS score observations
│       └── search.go          # Patient/encounter discovery
├── env.sh                      # Environment configuration (DO NOT COMMIT)
├── test_epds.sh               # Test script with examples
└── README.md
```

### Building
```bash
go build -o epds-service ./cmd/epds-service
```

### Running Tests
```bash
go test ./...
```

### Logs
The service provides detailed logging for debugging:
- Request validation
- EPDS score calculations  
- Encounter discovery process
- FHIR resource creation
- Error diagnostics

## 🚨 Troubleshooting

### Common Issues

**"patient not found from identifier"**
- Verify the identifier system/value exists in Patient.identifier
- Check that Zambdas are populating Patient.identifier during intake

**"no active encounter found"**
- Ensure visit is "arrived" (both Appointment and Encounter status)
- Check encounter status includes: planned, arrived, or in-progress

**"401/403 to FHIR"**
- Generate fresh bearer token (expires in 24 hours)
- Verify x-zapehr-project-id header is included
- Check token permissions in Oystehr console

**"200 but no red banner"**
- Verify Flag includes `encounter.reference` field
- Confirm you're viewing the correct visit page
- Check browser cache/refresh page

**"Service won't start"**
- Verify `env.sh` is sourced: `source ./env.sh`
- Check all required environment variables are set
- Ensure port 8080 is available

### Debug Commands

```bash
# Check service health
curl -v http://localhost:8080/api/v1/submit-epds

# Validate FHIR connectivity  
curl -sS \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-zapehr-project-id: $PROJECT_ID" \
  "https://fhir-api.zapehr.com/r4/Patient?_count=1"

# View service logs
tail -f service.log
```

## 🔐 Security Considerations

1. **Never commit secrets**: Keep `env.sh` in `.gitignore`
2. **Rotate credentials**: Update tokens/secrets regularly
3. **Network security**: Run service behind proper firewall/proxy
4. **Input validation**: Service validates all EPDS inputs (0-3 range)
5. **FHIR compliance**: All resources follow HL7 FHIR R4 standard

## 📞 Support

For technical issues:
1. Check service logs for detailed error messages
2. Verify all prerequisites and credentials
3. Test with provided curl examples
4. Review Oystehr console for FHIR resource creation

---

*This service implements the complete EPDS-to-EHR integration pipeline with automatic encounter discovery, enabling seamless clinical workflow integration.*