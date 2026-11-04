# Drydock

**A self-hosted control plane for managing Linux servers over SSH.**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![SQLite](https://img.shields.io/badge/SQLite-WAL-003B57?logo=sqlite&logoColor=white)](https://sqlite.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

Drydock is a single, self-contained web application that connects to your Linux
hosts over SSH and lets you operate them from one place: edit and reload Nginx
configuration, issue and renew TLS certificates, start/stop/upgrade services,
move listening ports, browse files, open a live terminal, and keep a full audit
trail — all without installing an agent on the target machines.

Every change runs through a **dry-run → diff → approve → snapshot → apply →
verify → rollback** pipeline, so an operator always sees exactly what will
happen before it happens, and a broken change never survives.

---

## Why

Managing a handful of servers usually means juggling SSH sessions, remembering
which `nginx -t` flag matters, hoping a `docker compose pull` won't take a
service down, and keeping no record of who changed what. Drydock turns that
improvisation into a repeatable, reviewable workflow behind a single login:

- **No agent** — everything is plain SSH plus standard tools already on the box.
- **Preview before you commit** — every mutating action is simulated first.
- **Rollback built in** — the previous state is snapshotted before it is touched.
- **Accountable** — every action is attributed to a user and written to the audit log.

---

## Features

| Area | What you can do |
| --- | --- |
| **Nginx** | Browse sites, edit `server` blocks in a code editor, validate (`nginx -t`), enable/disable, reload |
| **TLS / SSL** | List certificates with issuer and expiry, flag certs expiring soon, renew via Certbot |
| **Applications** | Group containers by compose project folder, discover compose directories on disk, start / stop / restart / reload, update images, view `logs` / `ps`, and **deploy a new app** from a pasted `docker-compose.yml` |
| **Ports** | Inspect listening sockets, change a site's `listen` port and update the proxy targets |
| **Files** | Dual-pane explorer with an in-browser editor, upload, download, rename, mkdir, delete |
| **Terminal** | Full interactive `xterm.js` shell over a WebSocket-tunnelled SSH session |
| **Health** | Per-host CPU, memory, disk and load, plus a certificate-expiry watch list |
| **Backups** | Automatic snapshots of every file before it is modified, with one-click restore |
| **Access control** | Three roles (`admin` / `operator` / `viewer`) and a queryable audit log |

The interface ships in **English and Turkish** and switches at runtime.

---

## How it works

Drydock never changes a file in place without a preview:

```
  ┌─────────┐   ┌────────────┐   ┌───────────┐   ┌──────────┐   ┌──────────┐
  │ dry-run │──▶│ show diff  │──▶│  approve  │──▶│ snapshot │──▶│  apply   │
  └─────────┘   └────────────┘   └───────────┘   └──────────┘   └────┬─────┘
                                                                     │
                                              ┌──────────────────────▼───────┐
                                              │ verify (nginx -t / systemctl)│
                                              └───────────┬───────────┬──────┘
                                                       ok │      fail │
                                                          ▼           ▼
                                                     ┌────────┐  ┌──────────┐
                                                     │ commit │  │ rollback │
                                                     └────────┘  └──────────┘
```

- **Dry-run** executes the same remote commands with a non-mutating guard and
  returns the commands, their output, and a unified diff of the intended change.
- **Snapshot** copies the target file (e.g. the Nginx site) into the local
  SQLite store before writing, so a restore is always one click away.
- **Verify** re-runs the service's own validator; a failure triggers an
  automatic rollback to the snapshot.

---

## Architecture

```
                     ┌──────────────────────────────────────────────┐
   Browser  ────────▶│  Drydock (Go)                                │
   (React SPA)       │                                              │
        ▲            │  ┌──────────┐  ┌─────────┐  ┌─────────────┐  │
        │  HTTP/WSS  │  │ REST API │  │  RBAC   │  │  Audit log  │  │
        └────────────│  └────┬─────┘  └────┬────┘  └──────┬──────┘  │
                     │       │             │              │         │
                     │  ┌────▼─────────────▼──────────────▼──────┐  │
                     │  │            modules (nginx, ssl,        │  │
                     │  │        apps, ports, files, health)     │  │
                     │  └───────────────────┬────────────────────┘  │
                     │              ┌───────▼────────┐              │
                     │              │  SSH client    │  AES-256-GCM │
                     │              │  (x/crypto/ssh)│  encrypted   │
                     │              └───────┬────────┘  credentials │
                     └──────────────────────┼──────────────────────┘
                                            │ SSH (key or password)
                          ┌─────────────────┼─────────────────┐
                          ▼                 ▼                 ▼
                     ┌─────────┐       ┌─────────┐       ┌─────────┐
                     │ host A  │       │ host B  │       │ host C  │
                     │ nginx   │       │ docker  │       │ systemd │
                     └─────────┘       └─────────┘       └─────────┘
```

- **Backend:** Go, standard library HTTP (`net/http` `ServeMux` with method-based
  routing), `golang.org/x/crypto/ssh` for transport, `modernc.org/sqlite` for a
  pure-Go, CGO-free SQLite store in WAL mode.
- **Frontend:** React 18 + TypeScript on Vite, `react-router-dom` for routing,
  `@xterm/xterm` for the terminal. The production build is embedded and served
  by the Go binary — the whole product is a single executable.

---

## Security model

| Concern | Approach |
| --- | --- |
| Stored SSH secrets | Encrypted at rest with **AES-256-GCM**; the key is derived from `DRYDOCK_MASTER_KEY` |
| Host identity | **Trust-on-first-use** — the host key fingerprint is pinned on first connect and can be reset explicitly |
| Passwords | Hashed with **bcrypt** |
| Sessions | **JWT** in an `HttpOnly`, `SameSite=Lax` cookie (or `Authorization: Bearer` for API clients) |
| Authorization | Role hierarchy `viewer < operator < admin`, enforced server-side on every route |
| Path safety | File and config operations are confined to an allow-list of roots (`/etc`, `/opt`, `/srv`, `/var/www`, …) and reject traversal |
| Destructive actions | Deletes require typing the target's name; the terminal and SFTP can be disabled entirely |
| Accountability | Every action (including dry-runs) is written to an append-only audit log with actor, host and outcome |

> Drydock holds SSH credentials for your fleet, so treat it as a tier-0 system:
> run it behind a reverse proxy with TLS, bind it to loopback or a private
> network, and never expose it directly to the public internet.

---

## Quick start (Docker)

```bash
git clone https://github.com/AzizARSLN/Drydock.git
cd Drydock
cp .env.example .env
# Edit .env and set strong random values:
#   openssl rand -hex 32   → DRYDOCK_MASTER_KEY
#   openssl rand -hex 32   → DRYDOCK_JWT_SECRET
docker compose up -d --build
```

Open <http://127.0.0.1:8788/> and complete the one-time setup to create the
first administrator, then add your first server.

## Quick start (from source)

**Requirements:** Go 1.26+, Node 20+.

```bash
# 1. Build the web UI → emits into backend/web
cd frontend && npm install && npm run build && cd ..

# 2. Configure
cp .env.example .env      # then edit the two secrets

# 3. Run
cd backend && go run .
```

For frontend work, run the two processes side by side — Vite proxies `/api`
(including the terminal WebSocket) to the backend:

```bash
cd backend  && go run .          # http://127.0.0.1:8788
cd frontend && npm run dev        # http://127.0.0.1:5173  (hot reload)
```

---

## Configuration

All settings are read from the environment, optionally preloaded from a local
`.env` file (real environment variables always win).

| Variable | Default | Description |
| --- | --- | --- |
| `DRYDOCK_ADDR` | `127.0.0.1:8788` | Listen address for the API and web UI |
| `DRYDOCK_DATA_DIR` | `./data` | Directory for the SQLite database |
| `DRYDOCK_DB` | `<data>/drydock.db` | Explicit path to the database file |
| `DRYDOCK_WEB_DIR` | `./web` | Directory containing the built web UI |
| `DRYDOCK_MASTER_KEY` | — (**required**, ≥ 32 chars) | Key material for encrypting stored SSH secrets |
| `DRYDOCK_JWT_SECRET` | — (**required**) | Signing secret for session tokens |
| `DRYDOCK_TOKEN_TTL_HOURS` | `12` | Session lifetime |
| `DRYDOCK_CORS_ORIGINS` | `http://127.0.0.1:5173,http://localhost:5173` | Allowed browser origins |
| `DRYDOCK_ENABLE_TERMINAL` | `true` | Enable the web terminal |
| `DRYDOCK_ENABLE_SFTP` | `true` | Enable the file manager |
| `DRYDOCK_HEALTH_INTERVAL` | `60` | Seconds between background health sweeps |
| `DRYDOCK_LOG_LEVEL` | `info` | Log verbosity |

---

## API reference

Every route is JSON over HTTP. Authentication is via the session cookie or a
`Bearer` token. `{id}` is the server's numeric ID.

| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/healthz` | — | Liveness probe |
| `GET` | `/api/meta` | — | Product name, version and feature flags |
| `GET` | `/api/setup/status` | — | Whether first-run setup is still required |
| `POST` | `/api/setup` | — | Create the initial administrator |
| `POST` | `/api/auth/login` | — | Start a session |
| `GET` | `/api/auth/me` | any | Current user |
| `POST` | `/api/auth/logout` | any | End the session |
| `POST` | `/api/auth/password` | any | Change own password |
| `GET` | `/api/servers` | any | List servers |
| `GET` | `/api/servers/{id}` | any | Get one server |
| `POST` | `/api/servers` | admin | Create a server |
| `PUT` | `/api/servers/{id}` | admin | Update a server |
| `DELETE` | `/api/servers/{id}` | admin | Delete a server |
| `POST` | `/api/servers/{id}/test` | operator | Test connectivity (host key, hostname, uname) |
| `GET` | `/api/servers/{id}/health` | any | CPU / memory / disk / load report |
| `GET` | `/api/servers/{id}/nginx` | any | Nginx sites and status |
| `GET` `POST` | `/api/servers/{id}/nginx/config` | any / operator | Read or write a site config |
| `POST` | `/api/servers/{id}/nginx/{enable,disable,test,reload}` | operator | Manage and validate Nginx |
| `GET` | `/api/servers/{id}/certs` | any | Certificates and expiry |
| `POST` | `/api/servers/{id}/certs/renew` | operator | Renew certificates |
| `GET` | `/api/servers/{id}/services` | any | Docker containers and systemd units |
| `POST` | `/api/servers/{id}/services/action` | operator | start / stop / restart / reload |
| `POST` | `/api/servers/{id}/services/update` | operator | Pull a new image / update a package |
| `GET` | `/api/servers/{id}/compose` | any | Compose project folders discovered on disk |
| `POST` | `/api/servers/{id}/services/compose` | operator | Run a project-level `docker compose` command (up / restart / down / logs / ps) |
| `POST` | `/api/servers/{id}/apps/deploy` | operator | Deploy a new app from a compose file (folder, compose, stack up, nginx vhost) |
| `GET` | `/api/servers/{id}/ports` | any | Listening sockets |
| `POST` | `/api/servers/{id}/ports/change` | operator | Change a site's listening port |
| `GET` | `/api/servers/{id}/files` | any | Directory listing |
| `GET` | `/api/servers/{id}/files/content` | any | Read a file |
| `GET` | `/api/servers/{id}/files/download` | any | Download a file |
| `POST` | `/api/servers/{id}/files/{content,mkdir,move,delete,upload}` | operator | Mutate the filesystem |
| `GET` | `/api/servers/{id}/snapshots` | any | List file snapshots |
| `POST` | `/api/servers/{id}/snapshots/restore` | operator | Restore a snapshot |
| `GET` | `/api/servers/{id}/terminal` | any | WebSocket terminal |
| `GET` | `/api/audit` | any | Audit log (filterable) |
| `GET` `POST` `PUT` `DELETE` | `/api/users[/{id}]` | admin | User management |

### Roles

| Capability | viewer | operator | admin |
| --- | :-: | :-: | :-: |
| View servers, configs, certs, logs, audit | ✅ | ✅ | ✅ |
| Nginx / TLS / services / ports / files changes | — | ✅ | ✅ |
| Manage servers and users | — | — | ✅ |

---

## Project layout

```
Drydock/
├── backend/
│   ├── main.go                 # entry point
│   ├── web/                    # embedded web UI output (git-ignored)
│   └── internal/
│       ├── api/                # HTTP routes, handlers, middleware
│       ├── app/                # dependency container & SSH connect helper
│       ├── audit/              # audit logger
│       ├── auth/               # JWT sessions
│       ├── config/             # env + .env loading
│       ├── crypto/             # AES-256-GCM credential cipher
│       ├── models/             # domain types
│       ├── modules/            # nginx, ssl, apps, ports, files, health
│       ├── sshx/               # SSH client, TOFU host-key pinning
│       └── store/              # SQLite persistence
├── frontend/
│   └── src/
│       ├── components/         # Layout, Modal, DryRun, ServerForm, ui
│       ├── context/            # auth + toast providers
│       ├── pages/              # Dashboard, ServerDetail + tabs, Users, Audit
│       ├── api.ts              # typed API client
│       └── i18n.tsx            # English / Turkish dictionary
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

## Roadmap

- [ ] Scheduled health checks with alerting (webhook / e-mail)
- [ ] Fine-grained, per-server permissions
- [ ] Firewall and fail2ban management
- [ ] Configuration drift detection across a fleet
- [ ] OIDC / LDAP single sign-on

## Contributing

Issues and pull requests are welcome. Please run `go vet ./...` and
`npm run build` before opening a PR, keep commits focused, and match the
existing code style.

---

## Türkçe Özet

**Drydock**, Linux sunucularını SSH üzerinden tek bir web arayüzünden yönetmek
için geliştirilmiş, kendi sunucunuzda barındırabileceğiniz (self-hosted) bir
kontrol panelidir. Hedef makinelere **hiçbir ajan kurulmaz**; tüm işlemler
standart SSH ve sunucuda zaten var olan araçlarla yapılır.

**Öne çıkanlar**

- **Nginx yönetimi:** site listeleme, yapılandırmayı düzenleme, `nginx -t` ile
  doğrulama, etkinleştirme/devre dışı bırakma ve yeniden yükleme.
- **HTTPS / SSL:** sertifika listesi, süre bitişi uyarıları ve Certbot ile yenileme.
- **Uygulama yaşam döngüsü:** Docker konteynerleri ve systemd servisleri için
  başlat / durdur / yeniden başlat / yeniden yükle ve imaj güncelleme.
- **Port yönetimi:** dinleyen portları görüntüleme ve bir sitenin `listen`
  portunu değiştirme.
- **Dosya yöneticisi:** iki panelli gezgin, tarayıcı içi editör, yükleme/indirme.
- **Web terminal:** `xterm.js` üzerinden WebSocket tünelli canlı SSH oturumu.
- **Sağlık izleme:** CPU, bellek, disk, load ve sertifika süresi takibi.
- **Yedekleme:** değiştirilen her dosya otomatik olarak saklanır, tek tıkla geri
  döndürülür.
- **Yetkilendirme:** `admin` / `operator` / `viewer` rolleri ve tam denetim kaydı
  (audit log).
- **Arayüz:** Türkçe ve İngilizce destekli, çalışma anında dil değiştirilebilir.

**Güvenlik yaklaşımı**

Her değişiklik **önce simüle edilir (dry-run), fark gösterilir, onay alınır,
ardından anlık görüntü alınarak uygulanır ve doğrulanır**; doğrulama başarısız
olursa otomatik olarak geri alınır. Saklanan SSH kimlik bilgileri
**AES-256-GCM** ile şifrelenir, parolalar **bcrypt** ile özetlenir, oturumlar
`HttpOnly` çerezle korunur ve sunucu anahtarları ilk bağlantıda sabitlenir
(trust-on-first-use).

**Hızlı başlangıç**

```bash
git clone https://github.com/AzizARSLN/Drydock.git
cd Drydock
cp .env.example .env        # DRYDOCK_MASTER_KEY ve DRYDOCK_JWT_SECRET'i düzenleyin
docker compose up -d --build
```

Ardından <http://127.0.0.1:8788/> adresini açıp ilk yönetici hesabını
oluşturun. Drydock'u internete doğrudan açmayın; TLS sonlandıran bir ters
vekil (reverse proxy) arkasında ve tercihen yalnızca yerel/özel ağda çalıştırın.

## Topics

`self-hosted` · `server-management` · `control-panel` · `devops` · `sysadmin` ·
`infrastructure` · `fleet-management` · `ssh` · `linux` · `nginx` ·
`reverse-proxy` · `docker` · `docker-compose` · `systemd` · `deploy` · `tls` ·
`ssl` · `certbot` · `golang` · `react` · `typescript` · `web-terminal` ·
`audit-log` · `agentless` · `dry-run` · `rollback`

## License

Released under the [MIT License](LICENSE).
