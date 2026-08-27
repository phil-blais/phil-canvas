import { describe, expect, it } from 'vitest'
import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import { mergeRestoredElements } from './restoreVersion'

function el(id: string, version: number, extra: Partial<OrderedExcalidrawElement> = {}): OrderedExcalidrawElement {
  return { id, version, isDeleted: false, ...extra } as unknown as OrderedExcalidrawElement
}

function byId(elements: OrderedExcalidrawElement[]) {
  return new Map(elements.map((e) => [e.id, e]))
}

describe('mergeRestoredElements', () => {
  it('bumps a restored element past the current live version for a shared id', () => {
    const current = [el('a', 10)]
    const restored = [el('a', 2, { x: 5 } as Partial<OrderedExcalidrawElement>)]

    const merged = mergeRestoredElements(current, restored)
    expect(merged).toHaveLength(1)
    const a = byId(merged).get('a')!
    expect(a.version).toBeGreaterThan(10)
    expect((a as unknown as { x: number }).x).toBe(5)
  })

  it('passes through a restored element with no live counterpart as-is', () => {
    const current: OrderedExcalidrawElement[] = []
    const restored = [el('new', 1)]

    const merged = mergeRestoredElements(current, restored)
    expect(merged).toHaveLength(1)
    expect(merged[0].id).toBe('new')
    expect(merged[0].version).toBe(1)
  })

  it('tombstones a live element absent from the restored version', () => {
    const current = [el('gone', 3)]
    const restored: OrderedExcalidrawElement[] = []

    const merged = mergeRestoredElements(current, restored)
    expect(merged).toHaveLength(1)
    const gone = byId(merged).get('gone')!
    expect(gone.isDeleted).toBe(true)
    expect(gone.version).toBeGreaterThan(3)
  })

  it('leaves an already-deleted live element untouched when absent from the restored version', () => {
    const current = [el('gone', 3, { isDeleted: true })]
    const restored: OrderedExcalidrawElement[] = []

    const merged = mergeRestoredElements(current, restored)
    expect(merged).toEqual([current[0]])
  })

  it('undeletes a live tombstone when the restored version has it present', () => {
    const current = [el('a', 5, { isDeleted: true })]
    const restored = [el('a', 1, { isDeleted: false })]

    const merged = mergeRestoredElements(current, restored)
    const a = byId(merged).get('a')!
    expect(a.isDeleted).toBe(false)
    expect(a.version).toBeGreaterThan(5)
  })

  it('produces the full union of ids exactly once each', () => {
    const current = [el('shared', 1), el('live-only', 1)]
    const restored = [el('shared', 1), el('restored-only', 1)]

    const merged = mergeRestoredElements(current, restored)
    expect(merged.map((e) => e.id).sort()).toEqual(['live-only', 'restored-only', 'shared'])
  })
})
