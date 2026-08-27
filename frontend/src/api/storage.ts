// Reads public artifacts (published scene JSON and image files) directly from
// Firebase Storage. Access is governed by storage.rules (public read on
// public/**); the base URL points at the Storage media endpoint, e.g.
//   https://firebasestorage.googleapis.com/v0/b/<bucket>/o
// so the object path is percent-encoded and fetched with ?alt=media.

import type { BinaryFileData, BinaryFiles } from '@excalidraw/excalidraw/types'

/** Storage media base URL, read lazily so tests can stub the env. */
function baseUrl(): string {
  return (import.meta.env.VITE_PUBLISHED_BASE_URL ?? '').replace(/\/$/, '')
}

/** Whether a public Storage base URL is configured. */
export function publishedConfigured(): boolean {
  return baseUrl() !== ''
}

function downloadUrl(path: string): string {
  return `${baseUrl()}/${encodeURIComponent(path)}?alt=media`
}

export interface PublishedScene {
  elements: unknown
  appState: unknown
  fileIds?: string[]
}

/** Fetch the published scene JSON, or null if none / not configured. */
export async function fetchPublishedScene(): Promise<PublishedScene | null> {
  if (!baseUrl()) return null
  try {
    const res = await fetch(downloadUrl('public/published-scene.json'))
    if (!res.ok) return null
    return (await res.json()) as PublishedScene
  } catch {
    return null
  }
}

/**
 * Load the given file ids into a BinaryFiles map, best-effort — a missing or
 * unreachable file is skipped rather than failing the whole load.
 */
export async function fetchSceneFiles(fileIds: string[]): Promise<BinaryFiles> {
  const files: Record<string, BinaryFileData> = {}
  if (!baseUrl() || fileIds.length === 0) return files as BinaryFiles
  await Promise.all(
    fileIds.map(async (id) => {
      try {
        const res = await fetch(downloadUrl(`public/files/${id}`))
        if (res.ok) files[id] = (await res.json()) as BinaryFileData
      } catch {
        // best effort
      }
    }),
  )
  return files as BinaryFiles
}
