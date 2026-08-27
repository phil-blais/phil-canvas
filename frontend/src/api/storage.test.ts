import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchPublishedScene, fetchSceneFiles, publishedConfigured } from './storage'

const BASE = 'https://firebasestorage.googleapis.com/v0/b/demo/o'

function stubBase(value?: string) {
  vi.stubEnv('VITE_PUBLISHED_BASE_URL', value ?? '')
}

function stubFetch(impl: (url: string) => { ok: boolean; json?: unknown }) {
  const fn = vi.fn((url: string) => {
    const r = impl(url)
    return Promise.resolve({ ok: r.ok, json: () => Promise.resolve(r.json) } as Response)
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
})

describe('published storage', () => {
  it('reports not configured when no base URL', () => {
    stubBase('')
    expect(publishedConfigured()).toBe(false)
  })

  it('returns null published scene when unconfigured (no fetch)', async () => {
    stubBase('')
    const fetchMock = stubFetch(() => ({ ok: true, json: {} }))
    expect(await fetchPublishedScene()).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('fetches and parses the published scene with an encoded path', async () => {
    stubBase(BASE)
    const fetchMock = stubFetch(() => ({ ok: true, json: { elements: [{ id: 'a' }], appState: {}, fileIds: ['f1'] } }))
    const scene = await fetchPublishedScene()
    expect(scene?.fileIds).toEqual(['f1'])
    expect(fetchMock.mock.calls[0][0]).toBe(`${BASE}/${encodeURIComponent('public/published-scene.json')}?alt=media`)
  })

  it('returns null when the published scene is missing', async () => {
    stubBase(BASE)
    stubFetch(() => ({ ok: false }))
    expect(await fetchPublishedScene()).toBeNull()
  })

  it('loads files best-effort, skipping failures', async () => {
    stubBase(BASE)
    stubFetch((url) => {
      if (url.includes('good')) return { ok: true, json: { id: 'good', dataURL: 'X' } }
      return { ok: false }
    })
    const files = await fetchSceneFiles(['good', 'missing'])
    expect(Object.keys(files)).toEqual(['good'])
  })
})
