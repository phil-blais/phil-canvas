import { serializeAsJSON } from '@excalidraw/excalidraw'
import type { ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import type { ScenePayload } from '../api/client'

/**
 * Build the save/publish payload from the live canvas. serializeAsJSON cleans
 * appState (dropping transient/non-serializable fields like the collaborators
 * Map) and filters files to only those referenced by elements.
 */
export function currentScenePayload(api: ExcalidrawImperativeAPI): ScenePayload {
  const json = serializeAsJSON(api.getSceneElements(), api.getAppState(), api.getFiles(), 'local')
  const { elements, appState, files } = JSON.parse(json) as ScenePayload
  return { elements, appState, files }
}
