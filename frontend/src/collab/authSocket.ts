// AuthWebSocket injects the auth frame that our relay requires before any Yjs
// traffic, and separates our text control frames from binary Yjs frames.
//
// Why a WebSocket subclass: y-websocket builds `new provider._WS(url)` on every
// (re)connect and assigns its own `onopen` (which sends Yjs sync step 1) *after*
// the constructor runs. Because our `open` listener is registered in the
// constructor — before that assignment — it fires first, so the auth frame is
// sent before sync step 1 on every connect and reconnect. WebSocket ordered
// delivery then guarantees the server sees auth first.

/** WebSocket close code the relay uses when an admin closes the room. */
export const CLOSE_ROOM_CLOSED = 4001

/** WebSocket close code the relay uses when the auth handshake fails. */
export const CLOSE_AUTH_FAILED = 4003

export interface AuthSocketHooks {
  /** Called with the raw string payload of any text control frame. */
  onTextMessage?: (data: string) => void
  /** Called with the close code when the connection closes. */
  onClose?: (code: number) => void
}

/**
 * Build a WebSocket subclass suitable for y-websocket's `WebSocketPolyfill`
 * option. getToken is read at open time, so reconnects re-auth with the current
 * token.
 */
export function makeAuthWebSocket(
  getToken: () => string,
  hooks: AuthSocketHooks = {},
): typeof WebSocket {
  return class AuthWebSocket extends WebSocket {
    constructor(url: string | URL, protocols?: string | string[]) {
      super(url, protocols)

      this.addEventListener('open', () => {
        this.send(JSON.stringify({ type: 'auth', token: getToken() }))
      })

      // Yjs frames arrive as ArrayBuffer (binaryType is 'arraybuffer'); our
      // control frames are text (string). Intercept the text frames and stop
      // them reaching y-websocket's onmessage, which would try to parse them as
      // Yjs. This listener is registered before y-websocket assigns onmessage,
      // so stopImmediatePropagation suppresses that handler for this event only.
      this.addEventListener('message', (event: MessageEvent) => {
        if (typeof event.data === 'string') {
          event.stopImmediatePropagation()
          hooks.onTextMessage?.(event.data)
        }
      })

      this.addEventListener('close', (event: CloseEvent) => {
        hooks.onClose?.(event.code)
      })
    }
  }
}

/** Returns true if a text control frame signals the room was closed. */
export function isRoomClosedMessage(data: string): boolean {
  try {
    return (JSON.parse(data) as { type?: string }).type === 'room-closed'
  } catch {
    return false
  }
}
