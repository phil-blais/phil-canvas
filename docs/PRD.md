# Product Requirements Document

## Vision

An experimental site built on an infinite Excalidraw canvas instead of a static
page. Visitors see a published, hand-drawn-style scene as the "website"
itself. Behind that, an admin can open a live collaboration room, invite
guests by sharing a short code, co-edit the canvas in real time, and publish
the result — the same live editing surface produces the public artifact.

The product goal and the engineering goal are the same thing: demonstrate a
real-time collaborative editor built on a self-hosted Go relay, not a
managed backend-as-a-service.

## Goals

- Serve a published Excalidraw scene at the root URL, rendered read-only, on
  a CDN — the canvas's static presentation layer.
- Let an admin create or reopen a **document**: a persisted, named,
  versioned canvas.
- Let multiple people co-edit a document live: shared shapes/text/images,
  cursor presence, and a laser pointer, with sub-second propagation.
- Let a guest join a live session with nothing but a 4-character code — no
  account required.
- Let the admin publish the current canvas as the new public site content at
  any time, independent of saving.
- Keep the whole stack cheap and idle-friendly (scale-to-zero-adjacent,
  Firebase free-tier-friendly) since this is an experimental project, not a
  production SaaS.

## Non-goals

- Multi-tenant document ownership / sharing permissions beyond "any admin
  can touch any document." There is one trusted admin pool (an allowlist),
  not per-document ACLs.
- Offline editing or conflict resolution beyond what Yjs gives for free.
- Mobile-optimized editing (Excalidraw's own responsiveness is relied on
  as-is).
- Guest accounts, guest identity persistence, or guest-created documents.
- High availability / multi-region. A single Cloud Run service with
  in-memory room state is an accepted single-point-of-failure for live
  sessions (see [EDD](EDD.md#accepted-trade-offs)).

## Users & Roles

| Role | Identity | Capabilities |
|---|---|---|
| **Visitor** | None | View the published scene at `/`, read-only. See the list of currently-live rooms. |
| **Guest** | A 4-character room code, nothing else | Join one specific live room and co-edit it for the session. No document management. |
| **Admin** | Google sign-in (allowlisted) | Everything a guest can do, plus: create/rename/delete documents, browse version history and restore, save, publish, close rooms. Any admin may act on any document — there's no per-document owner. |

The allowlist gate is a deliberate product decision, not an incidental
detail: the site's collaboration feature is an experimental demo, not an open
editor, so write access stays limited to the site owner (and anyone they
explicitly add).

## Core Features

1. **Published site** — the root URL renders the most recently published
   scene, read-only, with no sign-in required.
2. **Document catalog** (admin only) — browse every saved document, see
   which are currently live, open/rename/delete, and start a new blank
   document.
3. **Live collaboration room** — real-time shared canvas: shape/text/image
   edits, cursor positions, a laser pointer, and participant presence,
   for everyone currently in the room.
4. **Guest access** — join a specific live room via a shared 4-character
   code, no sign-up.
5. **Save & version history** — an admin can save the current canvas as a
   new version at any time; past versions are browsable and restorable
   without discarding history.
6. **Publish** — an admin can push the current canvas to the public root
   URL at any time, decoupled from saving a version.
7. **Rename** — a document's title is editable at any time, whether or not
   it has been saved yet.

## User Experience

All document and session actions live in a single flyout panel, bottom-right,
always visible:

- **Not in a room**: "Live now" (everyone) lists currently-open rooms,
  joinable by code. Admins additionally see "My documents": the full
  catalog with Open / Rename / Delete / Version History, and a "New
  document" action.
- **In a room**: an always-editable title, plus role-appropriate actions
  (guest: leave; admin: save, publish, version history, close room).

See [SRS.md](SRS.md) §2 "Documents & Rooms" for the precise state
transitions and [EDD.md](EDD.md) §Frontend, "Session state" for how the
panel is wired to the underlying session state.

## Success Criteria

This is an experimental piece, so "done" is measured against the demonstration
goal rather than a business metric:

- A stranger can open the site, watch (or join) a live multi-cursor editing
  session, and understand — without reading code — that the collaboration is
  real, not simulated.
- The full loop (create → invite → co-edit → save → publish) works
  end-to-end in production, not just in local dev.
- The codebase itself is presentable: the [EDD](EDD.md) and code should
  hold up to a reviewer reading both.
