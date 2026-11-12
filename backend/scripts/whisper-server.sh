#!/bin/bash
# Starts whisper.cpp's server for the transcription (docs/API.md, 3.9) on
# macOS or Linux. The backend talks to it at WHISPER_URL
# (default http://127.0.0.1:8178); the phone never does.
#
#   backend/scripts/whisper-server.sh download   # fetch and verify the models (once)
#   backend/scripts/whisper-server.sh            # start the server (keep it running)
#
# After the start it sends warmup.wav once, and it starts whisper-server
# again if it stops (crash), waiting 2 s, then up to 60 s if it keeps
# stopping. Ctrl-C stops both.
#
# Settings (all optional):
#   WHISPER_MODEL_DIR  where the models are  (default ~/.local/share/lifelog/whisper)
#   WHISPER_PORT       port                  (default 8178; 8080 is the backend)
#
# Install whisper.cpp first: macOS `brew install whisper-cpp`, see
# backend/README.md, section speech.

set -euo pipefail

MODEL_DIR="${WHISPER_MODEL_DIR:-$HOME/.local/share/lifelog/whisper}"
PORT="${WHISPER_PORT:-8178}"

# File, URL and SHA-256 as published by Hugging Face (checked 2026-11-12).
MODEL=ggml-large-v3-turbo.bin
MODEL_URL=https://huggingface.co/ggerganov/whisper.cpp/resolve/main/$MODEL
MODEL_SHA=1fc70f774d38eb169993ac391eea357ef47c88757ef72ee5943879b7e8e2bc69
VAD=ggml-silero-v5.1.2.bin
VAD_URL=https://huggingface.co/ggml-org/whisper-vad/resolve/main/$VAD
VAD_SHA=29940d98d42b91fbd05ce489f3ecf7c72f0a42f027e4875919a28fb4c04ea2cf

sha256() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# fetch <file> <url> <sha256>: download unless present, then verify.
fetch() {
  local file="$MODEL_DIR/$1"
  if [ ! -f "$file" ]; then
    echo "Downloading $1 …"
    curl -fL --retry 3 -o "$file.part" "$2"
    mv "$file.part" "$file"
  fi
  local got
  got=$(sha256 "$file")
  if [ "$got" != "$3" ]; then
    echo "SHA-256 of $1 does not match: $got (expected $3). Delete the file and download again." >&2
    exit 1
  fi
  echo "$1: SHA-256 ok"
}

if [ "${1:-}" = "download" ]; then
  mkdir -p "$MODEL_DIR"
  fetch "$VAD" "$VAD_URL" "$VAD_SHA"
  fetch "$MODEL" "$MODEL_URL" "$MODEL_SHA"
  exit 0
fi
if [ $# -gt 0 ]; then
  sed -n '2,19p' "$0" | sed 's/^# \{0,1\}//'
  exit 2
fi

if ! command -v whisper-server >/dev/null; then
  echo "whisper-server not found. Install whisper.cpp (backend/README.md, section speech)." >&2
  exit 1
fi
for f in "$MODEL" "$VAD"; do
  if [ ! -f "$MODEL_DIR/$f" ]; then
    echo "$MODEL_DIR/$f is missing. Run: $0 download" >&2
    exit 1
  fi
done

# German only, no temperature fallback, non-speech tokens suppressed,
# voice activity detection: only parts with speech are decoded. The backend
# sends a list of medical terms as prompt (internal/speech/whisper,
# Vocabulary); --carry-initial-prompt keeps it for every 30-second window.
# "--max-context 0" would switch the prompt off (measured 2026-11-12,
# docs/speech/README.md). Only reachable from this machine.
SERVER_ARGS=(
  --model "$MODEL_DIR/$MODEL"
  --host 127.0.0.1 --port "$PORT"
  --language de
  --carry-initial-prompt
  --no-fallback
  --suppress-nst
  --vad --vad-model "$MODEL_DIR/$VAD"
)
URL="http://127.0.0.1:$PORT"
WARMUP="$(cd "$(dirname "$0")" && pwd)/warmup.wav"

log() { echo "whisper-server.sh: $*" >&2; }

# Ctrl-C or kill: stop whisper-server too, then end.
child=""
stop() {
  trap - INT TERM
  if [ -n "$child" ]; then
    kill "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
  fi
  exit 0
}
trap stop INT TERM

# wait_ready: until whisper-server answers (it loads the model first);
# fails if it stops or takes longer than 120 s.
wait_ready() {
  for _ in $(seq 1 120); do
    kill -0 "$child" 2>/dev/null || return 1
    curl -fs -o /dev/null --max-time 2 "$URL/" && return 0
    sleep 1
  done
  return 1
}

# warm_up: one short recording with speech. whisper.cpp 1.9.5 crashes if
# the first request after the start has no speech (issue #4156, fixed after
# 1.9.5); the backend avoids it too (no_language_probabilities), this also
# prepares the GPU so the first real recording is not slower.
warm_up() {
  curl -fs -o /dev/null --max-time 60 -F "file=@$WARMUP" -F language=de -F response_format=json "$URL/inference"
}

first=1
delay=2
while true; do
  started=$(date +%s)
  whisper-server "${SERVER_ARGS[@]}" &
  child=$!
  if wait_ready; then
    first=0
    if warm_up; then
      log "ready at $URL (warmed up)"
    else
      log "warm-up failed"
    fi
  elif [ "$first" = 1 ]; then
    log "whisper-server did not start (see its output above)"
    kill "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
    exit 1
  fi

  code=0
  wait "$child" || code=$?
  child=""
  ran=$(( $(date +%s) - started ))
  if [ "$ran" -ge 300 ]; then
    delay=2 # it ran for a while: start again quickly
  fi
  log "whisper-server stopped (exit code $code) after $ran s; starting it again in $delay s"
  sleep "$delay" &
  wait $! || true
  delay=$(( delay * 2 > 60 ? 60 : delay * 2 ))
done
