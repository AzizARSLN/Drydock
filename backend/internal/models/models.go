package models

import "time"

// Role values understood by the authorization layer.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// User is a Drydock operator.
type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	FullName     string     `json:"full_name"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
}

// Server is a managed Linux host reachable over SSH.
type Server struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Username    string     `json:"username"`
	AuthType    string     `json:"auth_type"` // "key" | "password"
	Tags        string     `json:"tags"`
	Description string     `json:"description"`
	// HostKeyFingerprint pins the SSH host key (trust on first use).
	HostKeyFingerprint string     `json:"host_key_fingerprint"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`

	// Secrets are never serialized to API clients.
	PrivateKeyEnc    string `json:"-"`
	PasswordEnc      string `json:"-"`
	PassphraseEnc    string `json:"-"`
	SudoPasswordEnc  string `json:"-"`
}

// ServerInput carries secret material for create/update operations.
type ServerInput struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	AuthType       string `json:"auth_type"`
	PrivateKey     string `json:"private_key"`
	Password       string `json:"password"`
	Passphrase     string `json:"passphrase"`
	SudoPassword   string `json:"sudo_password"`
	Tags           string `json:"tags"`
	Description    string `json:"description"`
	// ResetHostKey clears the pinned fingerprint so the next connection re-pins it.
	ResetHostKey bool `json:"reset_host_key"`
}

// Snapshot stores the previous content of a file so changes can be rolled back.
type Snapshot struct {
	ID        int64     `json:"id"`
	ServerID  int64     `json:"server_id"`
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	Content   string    `json:"content,omitempty"`
	Note      string    `json:"note"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEntry records a single action performed through Drydock.
type AuditEntry struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Username   string    `json:"username"`
	ServerID   int64     `json:"server_id"`
	ServerName string    `json:"server_name"`
	Action     string    `json:"action"`
	Target     string    `json:"target"`
	Detail     string    `json:"detail"`
	Status     string    `json:"status"` // success | error | dry-run
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}
