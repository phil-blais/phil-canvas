// Typed client for the Go backend. All paths are same-origin: the Vite dev
// proxy (and Firebase Hosting rewrites in prod) route them to the service, so no
// base URL is needed.

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export interface RoomSummary {
  id: string
  name: string
  participantCount: number
  sceneId?: string
}

export interface CreateRoomResult {
  id: string
  name: string
  code: string
}

export interface OpenSceneResult {
  id: string
  name: string
  code: string
  sceneId: string
  /** True when this joined an already-live session rather than creating one. */
  live: boolean
}

export interface SceneSummary {
  id: string
  name: string
  createdAt: string
  createdBy: string
}

export interface VersionSummary {
  id: string
  createdAt: string
  savedBy: string
}

export interface VersionData {
  elements: unknown
  appState: unknown
  fileIds: string[]
}

/** The elements/appState/files snapshot posted to save and publish. */
export interface ScenePayload {
  elements: unknown
  appState: unknown
  files: unknown
}

interface RequestOptions {
  body?: unknown
  token?: string
}

async function request<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {}
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (opts.token) headers['Authorization'] = `Bearer ${opts.token}`

  const res = await fetch(path, {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })

  const text = await res.text()
  const data = text ? JSON.parse(text) : undefined

  if (!res.ok) {
    const message = (data as { error?: string } | undefined)?.error ?? res.statusText
    throw new ApiError(res.status, message)
  }
  return data as T
}

export const api = {
  /** Exchange a Firebase ID token for a Go admin JWT (allowlist-gated server-side). */
  adminLogin: (idToken: string) => request<{ token: string }>('POST', '/auth/admin', { body: { idToken } }),

  /** Exchange a room id + 4-char code for a room-bound guest JWT. */
  guestLogin: (room: string, code: string) =>
    request<{ token: string; room: string }>('POST', '/auth/guest', { body: { room, code } }),

  /** Public list of currently-live sessions. */
  listRooms: () => request<RoomSummary[]>('GET', '/rooms'),

  /** Create a brand-new blank/untitled room (admin). */
  createRoom: (token: string, name?: string) =>
    request<CreateRoomResult>('POST', '/rooms', { body: { name: name ?? 'Untitled' }, token }),

  /** Close a room (any admin). */
  closeRoom: (token: string, id: string) => request<void>('DELETE', `/rooms/${id}`, { token }),

  /** Rename a not-yet-saved room's draft title (any admin). */
  renameRoom: (token: string, roomId: string, name: string) =>
    request<{ id: string; name: string }>('PATCH', `/rooms/${roomId}/name`, { body: { name }, token }),

  /** Save the current canvas as a new version (any admin). */
  saveScene: (token: string, roomId: string, payload: ScenePayload) =>
    request<{ sceneId: string; versionId: string }>('POST', `/rooms/${roomId}/save`, { body: payload, token }),

  /** Publish the current canvas to the public scene (any admin). */
  publishScene: (token: string, roomId: string, payload: ScenePayload) =>
    request<{ published: boolean }>('POST', `/rooms/${roomId}/publish`, { body: payload, token }),

  /** List saved scenes (admin). */
  listScenes: (token: string) => request<SceneSummary[]>('GET', '/scenes', { token }),

  /**
   * Open a document (admin): joins its canonical live session if one exists,
   * else creates one. name seeds a freshly-created session's title; versionId
   * optionally selects which saved version to seed from (ignored when joining
   * an already-live session).
   */
  openScene: (token: string, sceneId: string, opts: { name: string; versionId?: string }) =>
    request<OpenSceneResult>('POST', `/scenes/${sceneId}/open`, { body: opts, token }),

  /** Rename a saved scene (admin). Syncs a currently-live session's title too. */
  renameScene: (token: string, sceneId: string, name: string) =>
    request<{ id: string; name: string }>('PATCH', `/scenes/${sceneId}`, { body: { name }, token }),

  /** Delete a saved scene and its version history (admin). 409s if it's currently live. */
  deleteScene: (token: string, sceneId: string) => request<void>('DELETE', `/scenes/${sceneId}`, { token }),

  /** List a scene's versions, oldest first (admin). */
  listVersions: (token: string, sceneId: string) =>
    request<VersionSummary[]>('GET', `/scenes/${sceneId}/versions`, { token }),

  /** Fetch a version's full data (admin). */
  getVersion: (token: string, sceneId: string, versionId: string) =>
    request<VersionData>('GET', `/scenes/${sceneId}/versions/${versionId}`, { token }),
}
