#!/usr/bin/env bash
#
# Generate the spoken "enter the code shown on your screen" prompt for the DTMF
# channel, in the 8 kHz mono format Asterisk plays. By default the DTMF call just
# beeps; install this prompt and the caller is voice-guided instead.
#
# Requires espeak-ng (or espeak) and sox. On Raspberry Pi OS / Debian:
#   sudo apt install espeak-ng sox
#
# Usage:
#   ./make-dtmf-prompt.sh [output_dir]            # default output_dir = .
#
# Then install it and point the receiver at it:
#   sudo cp ctv-enter-pin.wav /usr/share/asterisk/sounds/en/
#   export CTV_DTMF_PROMPT=ctv-enter-pin          # or set "dtmf_prompt" in config.json
#
set -euo pipefail

TEXT="Please enter the code shown on your screen, then press the pound key."
OUT_DIR="${1:-.}"
NAME="ctv-enter-pin"

SAY="$(command -v espeak-ng || command -v espeak || true)"
[ -n "$SAY" ] || { echo "need espeak-ng or espeak: sudo apt install espeak-ng" >&2; exit 1; }
command -v sox >/dev/null || { echo "need sox: sudo apt install sox" >&2; exit 1; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

"$SAY" -v en -s 150 -w "$TMP/raw.wav" "$TEXT"
# Asterisk's en sounds are 8 kHz, mono, 16-bit signed PCM.
sox "$TMP/raw.wav" -r 8000 -c 1 -b 16 -e signed-integer "$OUT_DIR/$NAME.wav"

echo "wrote $OUT_DIR/$NAME.wav"
echo "install: sudo cp $OUT_DIR/$NAME.wav /usr/share/asterisk/sounds/en/"
echo "enable:  export CTV_DTMF_PROMPT=$NAME   (or set \"dtmf_prompt\": \"$NAME\" in config.json)"
