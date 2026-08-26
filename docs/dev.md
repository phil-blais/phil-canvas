# Local Development

Run the whole stack locally — including admin sign-in, scene persistence, and
publishing — with the Firebase emulators in Docker. **No JRE or firebase-tools
on your host**; they live in the emulator container.

## Prerequisites

- Docker (with Compose)
- Go 1.25+
- Node 22+
- [air](https://github.com/air-verse/air) — `go install github.com/air-verse/air@latest`
  (hot-reloads the backend; config in `backend/.air.toml`)

## Start everything

Three terminals (or use the `Makefile` targets):

```sh
make emu         # 1) Firebase Auth + Firestore + Storage in Docker (UI: http://localhost:4000)
make backend     # 2) Go backend on :8080, wired to the emulators
make frontend    # 3) Vite dev server on :5173
```

Then open <http://localhost:5173>.

- The first `make emu` builds the image (installs a JRE + firebase-tools
  inside the container); later starts are fast. If you change the
  `Dockerfile`/`entrypoint.sh`, run `docker compose build` before `make emu` to
  pick up the change.
- Emulator data persists in a Docker volume across `down`/`up`. Rooms are
  in-memory and reset when the backend restarts (by design).
- `demo-canvas` is a Firebase "demo" project, so the emulators run fully
  offline — no real Firebase project or credentials.

## Wiring (how it fits together)

| Piece | Talks to | Via |
|-------|----------|-----|
| Backend Firestore/Storage/Auth | Emulators | `*_EMULATOR_HOST` env (set by `make backend`) |
| Frontend → backend | `:8080` | Vite proxy (`/ws`, `/auth`, `/rooms`, `/scenes`) |
| Frontend admin sign-in | Auth emulator | `VITE_FIREBASE_AUTH_EMULATOR_URL` (`frontend/.env.development`) |
| Frontend published scene | Storage emulator | `VITE_PUBLISHED_BASE_URL` |

Config lives in `frontend/.env.development` (committed, non-secret) and the
`backend` target in the `Makefile`.

## Full end-to-end test (two browsers)

1. In tab A, click **Sign in as admin**. The Auth emulator popup lets you create
   a test account — sign in as **`admin@example.com`** (this matches
   `ADMIN_ALLOWLIST` in the `Makefile`; change both to use another email).
2. **Create room** → a 4-char code appears.
3. In tab B (or another browser), open the room and enter the code → the two
   canvases sync live (draw, cursors, laser).
4. As admin, **Save** (writes a Firestore version — visible in the emulator UI)
   and **Publish** (writes to the Storage emulator).
5. Reload the root `/` unauthenticated → the published scene renders read-only.
6. **Close room** as admin → the guest is returned to the site.

## Tests

Unit tests need no emulators:

```sh
make test    # backend `go test ./...` + frontend `npm test`
```

CI runs the same suites plus `terraform fmt`/`validate` (`make tf-check`) — see
[`../.github/workflows/backend.yml`](../.github/workflows/backend.yml),
[`frontend.yml`](../.github/workflows/frontend.yml), and
[`infra.yml`](../.github/workflows/infra.yml).

## Without Firebase at all

For pure frontend work you can skip the emulators and run the backend in-memory:

```sh
cd backend && DISABLE_PERSISTENCE=true JWT_SECRET=x FIREBASE_PROJECT_ID=demo \
  ADMIN_ALLOWLIST=you@example.com go run .
```

Admin sign-in won't work (no Auth), but the guest/relay paths do if a room
exists.

## Troubleshooting

- **Emulator ports busy**: something else is on 9099/8081/9199/4000. Stop it or
  change the ports in `docker-compose.yml` + `firebase.json` + the env.
- **Backend can't reach Firestore/Storage**: ensure `make emu` is up; the
  `*_EMULATOR_HOST` values point at `localhost:<port>`.
- **Admin sign-in 403**: the signed-in email isn't in `ADMIN_ALLOWLIST`.
- **Published scene never loads**: nothing has been published yet — expected;
  you'll see the "Canvas" placeholder.
