import { useCallback, useRef } from 'react'

/**
 * Guards against an async response arriving after it's no longer the
 * authoritative one — because a newer request superseded it, or because the
 * caller has torn down. Call `next()` when starting an operation to get a
 * token; after any `await`, check `isCurrent(token)` before applying the
 * result. Calling `next()` again (e.g. in an effect's cleanup) invalidates
 * any token handed out so far.
 */
export function useLatestOnly() {
  const seqRef = useRef(0)
  const next = useCallback(() => ++seqRef.current, [])
  const isCurrent = useCallback((token: number) => token === seqRef.current, [])
  return { next, isCurrent }
}
