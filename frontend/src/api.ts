// Typed client for the Drydock HTTP API. All requests carry the session cookie.

export interface User {
  id: number
  username: string
  role: 'admin' | 'operator' | 'viewer'
  full_name: string
  created_at: string
  last_login?: string
}

export interface Server {
  id: number
  name: string
  host: string
  port: number
  username: string
  auth_type: 'key' | 'password'
  tags: string
  description: string
  host_key_fingerprint: string
  created_at: string
  updated_at?: string
}

export interface ServerInput {
  name: string
  host: string
  port: number
  username: string
  auth_type: 'key' | 'password'
  private_key?: string
  password?: string
  passphrase?: string
  sudo_password?: string
  tags?: string
  description?: string
  reset_host_key?: boolean
}

export interface Outcome {
  dry_run: boolean
  commands?: string[]
  message: string
  diff?: string
  output?: string
  backup?: string
}

export interface NginxSite {
  name: string
  path: string
  enabled: boolean
  server_name: string
  listen: string[]
  ssl: boolean
  cert_path: string
}

export interface NginxOverview {
  installed: boolean
  version: string
  state: string
  sites: NginxSite[]
}

export interface CertInfo {
  domain: string
  path: string
  issuer: string
  not_before: string
  not_after: string
  days_left: number
  auto_renew: boolean
}

export interface Service {
  name: string
  kind: 'docker' | 'systemd'
  status: string
  image: string
  version: string
  uptime: string
  detail: string
}

export interface ListeningPort {
  proto: string
  address: string
  port: number
  process: string
}

export interface FileEntry {
  name: string
  path: string
  is_dir: boolean
  size: number
  mode: string
  mod_time: string
  owner: string
}

export interface HealthReport {
  server_id: number
  reachable: boolean
  hostname: string
  os: string
  kernel: string
  uptime: string
  load_avg: string
  cpu_percent: number
  mem_total_mb: number
  mem_used_mb: number
  mem_percent: number
  disks: { mount: string; device: string; total_gb: number; used_gb: number; used_pct: number }[]
  nginx_state: string
  docker_on: boolean
  failed_units: string[]
  cert_expiring: CertInfo[]
  checked_at: string
  error?: string
}

export interface Snapshot {
  id: number
  server_id: number
  kind: string
  path: string
  note: string
  created_by: string
  created_at: string
}

export interface AuditEntry {
  id: number
  user_id: number
  username: string
  server_id: number
  server_name: string
  action: string
  target: string
  detail: string
  status: string
  ip: string
  created_at: string
}

export class ApiError extends Error {
  status: number
  outcome?: Outcome
  constructor(status: number, message: string, outcome?: Outcome) {
    super(message)
    this.status = status
    this.outcome = outcome
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      credentials: 'include',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError(0, 'err.network')
  }
  const text = await res.text()
  let data: any = undefined
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { raw: text }
    }
  }
  if (!res.ok) {
    const msg = data?.error || `HTTP ${res.status}`
    throw new ApiError(res.status, msg, data?.outcome)
  }
  return data as T
}

const enc = encodeURIComponent

export const api = {
  meta: () => request<any>('GET', '/api/meta'),
  setupStatus: () => request<{ needs_setup: boolean }>('GET', '/api/setup/status'),
  setup: (username: string, password: string, full_name: string) =>
    request<User>('POST', '/api/setup', { username, password, full_name }),
  login: (username: string, password: string) =>
    request<{ user: User }>('POST', '/api/auth/login', { username, password }),
  logout: () => request<{ ok: boolean }>('POST', '/api/auth/logout'),
  me: () => request<User>('GET', '/api/auth/me'),
  changePassword: (current: string, next: string) =>
    request<{ ok: boolean }>('POST', '/api/auth/password', { current, new: next }),

  servers: () => request<Server[]>('GET', '/api/servers'),
  server: (id: number) => request<Server>('GET', `/api/servers/${id}`),
  createServer: (inp: ServerInput) => request<Server>('POST', '/api/servers', inp),
  updateServer: (id: number, inp: ServerInput) => request<Server>('PUT', `/api/servers/${id}`, inp),
  deleteServer: (id: number) => request<{ ok: boolean }>('DELETE', `/api/servers/${id}`),
  testServer: (id: number) =>
    request<{ ok: boolean; fingerprint: string; output: string }>('POST', `/api/servers/${id}/test`),
  health: (id: number) => request<HealthReport>('GET', `/api/servers/${id}/health`),

  nginx: (id: number) => request<NginxOverview>('GET', `/api/servers/${id}/nginx`),
  nginxConfig: (id: number, path: string) =>
    request<{ path: string; content: string }>('GET', `/api/servers/${id}/nginx/config?path=${enc(path)}`),
  nginxSave: (id: number, path: string, content: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/nginx/config`, { path, content, dry_run }),
  nginxEnable: (id: number, name: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/nginx/enable`, { name, dry_run }),
  nginxDisable: (id: number, name: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/nginx/disable`, { name, dry_run }),
  nginxTest: (id: number) =>
    request<{ ok: boolean; output: string }>('POST', `/api/servers/${id}/nginx/test`),
  nginxReload: (id: number, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/nginx/reload`, { dry_run }),

  certs: (id: number) => request<CertInfo[]>('GET', `/api/servers/${id}/certs`),
  certRenew: (id: number, domain: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/certs/renew`, { domain, dry_run }),

  services: (id: number, kind: string, filter: string) =>
    request<Service[]>('GET', `/api/servers/${id}/services?kind=${enc(kind)}&filter=${enc(filter)}`),
  serviceAction: (id: number, kind: string, name: string, action: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/services/action`, { kind, name, action, dry_run }),
  serviceUpdate: (id: number, kind: string, name: string, image: string, compose_file: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/services/update`, { kind, name, image, compose_file, dry_run }),

  ports: (id: number) => request<ListeningPort[]>('GET', `/api/servers/${id}/ports`),
  portChange: (id: number, path: string, old_port: number, new_port: number, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/ports/change`, { path, old_port, new_port, dry_run }),

  files: (id: number, path: string) =>
    request<{ path: string; entries: FileEntry[] }>('GET', `/api/servers/${id}/files?path=${enc(path)}`),
  fileContent: (id: number, path: string) =>
    request<{ path: string; content: string }>('GET', `/api/servers/${id}/files/content?path=${enc(path)}`),
  fileSave: (id: number, path: string, content: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/files/content`, { path, content, dry_run }),
  fileMkdir: (id: number, path: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/files/mkdir`, { path, dry_run }),
  fileMove: (id: number, from: string, to: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/files/move`, { from, to, dry_run }),
  fileDelete: (id: number, path: string, recursive: boolean, dry_run: boolean, confirm: string) =>
    request<Outcome>('POST', `/api/servers/${id}/files/delete`, { path, recursive, dry_run, confirm }),
  fileDownload: (id: number, path: string) =>
    request<{ path: string; content_base64: string }>('GET', `/api/servers/${id}/files/download?path=${enc(path)}`),
  fileUpload: (id: number, path: string, content_base64: string, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/files/upload`, { path, content_base64, dry_run }),

  snapshots: (id: number, path = '') =>
    request<Snapshot[]>('GET', `/api/servers/${id}/snapshots?path=${enc(path)}`),
  snapshotRestore: (id: number, snapshot_id: number, dry_run: boolean) =>
    request<Outcome>('POST', `/api/servers/${id}/snapshots/restore`, { snapshot_id, dry_run }),

  audit: (limit = 200, serverId = 0) =>
    request<AuditEntry[]>('GET', `/api/audit?limit=${limit}&server_id=${serverId}`),

  users: () => request<User[]>('GET', '/api/users'),
  createUser: (username: string, password: string, role: string, full_name: string) =>
    request<User>('POST', '/api/users', { username, password, role, full_name }),
  updateUser: (id: number, role: string, full_name: string, password: string) =>
    request<{ ok: boolean }>('PUT', `/api/users/${id}`, { role, full_name, password }),
  deleteUser: (id: number) => request<{ ok: boolean }>('DELETE', `/api/users/${id}`),
}

export function terminalURL(serverId: number): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/api/servers/${serverId}/terminal`
}
