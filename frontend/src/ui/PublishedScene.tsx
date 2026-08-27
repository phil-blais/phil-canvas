import { Excalidraw } from '@excalidraw/excalidraw'
import '@excalidraw/excalidraw/index.css'
import { useEffect, useState } from 'react'
import type { AppState, BinaryFiles, ExcalidrawInitialDataState } from '@excalidraw/excalidraw/types'
import type { ExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import { fetchPublishedScene, fetchSceneFiles } from '../api/storage'
import { useLatestOnly } from '../hooks/useLatestOnly'

type State =
  | { status: 'loading' }
  | { status: 'empty' }
  | { status: 'ready'; data: ExcalidrawInitialDataState }

/**
 * The public-facing site: the published scene rendered read-only. Fetched from
 * Storage at load; falls back to a placeholder when nothing is published yet.
 */
export function PublishedScene() {
  const [state, setState] = useState<State>({ status: 'loading' })
  const { next, isCurrent } = useLatestOnly()

  useEffect(() => {
    const token = next()
    void (async () => {
      const scene = await fetchPublishedScene()
      if (!isCurrent(token)) return
      if (!scene) {
        setState({ status: 'empty' })
        return
      }
      const files = await fetchSceneFiles(scene.fileIds ?? [])
      if (!isCurrent(token)) return
      setState({
        status: 'ready',
        data: {
          elements: scene.elements as ExcalidrawElement[],
          appState: scene.appState as Partial<AppState>,
          files: files as BinaryFiles,
        },
      })
    })()
    return () => {
      next() // invalidate: any response still in flight is now stale
    }
  }, [next, isCurrent])

  if (state.status === 'loading') {
    return <div className="landing-backdrop">Loading…</div>
  }
  if (state.status === 'empty') {
    return <div className="landing-backdrop">Canvas</div>
  }

  return (
    <div style={{ position: 'absolute', inset: 0 }}>
      <Excalidraw initialData={state.data} viewModeEnabled />
    </div>
  )
}
