import { useEffect } from 'react'
import { VersionHistory } from './VersionHistory'

function CloseIcon() {
  return (
    <svg viewBox="0 0 20 20" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true" focusable="false">
      <path d="M5 5l10 10M15 5 5 15" />
    </svg>
  )
}

/**
 * Version history as its own docked panel, alongside the main flyout rather
 * than swapped in over it — so the document you're browsing history for (its
 * title, its Save/Publish controls, its row in the documents list) stays
 * visible the whole time, instead of history feeling disconnected from what
 * it's the history *of*.
 */
export function VersionHistoryPanel({
  documentName,
  token,
  sceneId,
  onRestore,
  onClose,
}: {
  documentName: string
  token: string
  sceneId: string
  onRestore: (versionId: string) => Promise<void>
  onClose: () => void
}) {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  return (
    <div className="history-panel">
      <div className="history-panel-header">
        <div>
          <h3>Version history</h3>
          <p className="muted">{documentName}</p>
        </div>
        <button className="btn small secondary icon-btn" aria-label="Close version history" onClick={onClose}>
          <CloseIcon />
        </button>
      </div>
      <div className="history-panel-body">
        <VersionHistory token={token} sceneId={sceneId} onRestore={onRestore} />
      </div>
    </div>
  )
}
