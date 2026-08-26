# Docs Index

Excalidraw Collaborative Canvas: an experimental site built on an
infinite Excalidraw canvas, with a self-hosted Go backend providing
real-time multi-user collaboration. See [PRD.md](PRD.md) for what this is
and why.

## Start here

| Doc | Answers | Read it when |
|---|---|---|
| [PRD.md](PRD.md) | What are we building, for whom, and why? | Onboarding, or deciding whether a change belongs in scope. |
| [SRS.md](SRS.md) | What must the system precisely do? API contracts, data schemas, auth rules, non-functional requirements. | Implementing or reviewing a feature; checking whether behavior is a bug or a spec gap. |
| [EDD.md](EDD.md) | How is it actually built, and why this way? Architecture, subsystems, the load-bearing design decisions, accepted trade-offs. | Before touching the Yjs binding, auth, or the relay — several parts are subtle and documented on purpose. |
| [deploy.md](deploy.md) | How do I ship this to GCP/Firebase? | Standing up infra, or deploying a change. |
| [dev.md](dev.md) | How do I run this on my machine? | First-time setup, or debugging locally. |

Read PRD → SRS → EDD in that order the first time: each narrows from *why*
to *what* to *how*.

## Architecture, one paragraph

The frontend (Vite + React + Excalidraw) is served as a static SPA from
Firebase Hosting's CDN. A single Go service on Cloud Run handles auth (JWT
issuance for admins and guests), room lifecycle, a pure WebSocket relay
(Yjs traffic is forwarded opaquely, never parsed), and scene persistence.
Firestore stores version history; Firebase Storage holds the published
scene JSON and content-addressed image files. The frontend never touches
Firestore/Storage directly for scene data — every read/write is
server-mediated through the backend's Admin SDK credentials. Full diagram
and detail in [EDD.md](EDD.md#architecture-at-a-glance).

## Repo layout

```
backend/    Go service — auth, JWT, in-memory rooms, WebSocket relay, scene persistence
  internal/auth        JWT issuance/verification, allowlist, guest rate limiting
  internal/rooms        In-memory room registry (live sessions)
  internal/relay         WebSocket broadcast relay
  internal/scenes       Firestore/Storage-backed + in-memory scene stores
  internal/httpapi       REST handlers for rooms/scenes
  internal/config, firebaseapp, token, web   Supporting packages
frontend/   Vite + React + Excalidraw client
  src/session    useSession — admin/room state and every session action
  src/collab     Yjs binding, awareness, transport, version restore
  src/ui         Flyout panel and its contents
  src/api        REST client + public Storage fetch helpers
  src/auth       Firebase JS SDK wrapper
docs/       PRD.md, SRS.md, EDD.md, deploy.md, dev.md (this index's siblings)
infra/terraform/   All GCP infrastructure (see deploy.md)
infra/firebase/    Firestore/Storage security rules
infra/firebase/emulators/   Firebase emulator suite Docker image (see dev.md)
firebase.json / .firebaserc   Hosting rewrites, emulator config
```

## Current status

The system is built and deployed — see the "Documents Redesign" work
(the Rooms/Scenes → Documents model in [EDD.md](EDD.md#rooms)) as the most
recent structural change. `git log` is the authoritative history; these
docs describe current-state design, not a build plan.

## Conventions for these docs

- **PRD** owns *why/what for users*; no API shapes or code references.
- **SRS** owns *precise, testable requirements*; no rationale — "why" links
  out to the EDD.
- **EDD** owns *how and why built this way*; organized by subsystem
  (`backend/internal/*`, `frontend/src/*`), not by historical build phase.
- Runbooks (`deploy.md`, `dev.md`) stay operational — commands and
  troubleshooting, not design rationale.
- When behavior changes, update the SRS requirement it affects and, if the
  reasoning changed, the corresponding EDD section — in the same PR as the
  code change, not as a follow-up.
