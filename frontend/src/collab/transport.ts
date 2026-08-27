// Resolves where the collab relay lives, for whichever environment the app is
// running in.

/** Base WebSocket URL up to and including `/ws` (no trailing slash). */
export function resolveWsBaseUrl(): string {
  const backendOrigin = import.meta.env.VITE_BACKEND_ORIGIN
  // Production: connect straight to the Cloud Run origin. Firebase Hosting does
  // not proxy WebSocket upgrades — routed through location.host, the upgrade
  // headers are stripped and the relay rejects the plain GET with a 400.
  if (!import.meta.env.DEV && backendOrigin) {
    return `${backendOrigin.replace(/^http/, 'ws')}/ws`
  }
  // Dev: the Vite proxy serves /ws same-origin (see vite.config.ts).
  return `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`
}
