# Phil Canvas

An infinite canvas app built on Excalidraw, demonstrating real-time
collaboration with a Go backend. See [`docs/index.md`](docs/index.md) for the
full documentation set (PRD, SRS, EDD, and the deploy/dev runbooks).

## Layout

```
backend/    Go service - auth, JWT, in-memory rooms, WebSocket relay, scene persistence
frontend/   Vite + React + Excalidraw client
docs/       index.md, PRD.md, SRS.md, EDD.md, deploy.md, dev.md
firebase.json / .firebaserc   Hosting rewrites, Storage/Firestore rules, emulators
infra/firebase/   storage.rules, firestore.rules - security rules (all scene access is server-mediated)
infra/firebase/emulators/   Firebase emulator suite Docker image (local dev)
```

See [`docs/index.md`](docs/index.md#repo-layout) for the full package/directory
breakdown.

## Quick start

```sh
make emu         # Firebase Auth + Firestore + Storage in Docker (UI at :4000)
make backend     # Go backend on :8080, wired to the emulators
make frontend    # Vite dev server on :5173
make test        # backend `go test ./...` + frontend `npm test`, no emulators needed
```

Open <http://localhost:5173>. Full walkthrough (including the two-browser
collaboration test): [`docs/dev.md`](docs/dev.md). Deploying to GCP/Firebase:
[`docs/deploy.md`](docs/deploy.md).
