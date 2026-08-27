# Local development. Dependencies (Firebase emulators) run in Docker; the Go
# backend and Vite frontend run on the host. See docs/dev.md.

.PHONY: help emu emu-down emu-logs backend frontend preview test tf-check

help:
	@echo "make emu             # start Firebase emulators (Auth/Firestore/Storage) in Docker"
	@echo "make emu-down        # stop the emulators (data preserved in a volume)"
	@echo "make backend         # run the Go backend against the emulators"
	@echo "make frontend        # run the Vite dev server (uses frontend/.env.development)"
	@echo "make preview         # build the production frontend bundle and serve it (needs emu + backend)"
	@echo "make test            # backend + frontend unit tests (no emulators needed)"
	@echo "make tf-check        # terraform fmt -check + validate (Docker, no local install needed)"

emu:
	docker compose up -d

emu-down:
	docker compose down

emu-logs:
	docker compose logs -f firebase-emulators

# Backend wired to the emulators. Persistence stays on (usingEmulators() =>
# no credentials needed); rooms are in-memory. Sign in as an ADMIN_ALLOWLIST
# email in the Auth emulator to act as admin.
backend:
	cd backend && \
	FIREBASE_PROJECT_ID=demo-canvas \
	FIREBASE_AUTH_EMULATOR_HOST=localhost:9099 \
	FIRESTORE_EMULATOR_HOST=localhost:8081 \
	STORAGE_EMULATOR_HOST=localhost:9199 \
	STORAGE_BUCKET=demo-canvas.appspot.com \
	JWT_SECRET=local-dev-secret \
	ADMIN_ALLOWLIST=admin@example.com \
	air

frontend:
	cd frontend && npm install && npm run dev

# Production bundle pointed at the local emulators/backend (--mode development
# loads frontend/.env.development instead of the default production mode,
# which has no committed .env.production). Requires `make emu` + `make backend`.
preview:
	cd frontend && npm install && npm run build -- --mode development && npm run preview

test:
	cd backend && go test ./...
	cd frontend && npm ci && npm test

tf-check:
	scripts/terraform-check.sh
