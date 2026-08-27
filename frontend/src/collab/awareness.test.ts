import { describe, expect, it } from 'vitest'
import { buildCollaborators, type AwarenessState } from './awareness'

const localId = 1

describe('buildCollaborators', () => {
  it('excludes the local client', () => {
    const states = new Map<number, AwarenessState>([
      [localId, { user: { username: 'me' } }],
      [2, { user: { username: 'peer' } }],
    ])
    const result = buildCollaborators(states, localId)
    expect(result.size).toBe(1)
    expect(result.has('2' as never)).toBe(true)
    expect(result.has('1' as never)).toBe(false)
  })

  it('maps pointer, button, selection, and username', () => {
    const states = new Map<number, AwarenessState>([
      [
        2,
        {
          user: { username: 'peer' },
          pointer: { x: 10, y: 20, tool: 'laser' },
          button: 'down',
          selectedElementIds: { el1: true },
        },
      ],
    ])
    const c = buildCollaborators(states, localId).get('2' as never)!
    expect(c.pointer).toEqual({ x: 10, y: 20, tool: 'laser' })
    expect(c.button).toBe('down')
    expect(c.selectedElementIds).toEqual({ el1: true })
    expect(c.username).toBe('peer')
    expect(c.socketId).toBe('2')
  })

  it('uses a null username when no user is present', () => {
    const states = new Map<number, AwarenessState>([[2, { pointer: { x: 0, y: 0, tool: 'pointer' } }]])
    const c = buildCollaborators(states, localId).get('2' as never)!
    expect(c.username).toBeNull()
  })
})
