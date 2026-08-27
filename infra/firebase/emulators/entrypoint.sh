#!/bin/sh
# Start the Firebase emulators. Persists data to a mounted volume across
# restarts: imports on start if a prior export exists, and exports on exit.
set -e

DATA=/workspace/.emulator-data
IMPORT=""
if [ -f "$DATA/firebase-export-metadata.json" ]; then
  IMPORT="--import=$DATA"
fi

exec firebase emulators:start \
  --project "${FIREBASE_PROJECT:-demo-canvas}" \
  --only auth,firestore,storage \
  $IMPORT \
  --export-on-exit="$DATA"
