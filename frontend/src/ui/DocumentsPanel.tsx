import { useCallback, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { api, type RoomSummary } from '../api/client'
import type { Session } from '../session/useSession'
import { useAsyncAction, type Run } from './useAsyncAction'
import { useLatestOnly } from '../hooks/useLatestOnly'
import { DocumentCatalogModal } from './DocumentCatalogModal'

/**
 * Not-in-room panel content. "Live now" (everyone, guests included) lists
 * currently-open sessions to join — this is the only view of documents a
 * guest ever gets (decision #3: guests never browse the full saved-documents
 * catalog). Admins additionally get "Documents": Create (a new document
 * seeded from the currently published scene, if any) and Open (the
 * file-manager-style DocumentCatalogModal, which owns the full saved-scenes
 * catalog — cross-referenced against this panel's live-rooms poll, passed
 * straight through, to show which are currently open).
 */
export function DocumentsPanel({ session }: { session: Session }) {
  const [rooms, setRooms] = useState<RoomSummary[] | null>(null)
  const [catalogOpen, setCatalogOpen] = useState(false)
  const { busy, error, run, reportError } = useAsyncAction()

  const { next, isCurrent } = useLatestOnly()

  const refresh = useCallback(async () => {
    const requestId = next()
    try {
      const result = await api.listRooms()
      if (!isCurrent(requestId)) return // superseded by a newer poll
      setRooms(result)
    } catch {
      if (!isCurrent(requestId)) return
      reportError('Could not load live documents')
      // Stop showing the skeleton if the very first load failed; leave any
      // already-loaded rooms in place if a later poll fails instead.
      setRooms((current) => current ?? [])
    }
  }, [reportError, next, isCurrent])

  useEffect(() => {
    void refresh()
    const timer = setInterval(refresh, 5000)
    return () => clearInterval(timer)
  }, [refresh])

  return (
    <>
      <div className="flyout-section">
        <h3>Live now</h3>
        {rooms === null ? (
          <ListSkeleton
            cell={
              <div>
                <div className="skeleton-bar skeleton-bar-title" />
                <div className="skeleton-bar skeleton-bar-meta" />
              </div>
            }
          />
        ) : rooms.length === 0 ? (
          <p className="muted">Nothing is live right now.</p>
        ) : (
          rooms.map((room) => (
            <LiveRoomRow
              key={room.id}
              room={room}
              admin={session.admin}
              enterAsAdmin={session.enterAsAdmin}
              joinAsGuest={session.joinAsGuest}
              run={run}
              busy={busy}
            />
          ))
        )}
      </div>

      {session.admin && (
        <div className="flyout-section">
          <h3>Documents</h3>
          <div className="btn-row">
            <button className="btn" disabled={busy} onClick={() => run(() => session.createDocument())}>
              Create
            </button>
            <button className="btn secondary" onClick={() => setCatalogOpen(true)}>
              Open
            </button>
          </div>
        </div>
      )}

      {catalogOpen && session.admin && (
        <DocumentCatalogModal
          admin={session.admin}
          liveRooms={rooms}
          openScene={session.openScene}
          enterAsAdmin={session.enterAsAdmin}
          onClose={() => setCatalogOpen(false)}
        />
      )}

      <div className="flyout-section">
        <IdentityControls
          firebaseEnabled={session.firebaseEnabled}
          admin={session.admin}
          authError={session.authError}
          signIn={session.signIn}
          signOut={session.signOut}
          busy={busy}
          run={run}
        />
      </div>

      {error && <p className="error">{error}</p>}
    </>
  )
}

/** Who you are: signed out, signed in as admin, or sign-in unavailable. */
function IdentityControls({
  firebaseEnabled,
  admin,
  authError,
  signIn,
  signOut,
  busy,
  run,
}: {
  firebaseEnabled: Session['firebaseEnabled']
  admin: Session['admin']
  authError: Session['authError']
  signIn: Session['signIn']
  signOut: Session['signOut']
  busy: boolean
  run: Run
}) {
  if (!firebaseEnabled) {
    return <p className="muted">Admin sign-in is unavailable (Firebase not configured).</p>
  }
  if (admin) {
    return (
      <div className="list-row">
        <span className="muted">Signed in as {admin.user.displayName ?? admin.user.email}</span>
        <button className="btn small secondary" disabled={busy} onClick={() => run(() => signOut())}>
          Sign out
        </button>
      </div>
    )
  }
  return (
    <div className="stack">
      <button className="btn" disabled={busy} onClick={() => run(() => signIn())}>
        Sign in as admin
      </button>
      {authError && <p className="error">{authError}</p>}
    </div>
  )
}

/** Placeholder rows shown while a list request is in flight. */
function ListSkeleton({ rows = 2, cell }: { rows?: number; cell: ReactNode }) {
  return (
    <>
      {Array.from({ length: rows }, (_, i) => (
        <div className="list-row" key={i} aria-hidden="true">
          {cell}
          <div className="skeleton-bar skeleton-bar-btn" />
        </div>
      ))}
    </>
  )
}

function LiveRoomRow({
  room,
  admin,
  enterAsAdmin,
  joinAsGuest,
  run,
  busy,
}: {
  room: RoomSummary
  admin: Session['admin']
  enterAsAdmin: Session['enterAsAdmin']
  joinAsGuest: Session['joinAsGuest']
  run: Run
  busy: boolean
}) {
  const [codeOpen, setCodeOpen] = useState(false)
  const [code, setCode] = useState('')

  return (
    <div className="list-row">
      <div>
        <div className="list-row-title">{room.name}</div>
        <div className="list-row-meta">
          {room.participantCount} {room.participantCount === 1 ? 'person' : 'people'}
        </div>
      </div>
      <div className="btn-row">
        {admin && (
          <button className="btn small secondary" disabled={busy} onClick={() => enterAsAdmin(room.id, room.name)}>
            Enter
          </button>
        )}
        <button className="btn small" disabled={busy} onClick={() => setCodeOpen((o) => !o)}>
          Join
        </button>
      </div>

      {codeOpen && (
        <div className="stack full-width">
          <input
            className="code-input"
            value={code}
            maxLength={4}
            placeholder="CODE"
            autoFocus
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void run(() => joinAsGuest(room.id, room.name, code))
            }}
          />
          <button
            className="btn small"
            disabled={busy || code.length !== 4}
            onClick={() => run(() => joinAsGuest(room.id, room.name, code))}
          >
            Enter code
          </button>
        </div>
      )}
    </div>
  )
}
