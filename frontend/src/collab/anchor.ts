// The anchor is the one element a room's canvas centers on when it first
// loads (see CollaborativeCanvas.tsx). It's marked via customData rather than
// a fixed id because ids are regenerated if the element is ever deleted and
// redrawn, silently breaking an id-based reference.

import type { AppState, NormalizedZoomValue } from '@excalidraw/excalidraw/types'
import { newElementWith } from '@excalidraw/excalidraw'
import type { ExcalidrawElement, OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { ElementUpdate } from '@excalidraw/excalidraw/element/mutateElement'

export function isAnchor(element: ExcalidrawElement): boolean {
  return element.customData?.isAnchor === true
}

export function findAnchor<T extends ExcalidrawElement>(elements: readonly T[]): T | undefined {
  return elements.find(isAnchor)
}

/**
 * Returns `appState` with scrollX/scrollY overridden to center the anchor in
 * `elements` (zoom preserved), or `appState` unchanged if there's no anchor.
 * Meant for initialData, not a post-mount imperative scrollToContent call:
 * Excalidraw's own initialData application (initializeScene, deferred behind
 * an internal async language-pack load — see the comment in
 * CollaborativeCanvas.tsx) re-applies initialData.appState's scroll after
 * mount, silently reverting anything set imperatively in that window. Baking
 * the target scroll into the initial appState instead means there's nothing
 * left to race. Uses window dimensions as the viewport size since both
 * mount sites render Excalidraw as a full-viewport absolutely-positioned box.
 */
export function centerOnAnchor(
  elements: readonly ExcalidrawElement[],
  appState?: Partial<AppState>,
): Partial<AppState> | undefined {
  const anchor = findAnchor(elements)
  if (!anchor) return appState

  const zoom = appState?.zoom?.value ?? (1 as NormalizedZoomValue)
  return {
    ...appState,
    zoom: { value: zoom },
    scrollX: window.innerWidth / 2 / zoom - (anchor.x + anchor.width / 2),
    scrollY: window.innerHeight / 2 / zoom - (anchor.y + anchor.height / 2),
  }
}

function withAnchorFlag<T extends ExcalidrawElement>(element: T, value: boolean): T {
  const { isAnchor: _drop, ...rest } = element.customData ?? {}
  const customData = (value ? { ...rest, isAnchor: true } : rest) as T['customData']
  return newElementWith(element, { customData } as ElementUpdate<T>)
}

/**
 * Moves the anchor flag onto `targetId`, clearing it from wherever it was
 * before — or clears it entirely if `targetId` was already the anchor.
 */
export function toggleAnchor<T extends OrderedExcalidrawElement>(elements: readonly T[], targetId: string): T[] {
  const makeAnchor = !elements.some((el) => el.id === targetId && isAnchor(el))
  return elements.map((el) => {
    if (el.id === targetId) return isAnchor(el) === makeAnchor ? el : withAnchorFlag(el, makeAnchor)
    return isAnchor(el) ? withAnchorFlag(el, false) : el
  })
}
