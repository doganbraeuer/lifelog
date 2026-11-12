#!/bin/bash
# Fills a Lifelog database with the synthetic demo data in one step:
#
#   1. checks that the backend is running (GET /api/v1/status),
#   2. inserts the seed entries (go run ./cmd/seed, with embeddings),
#   3. uploads the same 11 files as in the calibration baseline
#      (docs/calibration/2026-11-11-baseline.md).
#
# Afterwards: wait about a minute, then measure with
#   cd backend && go run ./cmd/calibrate
#
# Usage (from anywhere; the backend must be running):
#   demo/upload.sh              seed + uploads
#   demo/upload.sh --no-seed    uploads only
#   LIFELOG_URL=http://192.168.0.10:8080 demo/upload.sh
#
# Running it again is harmless: seed replaces its own entries, and files
# that already exist answer 409 (duplicate). On 503 (model overloaded) the
# script waits and tries again.

set -uo pipefail

cd "$(dirname "$0")" || exit 1
BASE="${LIFELOG_URL:-http://127.0.0.1:8080}/api/v1"
RETRIES=3        # attempts per file on 503
RETRY_WAIT=30    # seconds between attempts on 503
PAUSE=3          # seconds between uploads (free tier rate limit)

seed=1
for arg in "$@"; do
  case "$arg" in
    --no-seed) seed=0 ;;
    -h|--help) sed -n '2,19p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $arg (see --help)" >&2; exit 2 ;;
  esac
done

if ! status=$(curl -s --max-time 5 "$BASE/status"); then
  echo "Backend not reachable at $BASE. Start it first: cd backend && go run ./cmd/server" >&2
  exit 1
fi
if ! echo "$status" | grep -Eq '"database": *"ok"'; then
  echo "Backend is running but not ready: $status" >&2
  exit 1
fi

if [ "$seed" = 1 ]; then
  echo "== Seed entries"
  if ! (cd ../backend && go run ./cmd/seed); then
    echo "Seed failed, stopping before the uploads." >&2
    exit 1
  fi
fi

created=0 existing=0 failed=0
body=$(mktemp)
trap 'rm -f "$body"' EXIT

# upload <curl form argument> <label>
upload() {
  local attempt code type
  printf "%-48s " "$2"
  for attempt in $(seq 1 "$RETRIES"); do
    code=$(curl -s -o "$body" -w "%{http_code}" --max-time 120 -F "$1" "$BASE/entries")
    [ "$code" = 503 ] && [ "$attempt" -lt "$RETRIES" ] || break
    printf "503, retry in %ss … " "$RETRY_WAIT"
    sleep "$RETRY_WAIT"
  done
  case "$code" in
    201)
      type=$(grep -Eo '"type": *"[^"]*"' "$body" | head -n 1 | cut -d '"' -f 4)
      echo "201 created ($type)"
      created=$((created + 1)) ;;
    409)
      echo "409 already there"
      existing=$((existing + 1)) ;;
    *)
      echo "$code failed: $(grep -Eo '"message": *"[^"]*"' "$body" | cut -d '"' -f 4)"
      failed=$((failed + 1)) ;;
  esac
  sleep "$PAUSE"
}

echo "== Uploads to $BASE/entries"
upload "file=@pdf/01_lab_report_2026-01-20.pdf"                         "01 lab report January (PDF)"
upload "file=@photos/02_doctor_letter_orthopedics_2026-03-05_photo.png" "02 doctor's letter orthopedics (photo)"
upload "file=@pdf/03_referral_physiotherapy_2026-03-05.pdf"             "03 referral physiotherapy (PDF)"
upload "file=@pdf/05_vaccination_tbe_2026-06-10.pdf"                    "05 TBE vaccination (PDF)"
upload "file=@pdf/06_lab_report_followup_2026-07-22.pdf"                "06 lab report July (PDF)"
upload "text=<texts/07_note_physiotherapy_2026-08-12.txt"               "07 note physiotherapy (text)"
upload "text=<texts/08_note_blood_pressure_2026-09-23.txt"              "08 note blood pressure (text)"
upload "file=@live/L1_lab_report_2026-10-28.pdf"                        "L1 lab report October (PDF)"
upload "file=@live/L2_doctor_letter_followup_2026-04-30.pdf"            "L2 doctor's letter follow-up (PDF)"
upload "file=@live/04_prescription_2026-05-18_app.png"                  "04 prescription (photo from the app)"
upload "file=@live/L3_eye_test_2026-09-15_app.png"                      "L3 eye test (photo from the app)"

echo "== $created created, $existing already there, $failed failed"
if [ "$failed" -gt 0 ]; then
  echo "Run the script again later; existing files are skipped with 409." >&2
  exit 1
fi
echo "Wait about a minute until Atlas has indexed the new chunks, then:"
echo "  cd backend && go run ./cmd/calibrate"
