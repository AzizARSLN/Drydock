import { useCallback, useEffect, useRef, useState } from 'react'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { terminalURL } from '../../api'
import { Alert, Badge } from '../../components/ui'
import { useI18n } from '../../i18n'
import type { TabProps } from '../ServerDetail'

interface Frame {
  t: 'i' | 'r' | 'o' | 'e' | 'c'
  d?: string
  c?: number
  r?: number
  m?: string
}

export default function TerminalTab({ serverId }: TabProps) {
  const { t } = useI18n()
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')

  const teardown = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null
    termRef.current?.dispose()
    termRef.current = null
    fitRef.current = null
    setConnected(false)
  }, [])

  useEffect(() => teardown, [teardown])

  const fit = () => {
    try {
      fitRef.current?.fit()
    } catch {
      /* the host may be momentarily detached during tab switches */
    }
  }

  const connect = () => {
    if (!hostRef.current || termRef.current) return
    setError('')

    const term = new Terminal({
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      cursorBlink: true,
      theme: { background: '#05080c', foreground: '#e6edf3' },
    })
    const addon = new FitAddon()
    term.loadAddon(addon)
    term.open(hostRef.current)
    termRef.current = term
    fitRef.current = addon
    fit()

    const ws = new WebSocket(terminalURL(serverId))
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      ws.send(JSON.stringify({ t: 'r', c: term.cols, r: term.rows } satisfies Frame))
    }
    ws.onmessage = (ev) => {
      let frame: Frame
      try {
        frame = JSON.parse(ev.data as string) as Frame
      } catch {
        return
      }
      if (frame.t === 'o' && frame.d) term.write(frame.d)
      else if (frame.t === 'e') term.write(`\r\n\x1b[31m${frame.m ?? ''}\x1b[0m\r\n`)
      else if (frame.t === 'c') {
        term.write(`\r\n\x1b[33m${t('term.disconnected')}\x1b[0m\r\n`)
        setConnected(false)
      }
    }
    ws.onerror = () => setError(t('err.network'))
    ws.onclose = () => setConnected(false)

    term.onData((d) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ t: 'i', d } satisfies Frame))
    })
    term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ t: 'r', c: cols, r: rows } satisfies Frame))
    })

    term.focus()
  }

  useEffect(() => {
    const onResize = () => fit()
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  return (
    <>
      <div className="toolbar mb-12">
        <Badge kind={connected ? 'ok' : ''}>
          <span className="dot" /> {connected ? t('term.connect') : t('term.hint')}
        </Badge>
        <div className="spacer" />
        {termRef.current ? (
          <button className="btn btn-sm" onClick={teardown}>
            {t('term.disconnect')}
          </button>
        ) : (
          <button className="btn btn-sm btn-primary" onClick={connect}>
            {t('term.connect')}
          </button>
        )}
      </div>

      {error ? <Alert kind="err">{error}</Alert> : null}

      <div className="term-wrap" style={{ height: 'calc(100vh - 300px)', minHeight: 420 }}>
        <div ref={hostRef} className="term-host" />
      </div>
    </>
  )
}
