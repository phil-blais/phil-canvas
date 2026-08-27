// Session state: whether an admin is signed in (holding a Go admin JWT), and
// which room, if any, the user is currently in. The Go JWT lives only in memory
// — no persistence — matching the plan. On load, an existing Firebase session is
// silently re-exchanged for a fresh Go JWT (handles page refresh).

import { useCallback, useEffect, useRef, useState } from 'react'
import type { User } from 'firebase/auth'
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import { ApiError, api } from '../api/client'
import type { SceneSeed } from '../collab/CollaborativeCanvas'
import { fetchPublishedScene, fetchSceneFiles } from '../api/storage'
import { useLatestOnly } from '../hooks/useLatestOnly'
import {
  firebaseEnabled,
  getIdToken,
  signInWithGoogle,
  signOutAdmin,
  watchAuth,
} from '../auth/firebase'

/** Fetch a version's files and pack it into the shape a canvas seed expects. */
async function buildSeed(data: { elements: unknown; fileIds?: string[] }): Promise<SceneSeed> {
  const files = await fetchSceneFiles(data.fileIds ?? [])
  return { elements: data.elements as OrderedExcalidrawElement[], files }
}

export interface AdminSession {
  token: string
  user: User
}

export interface ActiveRoom {
  id: string
  name: string
  role: 'admin' | 'guest'
  /** Go JWT used for the WebSocket handshake. */
  token: string
  /** Guest code, shown to the owning admin to share. */
  code?: string
  /** Firestore scene id; drives first-save behavior (see docs/SRS.md FR-4.3). */
  sceneId?: string
  /** Initial scene the creating admin applies client-side (seeded rooms). */
  seed?: SceneSeed
}

export interface Session {
  admin: AdminSession | null
  room: ActiveRoom | null
  /** True once the initial Firebase auth check has resolved. */
  ready: boolean
  firebaseEnabled: boolean
  /** Why the admin exchange failed after a successful Firebase sign-in, if any. */
  authError: string | null
  signIn: () => Promise<void>
  signOut: () => Promise<void>
  /** Start a new document (defaults to "Untitled"), seeded from the currently
   * published scene if one exists, else blank. */
  createDocument: (name?: string) => Promise<void>
  /**
   * Open a saved document: joins its canonical live session if one is
   * already open, else creates one seeded from versionId (or the latest
   * saved version, if versionId is omitted).
   */
  openScene: (scene: { id: string; name: string }, versionId?: string) => Promise<void>
  joinAsGuest: (roomId: string, name: string, code: string) => Promise<void>
  enterAsAdmin: (roomId: string, name: string) => void
  leaveRoom: () => void
  closeRoom: () => Promise<void>
  /** Rename the current document — the not-yet-saved room title if there's no
   * scene yet, else the saved scene itself. */
  renameDocument: (name: string) => Promise<void>
  /** Record the scene id after the first save so later saves append silently. */
  markSceneId: (sceneId: string) => void
  /** Drop the seed once the canvas has applied it — it can be a large payload
   * (inlined images) with no further use for the rest of the room membership. */
  clearSeed: () => void
}

export function useSession(): Session {
  const [admin, setAdmin] = useState<AdminSession | null>(null)
  const [room, setRoom] = useState<ActiveRoom | null>(null)
  const [ready, setReady] = useState(!firebaseEnabled)
  const [authError, setAuthError] = useState<string | null>(null)

  // Latest admin token for callbacks that shouldn't re-bind on every change.
  const adminRef = useRef<AdminSession | null>(null)
  adminRef.current = admin

  // onAuthStateChanged can fire again (sign-out, a different sign-in) while a
  // previous invocation's adminLogin exchange is still in flight. Without
  // this, a stale response arriving after a newer auth event would clobber
  // the state that event already set.
  const { next, isCurrent } = useLatestOnly()

  useEffect(() => {
    return watchAuth(async (user) => {
      const seq = next()
      if (user) {
        try {
          const { token } = await api.adminLogin(await getIdToken(user))
          if (!isCurrent(seq)) return // superseded by a newer auth event
          setAdmin({ token, user })
          setAuthError(null)
        } catch (err) {
          if (!isCurrent(seq)) return
          // Signed into Firebase, but the admin exchange failed — surface why
          // (e.g. 403 not on the allowlist, or 401 token verification).
          setAdmin(null)
          setAuthError(
            err instanceof ApiError
              ? `Admin sign-in failed (${err.status}): ${err.message}`
              : 'Admin sign-in failed: could not reach the backend',
          )
          console.error('admin login failed', err)
        }
      } else {
        setAdmin(null)
      }
      setReady(true)
    })
  }, [next, isCurrent])

  const signIn = useCallback(() => signInWithGoogle(), [])

  const signOut = useCallback(async () => {
    setRoom(null)
    setAuthError(null)
    await signOutAdmin()
    setAdmin(null)
  }, [])

  const createDocument = useCallback(async (name?: string) => {
    const current = adminRef.current
    if (!current) throw new Error('not signed in as admin')
    const created = await api.createRoom(current.token, name)
    const published = await fetchPublishedScene()
    setRoom({
      id: created.id,
      name: created.name,
      role: 'admin',
      token: current.token,
      code: created.code,
      seed: published ? await buildSeed(published) : undefined,
    })
  }, [])

  const openScene = useCallback(async (scene: { id: string; name: string }, versionId?: string) => {
    const current = adminRef.current
    if (!current) throw new Error('not signed in as admin')
    const opened = await api.openScene(current.token, scene.id, { name: scene.name, versionId })

    let seed: SceneSeed | undefined
    if (!opened.live) {
      const version = versionId
        ? await api.getVersion(current.token, scene.id, versionId)
        : await (async () => {
            const versions = await api.listVersions(current.token, scene.id)
            if (versions.length === 0) return null
            const latest = versions[versions.length - 1]
            return api.getVersion(current.token, scene.id, latest.id)
          })()
      if (version) seed = await buildSeed(version)
    }

    setRoom({
      id: opened.id,
      name: opened.name,
      role: 'admin',
      token: current.token,
      code: opened.code,
      sceneId: opened.sceneId,
      seed,
    })
  }, [])

  const joinAsGuest = useCallback(async (roomId: string, name: string, code: string) => {
    const { token } = await api.guestLogin(roomId, code)
    setRoom({ id: roomId, name, role: 'guest', token })
  }, [])

  const enterAsAdmin = useCallback((roomId: string, name: string) => {
    const current = adminRef.current
    if (!current) throw new Error('not signed in as admin')
    setRoom({ id: roomId, name, role: 'admin', token: current.token })
  }, [])

  const leaveRoom = useCallback(() => setRoom(null), [])

  const markSceneId = useCallback((sceneId: string) => {
    setRoom((current) => (current && !current.sceneId ? { ...current, sceneId } : current))
  }, [])

  const clearSeed = useCallback(() => {
    setRoom((current) => (current?.seed ? { ...current, seed: undefined } : current))
  }, [])

  const closeRoom = useCallback(async () => {
    if (room?.role === 'admin' && adminRef.current) {
      // Fire-and-forget; the relay closes everyone (including us) with 4001.
      void api.closeRoom(adminRef.current.token, room.id).catch(() => {})
    }
    setRoom(null)
  }, [room])

  const renameDocument = useCallback(
    async (name: string) => {
      const current = adminRef.current
      if (!current || !room) throw new Error('not in a room')
      const res = room.sceneId
        ? await api.renameScene(current.token, room.sceneId, name)
        : await api.renameRoom(current.token, room.id, name)
      setRoom((r) => (r ? { ...r, name: res.name } : r))
    },
    [room],
  )

  return {
    admin,
    room,
    ready,
    firebaseEnabled,
    authError,
    signIn,
    signOut,
    createDocument,
    openScene,
    joinAsGuest,
    enterAsAdmin,
    leaveRoom,
    closeRoom,
    renameDocument,
    markSceneId,
    clearSeed,
  }
}
