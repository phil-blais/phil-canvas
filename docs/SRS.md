# Software Requirements Specification

Precise, testable requirements that satisfy the [PRD](PRD.md). This is the
contract between frontend and backend, and between the backend and its
storage. Requirement strength follows RFC 2119: **MUST**/**MUST NOT** are
non-negotiable, **SHOULD** allows a justified exception, **MAY** is
optional. Each requirement keeps a stable ID (`FR-x.x`, `NFR-x`) — other
docs and code comments link to these IDs directly, so don't renumber on
edit; append instead. For the concrete wire contract (HTTP paths, methods,
payload shapes) behind requirements that name a specific operation, see
[openapi.yaml](openapi.yaml), cross-referenced below by `operationId`. For
*why* these choices were made and *how* they're implemented, including
sequence diagrams for the multi-step flows below, see the [EDD](EDD.md).

## 1. Roles & Authentication

**FR-1.1 — Roles.** The system MUST recognize three roles: unauthenticated
visitor, guest, and admin. Role MUST be carried in a backend-issued JWT
(`sub`, `role`, `room` claims); visitors MUST hold no token.

**FR-1.2 — Admin authentication** (`openapi.yaml#adminLogin`).
- The server MUST verify the submitted Firebase ID token via the Admin SDK
  and respond `401` if verification fails.
- A verified token MUST NOT by itself be treated as authorization — the
  server MUST additionally check the verified email/UID against
  `ADMIN_ALLOWLIST` and respond `403` if it isn't present.
- Admin JWTs MUST NOT carry a `room` claim.
- Admin JWTs MUST expire after `JWT_TTL` (default `12h`).

See EDD §Auth for the sequence diagram.

**FR-1.3 — Guest authentication** (`openapi.yaml#guestLogin`).
- The server MUST issue a JWT with `role: guest` and `room: <roomId>`
  only when the submitted code matches the target room's current code.
- The server MUST respond `401` and issue no token on any mismatch
  (unknown room or wrong code).
- The server MUST rate-limit guest code attempts per IP, to bound
  brute-forcing the ~27⁴ ≈ 530k code space.

**FR-1.4 — Authorization.** For any protected route or WebSocket
connection:
- A `guest` token MUST be accepted only if its `room` claim equals the
  room ID of the resource being accessed.
- An `admin` token MUST be accepted for any room or document — the
  server MUST NOT check room/document ownership for admins. (`AdminUID`
  is provenance only.)
- A request with neither a valid token nor a satisfied role check MUST be
  rejected before any handler logic runs.

**FR-1.5 — Visitor access.** Requests with no token MUST be permitted for
the live-room listing (FR-2.5) and for public Storage reads (the published
scene and its image files) only.

## 2. Documents & Rooms

- **FR-2.1** A **document** MUST be a persisted, named, versioned scene. A
  **room** MUST be the ephemeral live session for a document. At most one
  room MUST be open per document at any time.
- **FR-2.2** Opening a document's room (`openapi.yaml#openScene`) MUST be
  idempotent: it MUST return the document's existing live room if one is
  open, and MUST otherwise create one atomically, seeded from the given
  version or the latest saved version. Two concurrent calls for the same
  document MUST NOT produce two rooms.
- **FR-2.3** Creating a room with no bound document
  (`openapi.yaml#createRoom`) MUST create a new, not-yet-saved document
  (default title `"Untitled"`) with no bound scene until its first save.
- **FR-2.4** Room seeding MUST happen client-side: the server MUST NOT
  store or inject canvas state into a newly created room. The opening
  admin's client MUST apply the fetched version JSON after connecting.
  (Accepted consequence: a peerless later joiner starts blank if the
  seeding admin has already disconnected — see EDD §Accepted Trade-offs.)
- **FR-2.5** The live-room listing (`openapi.yaml#listRooms`) MUST be
  available to visitors and guests and MUST list only currently-live
  rooms (`{id, name, participantCount}`), and MUST NOT expose codes. The
  document catalog listing (`openapi.yaml#listScenes`) MUST be admin-only
  and MUST return the full document catalog regardless of live status.
- **FR-2.6** Any admin MUST be able to join, save, publish, rename, or
  close any room — not only its creator.
- **FR-2.7** Closing a room (`openapi.yaml#closeRoom`) MUST notify every
  connected client via two redundant signals: a `{type: "room-closed"}`
  text frame AND a WebSocket close with code `4001`. (See EDD §Relay for
  why both.)
- **FR-2.8** Renaming MUST be supported for a not-yet-saved draft
  (`openapi.yaml#renameRoom`) and, separately, for a saved document
  (`openapi.yaml#renameScene`). Renaming a saved document with a live
  room MUST update that room's live title immediately.
- **FR-2.9** Deleting a document (`openapi.yaml#deleteScene`) MUST
  delete the document and its full version history, and MUST respond
  `409` instead if the document currently has a live room — deletion
  MUST NOT force-close a session.

## 3. Collaboration Protocol

- **FR-3.1** Each room's collaboration session MUST be reachable via a
  dedicated, room-scoped secure WebSocket connection
  (`openapi.yaml#relayConnect`). The backend MUST act as a pure relay:
  it MUST NOT parse Yjs message contents or retain document state.
- **FR-3.2** A connecting client MUST send exactly one auth frame,
  `{"type": "auth", "token": "<jwt>"}`, before any other traffic. The
  server MUST authorize it per FR-1.4 and MUST close with code `4003` on
  failure.
- **FR-3.3** Every frame after the auth frame MUST be broadcast opaquely
  to every other client in the room (never echoed to the sender) —
  Yjs sync, Yjs awareness, and image-file binaries alike.
- **FR-3.4** Initial document sync MUST be peer-driven (standard Yjs
  sync-step-1/step-2), not server-provided. A joiner with no peers MUST
  start from an empty document unless client-side seeding (FR-2.4)
  applies.
- **FR-3.5** On disconnect, the server MUST deregister the client and
  MUST NOT close or otherwise affect the room.

See EDD §Relay for the handshake and close-signal sequence diagrams.

## 4. Persistence

**FR-4.1 — Firestore schema.** The server MUST store scenes as:

```
scenes/{sceneId}
  name: string
  createdAt: timestamp
  createdBy: uid

  versions/{versionId}      # auto-ID, ordered by createdAt
    createdAt: timestamp
    savedBy: uid
    elements: <JSON string>  # Excalidraw elements array
    appState: <JSON string>  # viewport/zoom, save/publish-time only
    fileIds: string[]        # referenced image files; blobs live in Storage
```

- **FR-4.2** Image binaries MUST NOT be stored inline in Firestore. They
  MUST be uploaded to Storage at `public/files/{fileId}`, content-addressed
  by Excalidraw's own `fileId`, so re-uploading an existing `fileId` MUST
  be a no-op (idempotent).
- **FR-4.3** Saving (`openapi.yaml#saveScene`) MUST append a new version
  to the room's bound document and MUST NOT require a name (a document
  title is always set, default `"Untitled"`, independent of saving). It
  MUST return the new `{sceneId, versionId}`.
- **FR-4.4** Publishing (`openapi.yaml#publishScene`) MUST write
  `{elements, appState}` to `public/published-scene.json` and MUST
  ensure every referenced image is present at `public/files/{fileId}`.
  Publishing MUST NOT require or create a version.
- **FR-4.5** Listing a document's versions
  (`openapi.yaml#listVersions`) MUST return version summaries; fetching
  a single version (`openapi.yaml#getVersion`) MUST return that
  version's full `elements`/`appState`/`fileIds`.
- **FR-4.6** Restoring a version MUST NOT rewrite history. If the document
  is live, the restored content MUST be applied as an ordinary local edit
  (propagating to collaborators, becoming the new latest version on next
  save). If not live, opening MUST seed a fresh room from that version
  instead of the latest.
- **FR-4.7** All Firestore/Storage access MUST be server-mediated through
  the Admin SDK. Firestore rules MUST deny all direct client access.
  Storage rules MUST allow public read only under `public/**` and MUST
  deny all client writes.

See EDD §Persistence for the save/publish sequence diagram.

See [openapi.yaml](openapi.yaml) for the concrete route table (methods,
paths, auth, payload shapes) implementing the requirements above, and the
[EDD](EDD.md) for how each is implemented.

## 5. Non-Functional Requirements

- **NFR-1 — Ephemerality.** Room state MUST live only in backend process
  memory. Loss on crash/restart/redeploy is accepted — a room is a
  session, not a record. Documents (Firestore) MUST remain durable
  regardless of room state.
- **NFR-2 — WebSocket longevity.** The backend MUST tolerate long-lived
  WebSocket connections. Cloud Run timeout MUST be configured above the
  default (currently `3600s`, vs. the `60s` default), and
  `min_instance_count` MUST be `≥1` so scale-to-zero never drops
  in-memory rooms mid-session.
- **NFR-3 — Transport.** The Cloud Run service MUST run HTTP/1.1, not
  end-to-end HTTP/2 — WebSocket upgrades require it.
- **NFR-4 — Rate limiting.** Guest-code attempts MUST be rate-limited per
  IP (FR-1.3).
- **NFR-5 — Config.** Runtime config MUST be environment-driven:
  `JWT_SECRET`, `FIREBASE_PROJECT_ID`, and `ADMIN_ALLOWLIST` MUST be
  required and MUST fail startup (not first request) if missing. `PORT`,
  `JWT_TTL`, `STORAGE_BUCKET`, `DISABLE_PERSISTENCE`, and the
  `*_EMULATOR_HOST` vars MUST have safe defaults or be optional.
- **NFR-6 — Local dev parity.** The system MUST be runnable fully offline
  against Firebase emulators, and MAY run with `DISABLE_PERSISTENCE=true`
  against an in-memory store with no Firebase dependency at all. See
  [dev.md](dev.md).
- **NFR-7 — Test coverage.** Every subsystem with non-trivial logic (auth
  middleware, rate limiting, room registry, relay, scene stores, token
  issuance, frontend Yjs sync/awareness/restore logic) MUST have unit
  tests runnable without emulators; `make test` MUST run both suites in
  CI.
