import { useEffect, useState } from 'react'
import { api, type VersionSummary } from '../api/client'
import { useAsyncAction } from './useAsyncAction'

/** Compact "Saved" column value — the docked history panel is only 300px, so
 * a full toLocaleString() (with seconds) crowds out the By/Restore columns. */
function formatSavedAt(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
}

/**
 * Lists a scene's saved versions with a Restore action. Rendered inside
 * VersionHistoryPanel, which owns the surrounding panel chrome (header,
 * document name, close button) — this component is just the list. Used in
 * two contexts that share this list but differ in what "restore" means:
 *  - live (from DocumentControls): loads the version into the open canvas —
 *    the caller applies it via mergeRestoredElements and a normal local edit.
 *  - offline (from DocumentsPanel): opens a fresh session seeded from that
 *    version rather than the latest one.
 * Either way, restoring never rewrites history — it just becomes the new
 * latest version after the caller's next Save.
 */
export function VersionHistory({
  token,
  sceneId,
  onRestore,
}: {
  token: string
  sceneId: string
  onRestore: (versionId: string) => Promise<void>
}) {
  const [versions, setVersions] = useState<VersionSummary[] | null>(null)
  const { busy, error, run } = useAsyncAction()

  useEffect(() => {
    let cancelled = false
    api
      .listVersions(token, sceneId)
      .then((v) => {
        if (!cancelled) setVersions(v)
      })
      .catch(() => {
        if (!cancelled) setVersions([])
      })
    return () => {
      cancelled = true
    }
  }, [token, sceneId])

  return (
    <>
      {versions === null ? (
        <p className="muted">Loading…</p>
      ) : versions.length === 0 ? (
        <p className="muted">No saved versions yet.</p>
      ) : (
        <div className="version-table-wrap">
          <table className="version-table">
            <thead>
              <tr>
                <th>Saved</th>
                <th>By</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {[...versions].reverse().map((v) => (
                <tr key={v.id}>
                  <td>{formatSavedAt(v.createdAt)}</td>
                  <td>{v.savedBy}</td>
                  <td>
                    <button className="btn small" disabled={busy} onClick={() => run(() => onRestore(v.id))}>
                      Restore
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {error && <p className="error">{error}</p>}
    </>
  )
}
