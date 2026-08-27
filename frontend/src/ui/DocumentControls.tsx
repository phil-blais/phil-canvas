import { useState } from 'react'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import { api } from '../api/client'
import { fetchSceneFiles } from '../api/storage'
import { currentScenePayload } from '../collab/scenePayload'
import { mergeRestoredElements } from '../collab/restoreVersion'
import type { ActiveRoom, Session } from '../session/useSession'
import { useAsyncAction } from './useAsyncAction'
import { ContextMenu } from './ContextMenu'
import { VersionHistoryPanel } from './VersionHistoryPanel'

/**
 * In-room panel content. The document title is always editable in place (no
 * more "name this scene" prompt gated to the first save — see docs/EDD.md).
 * Admins get Save / Publish / History / Close; guests get Leave.
 */
export function DocumentControls({
  room,
  admin,
  markSceneId,
  closeRoom,
  leaveRoom,
  renameDocument,
  excalidrawApi,
  username,
}: {
  room: ActiveRoom
  admin: Session['admin']
  markSceneId: Session['markSceneId']
  closeRoom: Session['closeRoom']
  leaveRoom: Session['leaveRoom']
  renameDocument: Session['renameDocument']
  excalidrawApi: ExcalidrawImperativeAPI | null
  /** Display name for the current room membership (admin's Firebase name/email, or the random guest name). */
  username: string
}) {
  const { busy, error, status, run } = useAsyncAction()
  const [title, setTitle] = useState(room.name)
  const [historyOpen, setHistoryOpen] = useState(false)

  const isAdmin = room.role === 'admin'

  const commitTitle = () => {
    const trimmed = title.trim()
    if (!trimmed) {
      setTitle(room.name) // reject a blank rename; revert to the last-known title
      return
    }
    if (trimmed === room.name) return
    void run(() => renameDocument(trimmed))
  }

  const save = () =>
    run(async () => {
      if (!excalidrawApi) throw new Error('canvas not ready')
      const res = await api.saveScene(admin!.token, room.id, currentScenePayload(excalidrawApi))
      markSceneId(res.sceneId)
    }, 'Saved')

  const publish = () =>
    run(async () => {
      if (!excalidrawApi) throw new Error('canvas not ready')
      await api.publishScene(admin!.token, room.id, currentScenePayload(excalidrawApi))
    }, 'Published')

  // Restore just loads the chosen version into the live canvas as a normal
  // local edit — it propagates to collaborators the same way any other edit
  // does. History is never rewritten: the user's next Save appends it as the
  // new latest version.
  const restoreLiveVersion = async (versionId: string) => {
    if (!excalidrawApi || !room.sceneId) throw new Error('canvas not ready')
    const version = await api.getVersion(admin!.token, room.sceneId, versionId)
    const files = await fetchSceneFiles(version.fileIds ?? [])
    excalidrawApi.addFiles(Object.values(files))
    const current = excalidrawApi.getSceneElementsIncludingDeleted() as OrderedExcalidrawElement[]
    const merged = mergeRestoredElements(current, version.elements as OrderedExcalidrawElement[])
    excalidrawApi.updateScene({ elements: merged })
    setHistoryOpen(false)
  }

  return (
    <>
      <div className="flyout-section">
        {isAdmin ? (
          <input
            className="title-input"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={commitTitle}
            onKeyDown={(e) => {
              if (e.key === 'Enter') (e.target as HTMLInputElement).blur()
            }}
            aria-label="Document title"
          />
        ) : (
          <h3>{room.name}</h3>
        )}
        {isAdmin && room.code && (
          <div className="stack">
            <p className="muted">Share this code for guests to join:</p>
            <div className="share-code">{room.code}</div>
          </div>
        )}
      </div>

      {isAdmin && (
        <div className="flyout-section">
          <div className="btn-row">
            <button className="btn" disabled={busy || !excalidrawApi} onClick={() => void save()}>
              Save
            </button>
            <button className="btn secondary" disabled={busy || !excalidrawApi} onClick={() => void publish()}>
              Publish
            </button>
            {room.sceneId && (
              <ContextMenu label="More document actions">
                {(close) => (
                  <button
                    className="context-menu-item"
                    disabled={busy}
                    onClick={() => {
                      setHistoryOpen(true)
                      close()
                    }}
                  >
                    Version history
                  </button>
                )}
              </ContextMenu>
            )}
          </div>
        </div>
      )}

      {isAdmin && historyOpen && room.sceneId && (
        <VersionHistoryPanel
          documentName={room.name}
          token={admin!.token}
          sceneId={room.sceneId}
          onRestore={restoreLiveVersion}
          onClose={() => setHistoryOpen(false)}
        />
      )}

      <div className="flyout-section btn-row">
        {isAdmin ? (
          <button className="btn danger" onClick={() => void closeRoom()}>
            Close room
          </button>
        ) : (
          <button className="btn secondary" onClick={leaveRoom}>
            Leave room
          </button>
        )}
      </div>

      <div className="flyout-section">
        <p className="muted">
          {isAdmin ? 'Host' : 'Guest'} — {username}
        </p>
      </div>

      {status && <p className="muted">{status}.</p>}
      {error && <p className="error">{error}</p>}
    </>
  )
}
