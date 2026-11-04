import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, type User } from '../api'

export interface Meta {
  name: string
  version: string
  terminal: boolean
  sftp: boolean
  fsscope: string[]
}

interface AuthValue {
  ready: boolean
  user: User | null
  meta: Meta | null
  needsSetup: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  reload: () => Promise<void>
  completeSetup: () => void
}

const AuthContext = createContext<AuthValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [meta, setMeta] = useState<Meta | null>(null)
  const [needsSetup, setNeedsSetup] = useState(false)

  const reload = useCallback(async () => {
    try {
      const [m, s] = await Promise.all([api.meta(), api.setupStatus()])
      setMeta(m as Meta)
      setNeedsSetup(s.needs_setup)
      if (!s.needs_setup) {
        try {
          setUser(await api.me())
        } catch {
          setUser(null)
        }
      } else {
        setUser(null)
      }
    } catch {
      setUser(null)
    } finally {
      setReady(true)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login(username, password)
    setUser(res.user)
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } finally {
      setUser(null)
    }
  }, [])

  const value = useMemo<AuthValue>(
    () => ({
      ready,
      user,
      meta,
      needsSetup,
      login,
      logout,
      reload,
      completeSetup: () => setNeedsSetup(false),
    }),
    [ready, user, meta, needsSetup, login, logout, reload],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}

// useCanWrite is true for roles allowed to apply changes on remote hosts.
export function useCanWrite(): boolean {
  const { user } = useAuth()
  return user?.role === 'admin' || user?.role === 'operator'
}
