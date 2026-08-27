// Pure merge logic for restoring a saved version into a live canvas. Kept
// free of React and the Excalidraw component tree so it can be unit-tested
// against plain element arrays, the same way sync.ts is.

import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'

// Excalidraw exports a `bumpVersion` helper with equivalent semantics, but
// only from its top-level package entry, which pulls in the entire component
// bundle (including roughjs) — unusable from a plain unit-tested module like
// this one. Hand-rolling the same three fields keeps this file dependency-free.
function bumpVersion<T extends { version: number; versionNonce: number; updated: number }>(
  element: T,
  version?: number,
): T {
  element.version = (version ?? element.version) + 1
  element.versionNonce = Math.floor(Math.random() * 2 ** 31)
  element.updated = Date.now()
  return element
}

/**
 * Merge a restored (historical) version's elements against the current live
 * scene, producing the *complete* element list to hand to Excalidraw's
 * `updateScene` — which replaces the scene wholesale, so nothing here may be
 * silently dropped. Elements are matched by id:
 *
 *  - present in both: adopt the restored content, but bump its version/nonce
 *    past the live copy's version. A version loaded from history typically
 *    carries an older, lower version number for the same id, and
 *    `collectElementUpdates` (sync.ts) only forwards a version *increase* —
 *    without this bump the restore would silently fail to propagate to
 *    collaborators for any element that still exists live.
 *  - present only in the restored version: passed through as-is — a brand
 *    new id always propagates, regardless of its version number.
 *  - present only in the live scene (drawn after the restored save point):
 *    synthesized as a deletion tombstone (`isDeleted: true`, version
 *    bumped), mirroring the deletion convention documented in sync.ts.
 *  - already deleted live and absent from the restored version: passed
 *    through unchanged.
 */
export function mergeRestoredElements(
  current: readonly OrderedExcalidrawElement[],
  restored: readonly OrderedExcalidrawElement[],
): OrderedExcalidrawElement[] {
  const currentById = new Map(current.map((el) => [el.id, el]))
  const restoredIds = new Set(restored.map((el) => el.id))
  const merged: OrderedExcalidrawElement[] = []

  for (const el of restored) {
    const existing = currentById.get(el.id)
    const clone = structuredClone(el)
    if (existing) bumpVersion(clone, existing.version)
    merged.push(clone)
  }

  for (const el of current) {
    if (restoredIds.has(el.id)) continue
    if (el.isDeleted) {
      merged.push(el)
      continue
    }
    merged.push({ ...bumpVersion(structuredClone(el)), isDeleted: true })
  }

  return merged
}
