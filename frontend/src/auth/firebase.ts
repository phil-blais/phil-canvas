// Firebase Auth wrapper for admin sign-in. If Firebase config is absent (no
// VITE_FIREBASE_API_KEY), admin sign-in is disabled but the rest of the app —
// including the entire guest flow — still works.

import { initializeApp } from 'firebase/app'
import {
  GoogleAuthProvider,
  connectAuthEmulator,
  getAuth,
  onAuthStateChanged,
  signInWithPopup,
  signOut,
  type Auth,
  type User,
} from 'firebase/auth'

const config = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
  appId: import.meta.env.VITE_FIREBASE_APP_ID,
}

/** Whether admin sign-in is available (Firebase configured). */
export const firebaseEnabled = Boolean(config.apiKey)

let auth: Auth | null = null
if (firebaseEnabled) {
  const app = initializeApp(config)
  auth = getAuth(app)
  const emulatorUrl = import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_URL
  if (emulatorUrl) {
    connectAuthEmulator(auth, emulatorUrl, { disableWarnings: true })
  }
}

function requireAuth(): Auth {
  if (!auth) throw new Error('Firebase auth is not configured')
  return auth
}

/** Open the Google sign-in popup. */
export async function signInWithGoogle(): Promise<void> {
  await signInWithPopup(requireAuth(), new GoogleAuthProvider())
}

/**
 * Subscribe to auth state. If Firebase is disabled the callback is invoked once
 * with null and the returned unsubscribe is a no-op.
 */
export function watchAuth(callback: (user: User | null) => void): () => void {
  if (!auth) {
    callback(null)
    return () => {}
  }
  return onAuthStateChanged(auth, callback)
}

/** Current Firebase ID token for the signed-in user. */
export function getIdToken(user: User): Promise<string> {
  return user.getIdToken()
}

/** Sign the admin out of Firebase. */
export async function signOutAdmin(): Promise<void> {
  if (auth) await signOut(auth)
}
