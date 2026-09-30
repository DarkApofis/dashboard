#!/usr/bin/env sh
#
# Walks through the part of the system that is hardest to see from the UI: a provider
# notification, the redelivery that follows when the provider gets no answer, and the
# proof that the redelivery changed nothing.
#
# Usage:  ./scripts/webhook-demo.sh          (or: make webhook-demo)
#
# Needs the stack up (docker compose up --build -d) and WEBHOOK_SECRET empty, which is
# the default. With a secret set the webhook answers 401 unless the body is signed;
# the README shows how to sign it.

set -eu

API_URL="${API_URL:-http://localhost:8080}"
WEB_URL="${WEB_URL:-http://localhost:3000}"
COMPOSE="${COMPOSE:-docker compose}"

STAMP="$(date +%s)"
REF="DEMO-${STAMP}"
REF_B="DEMO-${STAMP}-B"
EVENT="evt_demo_${STAMP}"
TX="tx_demo_${STAMP}"
AMOUNT="185000.75"

JSON="Content-Type: application/json"

if ! curl -sf "${API_URL}/health" >/dev/null 2>&1; then
  echo "The API does not answer at ${API_URL}." >&2
  echo "Start the stack first:  docker compose up --build -d" >&2
  exit 1
fi

# First business in the list; the seed creates three.
BUSINESS="$(curl -s "${API_URL}/businesses" | sed -n 's/.*"id":\([0-9]*\).*/\1/p' | head -1)"
BUSINESS="${BUSINESS:-1}"

status_of() {
  curl -s "${API_URL}/payment-references?external_reference=$1" |
    sed -n 's/.*"external_reference":"'"$1"'","amount":[^,]*,"description":"[^"]*","status":"\([a-z]*\)".*/\1/p'
}

echo "==> 1. Creating the reference ${REF} for business ${BUSINESS}"
curl -s -X POST "${API_URL}/payment-references" -H "${JSON}" \
  -d "{\"business_id\":${BUSINESS},\"external_reference\":\"${REF}\",\"amount\":${AMOUNT},\"description\":\"Demo del webhook\",\"expires_at\":\"2030-01-01T23:59:59Z\"}"
printf '\n\n'

echo "==> 2. The provider notifies the payment (event_id ${EVENT})"
echo "       Expected: outcome=applied, applied=true."
curl -s -w '\n    HTTP %{http_code}\n\n' -X POST "${API_URL}/webhooks/payments" -H "${JSON}" \
  -d "{\"event_id\":\"${EVENT}\",\"external_reference\":\"${REF}\",\"amount\":${AMOUNT},\"transaction_id\":\"${TX}\",\"paid_at\":\"2026-09-28T20:15:00Z\"}"

echo "==> 3. The provider got no answer and redelivers THE SAME event_id"
echo "       Expected: duplicate=true, applied=false. Nothing is applied twice."
curl -s -w '\n    HTTP %{http_code}\n\n' -X POST "${API_URL}/webhooks/payments" -H "${JSON}" \
  -d "{\"event_id\":\"${EVENT}\",\"external_reference\":\"${REF}\",\"amount\":${AMOUNT},\"transaction_id\":\"${TX}\",\"paid_at\":\"2026-09-28T20:15:00Z\"}"

echo "==> 4. A new event_id carrying a DIFFERENT transaction for the same reference"
echo "       Expected: conflict_other_transaction. The first payment is not overwritten."
curl -s -w '\n    HTTP %{http_code}\n\n' -X POST "${API_URL}/webhooks/payments" -H "${JSON}" \
  -d "{\"event_id\":\"${EVENT}_otra\",\"external_reference\":\"${REF}\",\"amount\":${AMOUNT},\"transaction_id\":\"tx_otro\",\"paid_at\":\"2026-09-28T20:15:00Z\"}"

echo "==> 5. A wrong amount on a pending reference"
curl -s -o /dev/null -X POST "${API_URL}/payment-references" -H "${JSON}" \
  -d "{\"business_id\":${BUSINESS},\"external_reference\":\"${REF_B}\",\"amount\":${AMOUNT},\"description\":\"Monto que no coincide\",\"expires_at\":\"2030-01-01T23:59:59Z\"}"
echo "       Expected: amount_mismatch, and the reference stays pending."
curl -s -w '\n    HTTP %{http_code}\n' -X POST "${API_URL}/webhooks/payments" -H "${JSON}" \
  -d "{\"event_id\":\"${EVENT}_mismatch\",\"external_reference\":\"${REF_B}\",\"amount\":1.00,\"transaction_id\":\"tx_mal\",\"paid_at\":\"2026-09-28T20:15:00Z\"}"
echo "    status de ${REF_B}: $(status_of "${REF_B}")"
printf '\n'

echo "==> 6. The paid reference: one transaction, flagged for review"
curl -s "${API_URL}/payment-references?external_reference=${REF}"
printf '\n\n'

echo "==> 7. What was recorded. The event_id of step 2 must appear exactly once."
${COMPOSE} exec -T db psql -U payment_links -d payment_links -tAc \
  "SELECT 'event_id=${EVENT} -> ' || count(*) || ' fila(s)' FROM webhook_events WHERE event_id='${EVENT}';" || true
${COMPOSE} exec -T db psql -U payment_links -d payment_links -c \
  "SELECT event_id, outcome, payment_reference_id FROM webhook_events WHERE external_reference LIKE 'DEMO-${STAMP}%' ORDER BY id;" || true

echo "Done. Open ${WEB_URL}/payment-references?external_reference=${REF}"
