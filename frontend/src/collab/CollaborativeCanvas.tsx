import { Excalidraw } from '@excalidraw/excalidraw'
import '@excalidraw/excalidraw/index.css'
import { useEffect, useState } from 'react'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'

import { useCollaborativeSync, type CollaborativeSyncProps, type SceneSeed } from './useCollaborativeSync'
import { centerOnAnchor, findAnchor } from './anchor'

export type { SceneSeed }

export interface CollaborativeCanvasProps extends CollaborativeSyncProps {
  /** Exposes the Excalidraw API so the surrounding UI can read the live scene. */
  onApiChange?: (api: ExcalidrawImperativeAPI | null) => void
}

export function CollaborativeCanvas({ onApiChange, ...syncProps }: CollaborativeCanvasProps) {
  const [api, setApi] = useState<ExcalidrawImperativeAPI | null>(null)
  // Read once: Excalidraw only consumes initialData during its own mount, so
  // this must be the seed the component was first rendered with.
  const [initialSeed, setInitialSeed] = useState(() => syncProps.seed)

  // Excalidraw defers mounting its real editor behind an internal async
  // language-pack load (it renders a loading placeholder first), so it does
  // NOT consume initialData on our first commit. Wait for the imperative API
  // — only handed to us once the real editor has mounted and already read
  // initialData — before dropping our copy of a possibly large seed; clearing
  // on a bare mount effect races that load and reliably loses.
  useEffect(() => {
    if (api) setInitialSeed(undefined)
  }, [api])

  const { handleChange, handlePointerUpdate } = useCollaborativeSync(api, syncProps)

  const handleApi = (next: ExcalidrawImperativeAPI) => {
    setApi(next)
    onApiChange?.(next)
  }

  return (
    <div style={{ position: 'absolute', inset: 0 }}>
      <Excalidraw
        excalidrawAPI={handleApi}
        onChange={handleChange}
        onPointerUpdate={handlePointerUpdate}
        isCollaborating
        initialData={
          initialSeed
            ? {
                elements: initialSeed.elements,
                files: initialSeed.files,
                appState: centerOnAnchor(initialSeed.elements),
                scrollToContent: !findAnchor(initialSeed.elements),
              }
            : undefined
        }
      />
    </div>
  )
}
