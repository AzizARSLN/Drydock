import type { ReactNode } from 'react'
import { useI18n } from '../i18n'

export function Spinner() {
  return <span className="spinner" />
}

export function Loading({ label }: { label?: string }) {
  const { t } = useI18n()
  return (
    <div className="center">
      <div className="flex">
        <Spinner />
        <span className="muted">{label ?? t('common.loading')}</span>
      </div>
    </div>
  )
}

export function Empty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="empty">
      <strong>{title}</strong>
      {hint ? <span>{hint}</span> : null}
    </div>
  )
}

export function PageHead({ title, subtitle, children }: { title: string; subtitle?: string; children?: ReactNode }) {
  return (
    <div className="page-head">
      <div>
        <h1>{title}</h1>
        {subtitle ? <p>{subtitle}</p> : null}
      </div>
      <div className="spacer" />
      {children}
    </div>
  )
}

export function Badge({ kind = '', children }: { kind?: '' | 'ok' | 'warn' | 'danger' | 'info'; children: ReactNode }) {
  return <span className={`badge ${kind}`}>{children}</span>
}

export function Alert({ kind = 'info', children }: { kind?: 'ok' | 'err' | 'info' | 'warn'; children: ReactNode }) {
  return <div className={`alert ${kind}`}>{children}</div>
}

export function KV({ k, v }: { k: string; v: ReactNode }) {
  return (
    <div className="kv-row">
      <span className="k">{k}</span>
      <span className="v">{v}</span>
    </div>
  )
}

export function fmtBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i += 1
  }
  return `${i === 0 ? v : v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`
}

export function fmtDate(s?: string): string {
  if (!s) return '—'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleString()
}

export function levelClass(pct: number): string {
  if (pct >= 90) return 'danger'
  if (pct >= 75) return 'warn'
  return ''
}
