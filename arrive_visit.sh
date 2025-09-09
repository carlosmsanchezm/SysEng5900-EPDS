#!/bin/bash

# Script to arrive a visit by creating/updating FHIR Encounter
# Based on Oystehr FHIR API documentation

set -e  # Exit on any error

# Configuration
TOKEN="${TOKEN:-}"
PROJECT_ID="596a23c5-e239-412b-bb05-55e47f41e1f8"
FHIR_BASE="https://fhir-api.zapehr.com/r4"

# From your created patient/appointment
APPT_ID="${APPT_ID:-bdec0198-05c5-4ff4-9b46-451b16c2fed7}"
PATIENT_ID="${PATIENT_ID:-9ba19f41-6f9c-4f6a-9201-5c489da98fe2}"

# Check required parameters
if [ -z "$TOKEN" ]; then
    echo "ERROR: TOKEN environment variable required"
    echo "Usage: TOKEN='your-token' ./arrive_visit.sh"
    exit 1
fi

echo "=== Arriving Visit ==="
echo "Appointment ID: $APPT_ID"
echo "Patient ID: $PATIENT_ID"
echo "Project ID: $PROJECT_ID"
echo ""

# Common headers
H_AUTH="Authorization: Bearer $TOKEN"
H_PROJECT="x-zapehr-project-id: $PROJECT_ID"
H_ACCEPT="Accept: application/fhir+json"
H_CONTENT="Content-Type: application/fhir+json"

echo "Step 1: Verify Appointment exists..."
APPT_RESPONSE=$(curl -sS \
  -H "$H_AUTH" \
  -H "$H_PROJECT" \
  -H "$H_ACCEPT" \
  "$FHIR_BASE/Appointment/$APPT_ID" || echo "ERROR")

if [[ "$APPT_RESPONSE" == *"ERROR"* ]] || [[ "$APPT_RESPONSE" == *"Not Found"* ]]; then
    echo "ERROR: Could not find Appointment $APPT_ID"
    echo "Response: $APPT_RESPONSE"
    exit 1
fi

echo "✓ Appointment found"
echo ""

echo "Step 2: Check for existing Encounter..."
ENC_SEARCH=$(curl -sS \
  -H "$H_AUTH" \
  -H "$H_PROJECT" \
  -H "$H_ACCEPT" \
  "$FHIR_BASE/Encounter?appointment=Appointment/$APPT_ID&_sort=-date&_count=1")

# Check if we found any encounters
ENC_COUNT=$(echo "$ENC_SEARCH" | jq -r '.total // 0')
echo "Found $ENC_COUNT existing encounters for this appointment"

if [ "$ENC_COUNT" -gt 0 ]; then
    # Update existing encounter
    ENC_ID=$(echo "$ENC_SEARCH" | jq -r '.entry[0].resource.id')
    CURRENT_STATUS=$(echo "$ENC_SEARCH" | jq -r '.entry[0].resource.status')
    
    echo "Found existing Encounter: $ENC_ID (status: $CURRENT_STATUS)"
    
    if [ "$CURRENT_STATUS" = "arrived" ] || [ "$CURRENT_STATUS" = "in-progress" ]; then
        echo "✓ Encounter already in active status: $CURRENT_STATUS"
        echo "Encounter ID: $ENC_ID"
    else
        echo "Updating encounter status to 'arrived'..."
        PATCH_RESPONSE=$(curl -sS -X PATCH \
          -H "$H_AUTH" \
          -H "$H_PROJECT" \
          -H "Content-Type: application/json-patch+json" \
          "$FHIR_BASE/Encounter/$ENC_ID" \
          --data '[{"op":"replace","path":"/status","value":"arrived"}]')
        
        NEW_STATUS=$(echo "$PATCH_RESPONSE" | jq -r '.status')
        echo "✓ Updated encounter status to: $NEW_STATUS"
        echo "Encounter ID: $ENC_ID"
    fi
else
    # Create new encounter
    echo "No existing encounter found. Creating new one..."
    
    NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    CREATE_RESPONSE=$(curl -sS -X POST \
      -H "$H_AUTH" \
      -H "$H_PROJECT" \
      -H "$H_CONTENT" \
      "$FHIR_BASE/Encounter" \
      --data @- <<EOF
{
  "resourceType": "Encounter",
  "status": "arrived",
  "class": { 
    "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode", 
    "code": "AMB", 
    "display": "ambulatory" 
  },
  "subject": { "reference": "Patient/$PATIENT_ID" },
  "appointment": [ { "reference": "Appointment/$APPT_ID" } ],
  "period": { "start": "$NOW" }
}
EOF
)
    
    ENC_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id')
    NEW_STATUS=$(echo "$CREATE_RESPONSE" | jq -r '.status')
    
    if [ "$ENC_ID" = "null" ] || [ -z "$ENC_ID" ]; then
        echo "ERROR: Failed to create encounter"
        echo "Response: $CREATE_RESPONSE"
        exit 1
    fi
    
    echo "✓ Created new encounter: $ENC_ID (status: $NEW_STATUS)"
fi

echo ""
echo "=== Visit Arrived Successfully ==="
echo "Encounter ID: $ENC_ID"
echo "Patient ID: $PATIENT_ID"
echo ""
echo "You can now test EPDS submission with:"
echo "curl -X POST http://localhost:8080/api/v1/submit-epds \\"
echo "  -d \"patientId=$PATIENT_ID\" \\"
echo "  -d \"q1=3&q2=2&q3=1&q4=2&q5=1&q6=3&q7=1&q8=0&q9=0&q10=1\""
echo ""
echo "Or with explicit encounter ID:"
echo "curl -X POST http://localhost:8080/api/v1/submit-epds \\"
echo "  -d \"patientId=$PATIENT_ID\" \\"
echo "  -d \"encounterId=$ENC_ID\" \\"
echo "  -d \"q1=3&q2=2&q3=1&q4=2&q5=1&q6=3&q7=1&q8=0&q9=0&q10=1\""