#!/bin/bash

# Test script for EPDS service
# Usage: ./test_epds.sh

BASE_URL="http://localhost:8080"
ENDPOINT="/api/v1/submit-epds"

echo "EPDS Service Test Script"
echo "========================"
echo "Make sure the service is running on $BASE_URL"
echo ""

# Test A: With patientId only (Encounter discovery auto)
echo "Test A: With patientId only (auto encounter discovery)"
echo "Replace <PATIENT_UUID> with actual patient ID:"
echo "curl -X POST $BASE_URL$ENDPOINT \\"
echo "  -d \"patientId=<PATIENT_UUID>\" \\"
echo "  -d \"q1=3&q2=2&q3=1&q4=2&q5=1&q6=3&q7=1&q8=0&q9=0&q10=1\""
echo ""

# Test B: With identifier (no patientId)
echo "Test B: With patient identifier (no patientId)"
echo "Replace system and value with actual identifier:"
echo "curl -X POST $BASE_URL$ENDPOINT \\"
echo "  -d \"patientIdentifierSystem=http://hospital.example/mrn\" \\"
echo "  -d \"patientIdentifierValue=MRN-12345\" \\"
echo "  -d \"q1=3&q2=2&q3=1&q4=2&q5=1&q6=3&q7=1&q8=0&q9=0&q10=1\""
echo ""

# Test C: Explicit Encounter (bypass discovery)
echo "Test C: With explicit encounter ID"
echo "Replace UUIDs with actual values:"
echo "curl -X POST $BASE_URL$ENDPOINT \\"
echo "  -d \"patientId=<PATIENT_UUID>\" \\"
echo "  -d \"encounterId=<ENCOUNTER_UUID>\" \\"
echo "  -d \"q1=4&q2=3&q3=2&q4=3&q5=1&q6=3&q7=1&q8=0&q9=0&q10=0\""
echo ""

# Test D: Low-risk (no Flag)
echo "Test D: Low-risk score (no Flag created)"
echo "Replace <PATIENT_UUID> with actual patient ID:"
echo "curl -X POST $BASE_URL$ENDPOINT \\"
echo "  -d \"patientId=<PATIENT_UUID>\" \\"
echo "  -d \"q1=0&q2=0&q3=0&q4=1&q5=0&q6=1&q7=0&q8=0&q9=0&q10=0\""
echo ""

# Test E: Missing required fields
echo "Test E: Missing required fields (should return error)"
echo "curl -X POST $BASE_URL$ENDPOINT \\"
echo "  -d \"q1=1&q2=1&q3=1&q4=1&q5=1&q6=1&q7=1&q8=1&q9=1&q10=1\""
echo ""

echo "Expected behaviors:"
echo "- Test A: Should find active encounter automatically"
echo "- Test B: Should resolve patient ID from identifier"
echo "- Test C: Should use explicit encounter ID"  
echo "- Test D: Should create Observation but no Flag (low risk)"
echo "- Test E: Should return error about missing patient info"