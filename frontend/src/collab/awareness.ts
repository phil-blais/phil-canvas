// Maps Yjs awareness state to Excalidraw's collaborators, and defines the shape
// we store in local awareness. Cursor, laser pointer (pointer.tool === 'laser'),
// selection, and presence all travel through awareness.

import type { AppState, Collaborator, SocketId } from '@excalidraw/excalidraw/types'

export interface PointerState {
  x: number
  y: number
  tool: 'pointer' | 'laser'
}

export interface AwarenessUser {
  username: string
}

/** The value each client publishes into awareness. */
export interface AwarenessState {
  user?: AwarenessUser
  pointer?: PointerState
  button?: 'up' | 'down'
  selectedElementIds?: AppState['selectedElementIds']
}

/**
 * Build the Map<SocketId, Collaborator> Excalidraw expects from the awareness
 * states, excluding the local client.
 */
export function buildCollaborators(
  states: Map<number, AwarenessState>,
  localClientId: number,
): Map<SocketId, Collaborator> {
  const collaborators = new Map<SocketId, Collaborator>()
  for (const [clientId, state] of states) {
    if (clientId === localClientId) continue
    const socketId = String(clientId) as SocketId
    collaborators.set(socketId, {
      pointer: state.pointer,
      button: state.button,
      selectedElementIds: state.selectedElementIds,
      username: state.user?.username ?? null,
      socketId,
    })
  }
  return collaborators
}
