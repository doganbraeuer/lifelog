#!/bin/bash
# Makes the synthetic recordings NN_anna.m4a from the reference texts NN.txt
# with the macOS voice "Anna" (de_DE), in the format the app records: AAC in
# M4A, mono, 16 kHz, 32 kbit/s. Without metadata and bit-exact, so running
# it again gives the same files on the same macOS version.
#
#   demo/speech/make-voice.sh
#
# Needs macOS (say) and ffmpeg. Only for cmd/speechcheck; real recordings of
# a person's voice never go into this repository (demo/README.md).

set -euo pipefail
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

for txt in [0-9][0-9].txt; do
  n=${txt%.txt}
  say -v Anna -f "$txt" -o "$tmp/$n.aiff"
  ffmpeg -nostdin -y -loglevel error -i "$tmp/$n.aiff" \
    -ac 1 -ar 16000 -c:a aac -b:a 32k \
    -map_metadata -1 -fflags +bitexact -flags:a +bitexact \
    "${n}_anna.m4a"
  echo "${n}_anna.m4a"
done
