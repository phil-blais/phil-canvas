// Pure Yjs <-> Excalidraw sync logic. Kept free of React and the transport so it
// can be unit-tested against a real Y.Doc.
//
// Elements are stored in a Y.Map keyed by element id. The map is unordered;
// z-order is reconstructed from each element's fractional `index`, so map
// iteration order does not matter. Image files live in a separate, add-only
// Y.Map keyed by their content-hash id.

import * as Y from 'yjs'
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { BinaryFileData, BinaryFiles } from '@excalidraw/excalidraw/types'

/** Transaction origin marking a write as local, so observers ignore the echo. */
export const LOCAL_ORIGIN = 'local'

export type YElements = Y.Map<OrderedExcalidrawElement>
export type YFiles = Y.Map<BinaryFileData>

/**
 * Collect elements whose local version is newer than the shared copy. This one
 * rule covers adds (no existing entry), updates, and deletions alike: Excalidraw
 * marks a deletion by bumping `version` and setting `isDeleted: true`, so a
 * deleted element propagates like any other change — never remove it from the
 * map. The version gate is also what prevents echoing a just-applied remote
 * element back to peers.
 */
export function collectElementUpdates(
  elements: readonly OrderedExcalidrawElement[],
  yElements: YElements,
): OrderedExcalidrawElement[] {
  const updates: OrderedExcalidrawElement[] = []
  for (const el of elements) {
    const existing = yElements.get(el.id)
    if (!existing || el.version > existing.version) {
      updates.push(el)
    }
  }
  return updates
}

/** Write element updates to the shared map in one local-origin transaction. */
export function writeElements(
  doc: Y.Doc,
  yElements: YElements,
  updates: readonly OrderedExcalidrawElement[],
): void {
  if (updates.length === 0) return
  doc.transact(() => {
    // Store an immutable snapshot, not the live element. Y.Map returns the same
    // reference it was given for local reads, and Excalidraw mutates elements in
    // place (bumping `version`) — so storing the live object would make the Y.Map
    // copy and the scene element the same object, and the version gate in
    // collectElementUpdates could never fire again after the first write.
    for (const el of updates) yElements.set(el.id, structuredClone(el))
  }, LOCAL_ORIGIN)
}

/**
 * Read every element from the shared map (order is derived downstream). Returns
 * clones: the scene must never hold a reference to a Y.Map value, or Excalidraw's
 * in-place mutation of an applied element would corrupt the shared copy and stall
 * the version gate (the mirror of the clone-on-write in writeElements).
 */
export function readElements(yElements: YElements): OrderedExcalidrawElement[] {
  return Array.from(yElements.values(), (el) => structuredClone(el))
}

/**
 * Collect image files present locally but not yet shared. Files are immutable
 * and content-addressed, so this is add-only — existing ids are never rewritten.
 */
export function collectNewFiles(files: BinaryFiles, yFiles: YFiles): BinaryFileData[] {
  const added: BinaryFileData[] = []
  for (const id of Object.keys(files)) {
    if (!yFiles.has(id)) added.push(files[id])
  }
  return added
}

/** Write new files to the shared map in one local-origin transaction. */
export function writeFiles(doc: Y.Doc, yFiles: YFiles, added: readonly BinaryFileData[]): void {
  if (added.length === 0) return
  doc.transact(() => {
    for (const file of added) yFiles.set(file.id, file)
  }, LOCAL_ORIGIN)
}

/** Read every shared file. */
export function readFiles(yFiles: YFiles): BinaryFileData[] {
  return Array.from(yFiles.values())
}
