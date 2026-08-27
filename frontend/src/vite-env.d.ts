/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_BACKEND_ORIGIN?: string
  readonly VITE_PUBLISHED_BASE_URL?: string
  readonly VITE_FIREBASE_API_KEY?: string
  readonly VITE_FIREBASE_AUTH_DOMAIN?: string
  readonly VITE_FIREBASE_PROJECT_ID?: string
  readonly VITE_FIREBASE_APP_ID?: string
  /** e.g. http://localhost:9099 to target the local Auth emulator. */
  readonly VITE_FIREBASE_AUTH_EMULATOR_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
