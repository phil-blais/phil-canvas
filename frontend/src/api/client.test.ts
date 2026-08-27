import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api } from './client'

function mockFetch(status: number, body: unknown) {
  const text = body === undefined ? '' : JSON.stringify(body)
  const fn = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: `status ${status}`,
    text: () => Promise.resolve(text),
  } as Response)
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('api client', () => {
  it('adminLogin posts the id token and returns the Go JWT', async () => {
    const fetchMock = mockFetch(200, { token: 'go-jwt' })
    const result = await api.adminLogin('firebase-id-token')

    expect(result).toEqual({ token: 'go-jwt' })
    const [path, init] = fetchMock.mock.calls[0]
    expect(path).toBe('/auth/admin')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual({ idToken: 'firebase-id-token' })
  })

  it('throws ApiError with the server message on non-2xx', async () => {
    mockFetch(403, { error: 'not authorized as admin' })
    await expect(api.adminLogin('x')).rejects.toMatchObject({
      status: 403,
      message: 'not authorized as admin',
    })
    await expect(api.adminLogin('x')).rejects.toBeInstanceOf(ApiError)
  })

  it('guestLogin sends room and code', async () => {
    const fetchMock = mockFetch(200, { token: 'guest-jwt', room: 'r1' })
    await api.guestLogin('r1', 'WXYZ')
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ room: 'r1', code: 'WXYZ' })
  })

  it('listRooms GETs the public list', async () => {
    mockFetch(200, [{ id: 'r1', name: 'Untitled', participantCount: 2, sceneId: 's1' }])
    const rooms = await api.listRooms()
    expect(rooms).toHaveLength(1)
    expect(rooms[0].name).toBe('Untitled')
    expect(rooms[0].sceneId).toBe('s1')
  })

  it('createRoom attaches the bearer token and defaults the name', async () => {
    const fetchMock = mockFetch(201, { id: 'r1', name: 'Untitled', code: 'WXYZ' })
    await api.createRoom('admin-jwt')
    const init = fetchMock.mock.calls[0][1]
    expect(init.headers.Authorization).toBe('Bearer admin-jwt')
    expect(JSON.parse(init.body)).toEqual({ name: 'Untitled' })
  })

  it('createRoom sends an explicit name', async () => {
    const fetchMock = mockFetch(201, { id: 'r1', name: 'My Doc', code: 'WXYZ' })
    await api.createRoom('admin-jwt', 'My Doc')
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ name: 'My Doc' })
  })

  it('closeRoom handles an empty 204 body', async () => {
    mockFetch(204, undefined)
    await expect(api.closeRoom('admin-jwt', 'r1')).resolves.toBeUndefined()
  })

  it('renameRoom PATCHes the room name', async () => {
    const fetchMock = mockFetch(200, { id: 'r1', name: 'Renamed' })
    const result = await api.renameRoom('admin-jwt', 'r1', 'Renamed')
    expect(result).toEqual({ id: 'r1', name: 'Renamed' })
    const [path, init] = fetchMock.mock.calls[0]
    expect(path).toBe('/rooms/r1/name')
    expect(init.method).toBe('PATCH')
    expect(JSON.parse(init.body)).toEqual({ name: 'Renamed' })
  })

  it('saveScene posts the payload and returns ids', async () => {
    const fetchMock = mockFetch(200, { sceneId: 's1', versionId: 'v1' })
    const result = await api.saveScene('admin-jwt', 'r1', {
      elements: [{ id: 'a' }],
      appState: { zoom: 1 },
      files: {},
    })
    expect(result).toEqual({ sceneId: 's1', versionId: 'v1' })
    const [path, init] = fetchMock.mock.calls[0]
    expect(path).toBe('/rooms/r1/save')
    expect(init.headers.Authorization).toBe('Bearer admin-jwt')
  })

  it('publishScene posts to the publish endpoint', async () => {
    const fetchMock = mockFetch(200, { published: true })
    await api.publishScene('admin-jwt', 'r1', { elements: [], appState: {}, files: {} })
    expect(fetchMock.mock.calls[0][0]).toBe('/rooms/r1/publish')
  })

  it('openScene posts to the scene open endpoint', async () => {
    const fetchMock = mockFetch(200, { id: 'r1', name: 'Canvas', code: 'WXYZ', sceneId: 's1', live: false })
    const result = await api.openScene('admin-jwt', 's1', { name: 'Canvas' })
    expect(result).toEqual({ id: 'r1', name: 'Canvas', code: 'WXYZ', sceneId: 's1', live: false })
    const [path, init] = fetchMock.mock.calls[0]
    expect(path).toBe('/scenes/s1/open')
    expect(JSON.parse(init.body)).toEqual({ name: 'Canvas' })
  })

  it('renameScene PATCHes the scene name', async () => {
    const fetchMock = mockFetch(200, { id: 's1', name: 'Renamed' })
    const result = await api.renameScene('admin-jwt', 's1', 'Renamed')
    expect(result).toEqual({ id: 's1', name: 'Renamed' })
    const [path, init] = fetchMock.mock.calls[0]
    expect(path).toBe('/scenes/s1')
    expect(init.method).toBe('PATCH')
  })

  it('deleteScene DELETEs the scene', async () => {
    mockFetch(204, undefined)
    await expect(api.deleteScene('admin-jwt', 's1')).resolves.toBeUndefined()
  })

  it('getVersion fetches a specific version', async () => {
    const fetchMock = mockFetch(200, { elements: [{ id: 'a' }], appState: {}, fileIds: ['f1'] })
    const data = await api.getVersion('admin-jwt', 's1', 'v1')
    expect(fetchMock.mock.calls[0][0]).toBe('/scenes/s1/versions/v1')
    expect(data.fileIds).toEqual(['f1'])
  })
})
