import { describe, expect, it } from 'vitest'
import * as Y from 'yjs'
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { BinaryFileData, BinaryFiles } from '@excalidraw/excalidraw/types'
import {
  collectElementUpdates,
  collectNewFiles,
  LOCAL_ORIGIN,
  readElements,
  readFiles,
  writeElements,
  writeFiles,
  type YElements,
  type YFiles,
} from './sync'

function el(id: string, version: number, extra: Partial<OrderedExcalidrawElement> = {}): OrderedExcalidrawElement {
  return { id, version, isDeleted: false, ...extra } as unknown as OrderedExcalidrawElement
}

function file(id: string, dataURL = 'data:image/png;base64,AAAA'): BinaryFileData {
  return { id, dataURL, mimeType: 'image/png', created: 0 } as unknown as BinaryFileData
}

function newElements(): { doc: Y.Doc; yElements: YElements } {
  const doc = new Y.Doc()
  return { doc, yElements: doc.getMap<OrderedExcalidrawElement>('elements') }
}

describe('collectElementUpdates', () => {
  it('includes elements not yet in the shared map (adds)', () => {
    const { yElements } = newElements()
    expect(collectElementUpdates([el('a', 1)], yElements).map((e) => e.id)).toEqual(['a'])
  })

  it('includes elements with a newer version (updates)', () => {
    const { doc, yElements } = newElements()
    writeElements(doc, yElements, [el('a', 1)])
    expect(collectElementUpdates([el('a', 2)], yElements).map((e) => e.id)).toEqual(['a'])
  })

  it('excludes elements with the same version (echo prevention)', () => {
    const { doc, yElements } = newElements()
    writeElements(doc, yElements, [el('a', 5)])
    expect(collectElementUpdates([el('a', 5)], yElements)).toEqual([])
  })

  it('excludes elements with an older version (stale local)', () => {
    const { doc, yElements } = newElements()
    writeElements(doc, yElements, [el('a', 5)])
    expect(collectElementUpdates([el('a', 3)], yElements)).toEqual([])
  })

  it('propagates a deletion as a version-bumped isDeleted update', () => {
    const { doc, yElements } = newElements()
    writeElements(doc, yElements, [el('a', 1)])
    const deleted = collectElementUpdates([el('a', 2, { isDeleted: true })], yElements)
    expect(deleted).toHaveLength(1)
    expect(deleted[0].isDeleted).toBe(true)
    // Deleting must go through as an update, never as a map removal.
    writeElements(doc, yElements, deleted)
    expect(yElements.has('a')).toBe(true)
    expect(yElements.get('a')!.isDeleted).toBe(true)
  })
})

describe('writeElements / readElements', () => {
  it('round-trips and tags the transaction as local origin', () => {
    const { doc, yElements } = newElements()
    let origin: unknown = 'unset'
    yElements.observe((_e, txn) => {
      origin = txn.origin
    })
    writeElements(doc, yElements, [el('a', 1), el('b', 1)])
    expect(origin).toBe(LOCAL_ORIGIN)
    expect(readElements(yElements).map((e) => e.id).sort()).toEqual(['a', 'b'])
  })

  it('is a no-op for an empty update set', () => {
    const { doc, yElements } = newElements()
    let fired = false
    yElements.observe(() => {
      fired = true
    })
    writeElements(doc, yElements, [])
    expect(fired).toBe(false)
  })

  it('stores an immutable snapshot so in-place mutation of the source still syncs', () => {
    // Regression: Excalidraw mutates elements in place (bumping version), and a
    // Y.Map returns the same reference it was given. Storing the live object made
    // the stored copy and the scene element identical, so no update after the
    // first ever propagated (element appeared but never resized).
    const { doc, yElements } = newElements()
    const live = el('a', 1)
    writeElements(doc, yElements, [live])

    // Simulate Excalidraw mutating the element in place.
    ;(live as { version: number }).version = 2

    // The stored snapshot must remain at version 1, so the bump is detectable.
    expect(yElements.get('a')!.version).toBe(1)
    expect(collectElementUpdates([live], yElements).map((e) => e.id)).toEqual(['a'])
  })

  it('returns clones so mutating a read element does not corrupt the shared map', () => {
    // Regression: the receiver applies read elements to its scene; Excalidraw then
    // mutates them in place. If reads shared references with the Y.Map, the
    // receiver's edits would never propagate (the asymmetric half of the bug).
    const { doc, yElements } = newElements()
    writeElements(doc, yElements, [el('a', 1)])
    const [read] = readElements(yElements)
    ;(read as { version: number }).version = 99
    expect(yElements.get('a')!.version).toBe(1)
  })
})

describe('files (add-only, content-addressed)', () => {
  function newFiles(): { doc: Y.Doc; yFiles: YFiles } {
    const doc = new Y.Doc()
    return { doc, yFiles: doc.getMap<BinaryFileData>('files') }
  }

  it('collects only files not already shared', () => {
    const { doc, yFiles } = newFiles()
    writeFiles(doc, yFiles, [file('a')])
    const files: BinaryFiles = { a: file('a'), b: file('b') } as unknown as BinaryFiles
    expect(collectNewFiles(files, yFiles).map((f) => f.id)).toEqual(['b'])
  })

  it('never overwrites an existing file (immutability)', () => {
    const { doc, yFiles } = newFiles()
    writeFiles(doc, yFiles, [file('a', 'ORIGINAL')])
    const files: BinaryFiles = { a: file('a', 'DIFFERENT') } as unknown as BinaryFiles
    expect(collectNewFiles(files, yFiles)).toEqual([])
    expect(yFiles.get('a')!.dataURL).toBe('ORIGINAL')
  })

  it('round-trips through readFiles', () => {
    const { doc, yFiles } = newFiles()
    writeFiles(doc, yFiles, [file('a'), file('b')])
    expect(readFiles(yFiles).map((f) => f.id).sort()).toEqual(['a', 'b'])
  })
})
