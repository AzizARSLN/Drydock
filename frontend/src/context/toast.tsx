import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'

type Kind = 'ok' | 'err' | 'info' | 'warn'

interface Item {
  id: number
  kind: Kind
  text: string
}

interface ToastValue {
  push: (text: string, kind?: Kind) => void
  ok: (text: string) => void
  err: (text: string) => void
  warn: (text: string) => void
}

const ToastContext = createContext<ToastValue | null>(null)

// ToastProvider renders transient notifications in a fixed stack. Messages
// auto-dismiss; the queue is intentionally small so it never blocks the UI.
export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Item[]>([])
  const seq = useRef(0)

  const push = useCallback((text: string, kind: Kind = 'info') => {
    const id = ++seq.current
    setItems((prev) => [...prev, { id, kind, text }])
    window.setTimeout(() => {
      setItems((prev) => prev.filter((t) => t.id !== id))
    }, 5200)
  }, [])

  const value = useMemo<ToastValue>(
    () => ({
      push,
      ok: (t) => push(t, 'ok'),
      err: (t) => push(t, 'err'),
      warn: (t) => push(t, 'warn'),
    }),
    [push],
  )

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toasts">
        {items.map((t) => (
          <div key={t.id} className={`toast ${t.kind}`}>
            <span className="dot" />
            <span>{t.text}</span>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside ToastProvider')
  return ctx
}
