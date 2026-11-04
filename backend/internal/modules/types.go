package modules

// Outcome is the uniform result returned by every mutating operation so the UI
// can render a consistent dry-run / applied view.
type Outcome struct {
	DryRun   bool     `json:"dry_run"`
	Commands []string `json:"commands"`
	Message  string   `json:"message"`
	Diff     string   `json:"diff,omitempty"`
	Output   string   `json:"output,omitempty"`
	Backup   string   `json:"backup,omitempty"`
}

// Job is a long running unit of work reported to the UI as it progresses.
type Job struct {
	ID       string   `json:"id"`
	ServerID int64    `json:"server_id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"` // pending | running | done | error
	Done     int      `json:"done"`
	Total    int      `json:"total"`
	Log      []string `json:"log"`
	Error    string   `json:"error,omitempty"`
}

// NginxSite describes a single served site configuration.
type NginxSite struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Enabled  bool   `json:"enabled"`
	ServerName string `json:"server_name"`
	Listen   []string `json:"listen"`
	SSL      bool   `json:"ssl"`
	CertPath string `json:"cert_path"`
}

// Service is a managed process exposed by docker or systemd.
type Service struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"` // docker | systemd
	Status  string `json:"status"`
	Image   string `json:"image"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
	Detail  string `json:"detail"`
}

// CertInfo describes an X.509 certificate found on the host.
type CertInfo struct {
	Domain    string `json:"domain"`
	Path      string `json:"path"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	DaysLeft  int    `json:"days_left"`
	AutoRenew bool   `json:"auto_renew"`
}

// DiskMount is a filesystem usage record.
type DiskMount struct {
	Mount   string  `json:"mount"`
	Device  string  `json:"device"`
	TotalGB float64 `json:"total_gb"`
	UsedGB  float64 `json:"used_gb"`
	UsedPct float64 `json:"used_pct"`
}

// HealthReport is the aggregated state of a managed host.
type HealthReport struct {
	ServerID   int64       `json:"server_id"`
	Reachable  bool        `json:"reachable"`
	Hostname   string      `json:"hostname"`
	OS         string      `json:"os"`
	Kernel     string      `json:"kernel"`
	Uptime     string      `json:"uptime"`
	LoadAvg    string      `json:"load_avg"`
	CPUPercent float64     `json:"cpu_percent"`
	MemTotalMB float64     `json:"mem_total_mb"`
	MemUsedMB  float64     `json:"mem_used_mb"`
	MemPercent float64     `json:"mem_percent"`
	Disks      []DiskMount `json:"disks"`
	NginxState string      `json:"nginx_state"`
	DockerOn   bool        `json:"docker_on"`
	FailedUnits []string   `json:"failed_units"`
	CertExpiring []CertInfo `json:"cert_expiring"`
	CheckedAt  string      `json:"checked_at"`
	Error      string      `json:"error,omitempty"`
}

// ListeningPort is a socket bound on the host.
type ListeningPort struct {
	Proto   string `json:"proto"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Process string `json:"process"`
}

// FileEntry is a directory listing item.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	Owner   string `json:"owner"`
}
