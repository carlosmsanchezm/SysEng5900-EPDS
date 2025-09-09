# --- Oystehr API Endpoints ---
# (These usually don't change, but confirm from your console if needed)
export OYSTEHR_FHIR_BASE_URL="https://fhir-api.zapehr.com/r4"
export OYSTEHR_AUTH_URL="https://auth.zapehr.com/oauth/token"

# --- Your Project and M2M Client Details ---
export OYSTEHR_PROJECT_ID="596a23c5-e239-412b-bb05-55e47f41e1f8"
export OYSTEHR_M2M_CLIENT_ID="QfHq018YKWQDhWlbL9mW9jsT7JakDP5n"

# --- IMPORTANT: Paste your NEW Secret Here ---
# Replace YOUR_NEW_M2M_CLIENT_SECRET below with the actual secret you copied
export OYSTEHR_M2M_CLIENT_SECRET="....."

# --- Target Provider for Alerts ---
# Using "Example Doctor" ID you provided
export ALERT_PROVIDER_FHIR_ID="Practitioner/f5d7cbdf-f829-4e0e-846b-63ce53ca8c6c"

# --- Optional Port ---
# (Uncomment the line below only if you need a port other than 8080)
# export PORT="8080"
