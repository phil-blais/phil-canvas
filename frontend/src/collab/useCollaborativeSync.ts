// Yjs/WebSocket sync wiring for the collaborative canvas: doc lifecycle,
// transport, remote reconciliation, and awareness/presence. Kept separate from
// CollaborativeCanvas.tsx so that component owns only Excalidraw mount state
// and the render tree — this hook owns everything else.

import { useCallback, useEffect, useRef } from 'react'
import { CaptureUpdateAction, reconcileElements } from '@excalidraw/excalidraw'
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'
import type { Awareness } from 'y-protocols/awareness'
import type { AppState, BinaryFileData, BinaryFiles, ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { RemoteExcalidrawElement } from '@excalidraw/excalidraw/data/reconcile'

import { CLOSE_AUTH_FAILED, CLOSE_ROOM_CLOSED, isRoomClosedMessage, makeAuthWebSocket } from './authSocket'
import {
  collectElementUpdates,
  collectNewFiles,
  LOCAL_ORIGIN,
  readElements,
  readFiles,
  writeElements,
  writeFiles,
  type YElements,
  type YFiles,
} from './sync'
import { buildCollaborators, type AwarenessState, type AwarenessUser } from './awareness'

export interface SceneSeed {
  elements: OrderedExcalidrawElement[]
  files?: BinaryFiles
}

export interface CollaborativeSyncProps {
  /** Room id; the relay path is `${wsBaseUrl}/${roomId}`. */
  roomId: string
  /** Base WebSocket URL up to and including `/ws` (no trailing slash). */
  wsBaseUrl: string
  /** Returns the current Go-issued JWT; read at every (re)connect. */
  getToken: () => string
  /** Local user identity for presence. */
  user: AwarenessUser
  /** Optional initial scene, applied client-side by the creating admin. */
  seed?: SceneSeed
  /** Called when the room is closed (room-closed message or close code 4001). */
  onRoomClosed?: () => void
  /** Called when the relay rejects auth (close code 4003). */
  onAuthError?: () => void
  /** Called once the seed (if any) has been applied to the shared doc — the
   * caller can drop its copy, since this hook holds no reference to it after. */
  onSeedConsumed?: () => void
}

interface Binding {
  api: ExcalidrawImperativeAPI
  doc: Y.Doc
  yElements: YElements
  yFiles: YFiles
  awareness: Awareness
  applyingRemote: boolean
}

/**
 * Wires the given Excalidraw API to the room's shared Yjs document over
 * WebSocket, and returns the `onChange`/`onPointerUpdate` handlers Excalidraw
 * needs to publish local edits and presence back into that document.
 */
export function useCollaborativeSync(api: ExcalidrawImperativeAPI | null, props: CollaborativeSyncProps) {
  const { roomId, wsBaseUrl } = props
  const bindingRef = useRef<Binding | null>(null)

  // Keep the latest callback-style props in a ref so the effect below can read
  // current values without re-subscribing when they change identity.
  const propsRef = useRef(props)
  propsRef.current = props

  useEffect(() => {
    if (!api) return

    const doc = new Y.Doc()
    const yElements = doc.getMap<OrderedExcalidrawElement>('elements')
    const yFiles = doc.getMap<BinaryFileData>('files')

    const binding: Binding = { api, doc, yElements, yFiles, awareness: null!, applyingRemote: false }
    bindingRef.current = binding

    // Seed (creating admin) before wiring anything else, so the seed becomes the
    // document's initial state and propagates to later joiners.
    const seed = propsRef.current.seed
    if (seed) {
      writeElements(doc, yElements, seed.elements)
      if (seed.files) writeFiles(doc, yFiles, Object.values(seed.files))
    }

    // Stop reconnection and notify. Guarded so either close signal fires once.
    let terminated = false
    const terminate = (notify?: () => void) => {
      if (terminated) return
      terminated = true
      provider.disconnect()
      notify?.()
    }

    const provider = new WebsocketProvider(wsBaseUrl, roomId, doc, {
      WebSocketPolyfill: makeAuthWebSocket(() => propsRef.current.getToken(), {
        onTextMessage: (data) => {
          if (isRoomClosedMessage(data)) terminate(propsRef.current.onRoomClosed)
        },
        onClose: (code) => {
          if (code === CLOSE_ROOM_CLOSED) terminate(propsRef.current.onRoomClosed)
          else if (code === CLOSE_AUTH_FAILED) terminate(propsRef.current.onAuthError)
        },
      }),
    })

    // Merge the shared elements into the local scene. reconcileElements resolves
    // per-element by version/versionNonce (Excalidraw's own collab algorithm), so
    // updates to existing elements are applied, not just new ones. NEVER keeps
    // remote changes out of this client's undo/redo stack.
    const applyRemoteElements = () => {
      binding.applyingRemote = true
      try {
        const reconciled = reconcileElements(
          api.getSceneElementsIncludingDeleted(),
          readElements(yElements) as unknown as RemoteExcalidrawElement[],
          api.getAppState(),
        )
        api.updateScene({ elements: reconciled, captureUpdate: CaptureUpdateAction.NEVER })
      } finally {
        binding.applyingRemote = false
      }
    }

    const onElements = (_e: unknown, txn: Y.Transaction) => {
      if (txn.origin === LOCAL_ORIGIN) return
      applyRemoteElements()
    }
    yElements.observe(onElements)

    // Add referenced files before the elements that use them are painted.
    const syncFiles = () => api.addFiles(readFiles(yFiles))

    const onFiles = (_e: unknown, txn: Y.Transaction) => {
      if (txn.origin === LOCAL_ORIGIN) return
      syncFiles()
    }
    yFiles.observe(onFiles)

    // Awareness / presence.
    const awareness = provider.awareness
    binding.awareness = awareness
    awareness.setLocalStateField('user', propsRef.current.user)
    const onAwareness = (changes: { added: number[]; updated: number[]; removed: number[] }) => {
      // buildCollaborators excludes the local client, so a change that only
      // touched our own state (e.g. every local pointer move) can't affect
      // the collaborators it produces — skip the rebuild in that case.
      const isLocalEcho =
        changes.added.length === 0 && changes.removed.length === 0 && changes.updated.every((id) => id === awareness.clientID)
      if (isLocalEcho) return

      const states = awareness.getStates() as Map<number, AwarenessState>
      // Collaborators are applied via updateScene (not a prop) in v0.18.
      api.updateScene({
        collaborators: buildCollaborators(states, awareness.clientID),
        captureUpdate: CaptureUpdateAction.NEVER,
      })
    }
    awareness.on('change', onAwareness)

    // If we seeded, the initialData prop (below) paints the local scene, so
    // just push files into the CRDT-visible map — no observer fires for local
    // writes. (Painting here too would race Excalidraw's own async
    // initializeScene, which restores initialData and can clobber a scene set
    // via updateScene before it resolves.)
    if (seed) {
      syncFiles()
      // The caller's copy (elements + any inlined image data) has now been
      // fully handed off to the shared doc; nothing here needs it again.
      propsRef.current.onSeedConsumed?.()
    }

    return () => {
      yElements.unobserve(onElements)
      yFiles.unobserve(onFiles)
      awareness.off('change', onAwareness)
      provider.destroy()
      doc.destroy()
      bindingRef.current = null
    }
  }, [api, roomId, wsBaseUrl])

  const handleChange = useCallback(
    (_elements: readonly OrderedExcalidrawElement[], appState: AppState, files: BinaryFiles) => {
      const b = bindingRef.current
      if (!b || b.applyingRemote) return
      // Use including-deleted so deletions (isDeleted:true) propagate too.
      const all = b.api.getSceneElementsIncludingDeleted()
      writeElements(b.doc, b.yElements, collectElementUpdates(all, b.yElements))
      writeFiles(b.doc, b.yFiles, collectNewFiles(files, b.yFiles))
      b.awareness.setLocalStateField('selectedElementIds', appState.selectedElementIds)
    },
    [],
  )

  const handlePointerUpdate = useCallback(
    (payload: { pointer: { x: number; y: number; tool: 'pointer' | 'laser' }; button: 'down' | 'up' }) => {
      const b = bindingRef.current
      if (!b) return
      b.awareness.setLocalStateField('pointer', payload.pointer)
      b.awareness.setLocalStateField('button', payload.button)
    },
    [],
  )

  return { handleChange, handlePointerUpdate }
}
