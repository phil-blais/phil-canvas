import { useEffect, useRef, useState, type ReactNode } from 'react'

function KebabIcon() {
  return (
    <svg viewBox="0 0 20 20" width="16" height="16" fill="currentColor" aria-hidden="true" focusable="false">
      <circle cx="10" cy="4" r="1.6" />
      <circle cx="10" cy="10" r="1.6" />
      <circle cx="10" cy="16" r="1.6" />
    </svg>
  )
}

/**
 * A small "more actions" dropdown anchored to its trigger button. Manages its
 * own open/closed state (click-outside and Escape both close it, mirroring
 * FlyoutPanel's pattern). Items are passed as a function of `close` so a
 * caller can choose, per item, whether clicking it should close the menu
 * (most actions) or keep it open for a multi-step flow (e.g. a delete
 * confirmation).
 */
export function ContextMenu({
  label = 'More actions',
  children,
  onClose,
}: {
  label?: string
  children: (close: () => void) => ReactNode
  onClose?: () => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const close = () => {
    setOpen(false)
    onClose?.()
  }

  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) close()
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
    // Deliberately re-runs only on open/close, not on every onClose identity
    // change — callers typically pass a fresh closure each render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  return (
    <div className="app-context-menu" ref={ref}>
      <button
        type="button"
        className="btn small secondary icon-btn"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <KebabIcon />
      </button>
      {open && (
        <div className="app-context-menu-list" role="menu">
          {children(close)}
        </div>
      )}
    </div>
  )
}
