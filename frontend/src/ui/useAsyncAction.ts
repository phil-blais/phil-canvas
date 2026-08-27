import { useCallback, useState } from 'react'
import { ApiError } from '../api/client'

export type Run = (fn: () => Promise<void>, label?: string) => Promise<void>

/** Shared busy/error/status state machine for wrapping an async UI action. */
export function useAsyncAction() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<string | null>(null)

  const run = useCallback<Run>(async (fn, label) => {
    setError(null)
    setStatus(null)
    setBusy(true)
    try {
      await fn()
      if (label) setStatus(label)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong')
    } finally {
      setBusy(false)
    }
  }, [])

  // For failures that happen outside `run` (e.g. background polling), clearing
  // `status` too keeps it from lingering next to an unrelated fresh error.
  const reportError = useCallback((message: string) => {
    setStatus(null)
    setError(message)
  }, [])

  return { busy, error, status, run, reportError }
}
