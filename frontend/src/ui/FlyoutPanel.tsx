import { useEffect, useRef, useState, type ReactNode } from 'react'
import './panel.css'

function MenuIcon() {
  return (
    <svg viewBox="0 0 20 20" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true" focusable="false">
      <path d="M3 5h14M3 10h14M3 15h14" />
    </svg>
  )
}

function CloseIcon() {
  return (
    <svg viewBox="0 0 20 20" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true" focusable="false">
      <path d="M5 5l10 10M15 5 5 15" />
    </svg>
  )
}

/** Always-visible collapsible panel anchored bottom-right (the app's control surface). */
export function FlyoutPanel({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  return (
    <div className="flyout" ref={ref}>
      {open && <div className="flyout-body">{children}</div>}
      <button
        className="flyout-toggle"
        onClick={() => setOpen((o) => !o)}
        aria-label={open ? 'Collapse panel' : 'Open panel'}
      >
        {open ? <CloseIcon /> : <MenuIcon />}
      </button>
    </div>
  )
}
