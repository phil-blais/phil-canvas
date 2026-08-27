import { useCallback, useEffect, useState } from 'react'
import { api, type RoomSummary, type SceneSummary } from '../api/client'
import type { AdminSession, Session } from '../session/useSession'
import { useAsyncAction, type Run } from './useAsyncAction'
import { VersionHistory } from './VersionHistory'

function CloseIcon() {
  return (
    <svg viewBox="0 0 20 20" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true" focusable="false">
      <path d="M5 5l10 10M15 5 5 15" />
    </svg>
  )
}

/** A row in the modal's list pane: either a saved scene, or a live-but-unsaved draft room. */
interface CatalogEntry {
  id: string
  name: string
  createdAt?: string
  live?: { participantCount: number }
  /** Absent for a synthetic unsaved-draft entry — gates rename/delete/history. */
  sceneId?: string
}

/**
 * File-manager-style document browser: a list on the left, a detail pane
 * (info, actions, version history) for the selected document on the right.
 * Pulled out of the flyout entirely so document management has real room to
 * breathe instead of fighting a 300px-wide panel.
 */
export function DocumentCatalogModal({
  admin,
  liveRooms,
  openScene,
  enterAsAdmin,
  onClose,
}: {
  admin: AdminSession
  liveRooms: RoomSummary[] | null
  openScene: Session['openScene']
  enterAsAdmin: Session['enterAsAdmin']
  onClose: () => void
}) {
  const [scenes, setScenes] = useState<SceneSummary[] | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const { busy, error, run, reportError } = useAsyncAction()

  const loadScenes = useCallback(async () => {
    try {
      setScenes(await api.listScenes(admin.token))
    } catch {
      reportError('Could not load documents')
    }
  }, [admin.token, reportError])

  useEffect(() => {
    void loadScenes()
  }, [loadScenes])

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  const liveBySceneId = new Map<string, RoomSummary>()
  const unsavedLiveRooms: RoomSummary[] = []
  for (const room of liveRooms ?? []) {
    if (room.sceneId) liveBySceneId.set(room.sceneId, room)
    else unsavedLiveRooms.push(room)
  }

  const entries: CatalogEntry[] = [
    // Newest-first: the backend lists scenes oldest-first, so reverse for the
    // recency-first ordering a file manager's list is expected to have.
    ...[...(scenes ?? [])].reverse().map((scene) => ({
      id: scene.id,
      name: scene.name,
      createdAt: scene.createdAt,
      live: liveBySceneId.get(scene.id),
      sceneId: scene.id,
    })),
    ...unsavedLiveRooms.map((room) => ({
      id: room.id,
      name: room.name,
      live: { participantCount: room.participantCount },
    })),
  ]

  const selected = entries.find((e) => e.id === selectedId) ?? null

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="doc-modal" onClick={(e) => e.stopPropagation()}>
        <div className="doc-modal-header">
          <h3>My documents</h3>
          <button className="btn small secondary icon-btn" aria-label="Close" onClick={onClose}>
            <CloseIcon />
          </button>
        </div>

        <div className="doc-modal-body">
          <div className="doc-modal-list">
            {scenes === null ? (
              <p className="muted">Loading…</p>
            ) : entries.length === 0 ? (
              <p className="muted">No documents yet.</p>
            ) : (
              entries.map((entry) => (
                <button
                  key={entry.id}
                  className={`doc-modal-list-item${entry.id === selectedId ? ' selected' : ''}`}
                  onClick={() => setSelectedId(entry.id)}
                >
                  <span className="list-row-title">{entry.name}</span>
                  {entry.live && (
                    <span className="list-row-meta">
                      <span className="badge-live" /> Live · {entry.live.participantCount}
                    </span>
                  )}
                </button>
              ))
            )}
          </div>

          <div className="doc-modal-detail">
            {!selected ? (
              <p className="doc-modal-empty muted">Select a document to see its details.</p>
            ) : (
              <DocumentDetail
                key={selected.id}
                admin={admin}
                entry={selected}
                busy={busy}
                run={run}
                openScene={openScene}
                enterAsAdmin={enterAsAdmin}
                onDeleted={() => {
                  setSelectedId(null)
                  void loadScenes()
                }}
                onRenamed={() => void loadScenes()}
              />
            )}
            {error && <p className="error">{error}</p>}
          </div>
        </div>
      </div>
    </div>
  )
}

function DocumentDetail({
  admin,
  entry,
  busy,
  run,
  openScene,
  enterAsAdmin,
  onDeleted,
  onRenamed,
}: {
  admin: AdminSession
  entry: CatalogEntry
  busy: boolean
  run: Run
  openScene: Session['openScene']
  enterAsAdmin: Session['enterAsAdmin']
  onDeleted: () => void
  onRenamed: () => void
}) {
  const [renaming, setRenaming] = useState(false)
  const [title, setTitle] = useState(entry.name)
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const open = () =>
    entry.sceneId
      ? run(() => openScene({ id: entry.sceneId!, name: entry.name }))
      : run(async () => enterAsAdmin(entry.id, entry.name))

  const commitRename = () => {
    const trimmed = title.trim()
    if (trimmed && trimmed !== entry.name) {
      void run(async () => {
        await api.renameScene(admin.token, entry.sceneId!, trimmed)
        onRenamed()
      })
    } else {
      setTitle(entry.name) // reject a blank/unchanged rename; revert to the last-known title
    }
    setRenaming(false)
  }

  return (
    <>
      {renaming ? (
        <input
          className="title-input"
          value={title}
          autoFocus
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') commitRename()
            if (e.key === 'Escape') {
              setTitle(entry.name)
              setRenaming(false)
            }
          }}
          aria-label="Document title"
        />
      ) : (
        <h3>{entry.name}</h3>
      )}

      {entry.createdAt && <p className="muted">Created {new Date(entry.createdAt).toLocaleString()}</p>}
      {entry.live && (
        <p className="list-row-meta">
          <span className="badge-live" /> Live · {entry.live.participantCount}{' '}
          {entry.live.participantCount === 1 ? 'person' : 'people'} editing now
        </p>
      )}

      {renaming ? (
        <div className="btn-row">
          <button className="btn" disabled={busy || !title.trim()} onClick={commitRename}>
            Save
          </button>
          <button
            className="btn secondary"
            disabled={busy}
            onClick={() => {
              setTitle(entry.name)
              setRenaming(false)
            }}
          >
            Cancel
          </button>
        </div>
      ) : confirmingDelete ? (
        <div className="stack">
          <p className="muted">Delete this document permanently?</p>
          <div className="btn-row">
            <button
              className="btn danger"
              disabled={busy}
              onClick={() =>
                run(async () => {
                  await api.deleteScene(admin.token, entry.sceneId!)
                  onDeleted()
                })
              }
            >
              Confirm delete
            </button>
            <button className="btn secondary" disabled={busy} onClick={() => setConfirmingDelete(false)}>
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <div className="btn-row">
          <button className="btn" disabled={busy} onClick={open}>
            Open
          </button>
          {entry.sceneId && (
            <button className="btn secondary" disabled={busy} onClick={() => setRenaming(true)}>
              Rename
            </button>
          )}
          {entry.sceneId && (
            <button className="btn secondary" disabled={busy} onClick={() => setConfirmingDelete(true)}>
              Delete
            </button>
          )}
        </div>
      )}

      {entry.sceneId && (
        <div className="doc-modal-history">
          <h3>Version history</h3>
          <VersionHistory
            token={admin.token}
            sceneId={entry.sceneId}
            onRestore={(versionId) => openScene({ id: entry.sceneId!, name: entry.name }, versionId)}
          />
        </div>
      )}
    </>
  )
}
