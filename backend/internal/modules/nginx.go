package modules

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/sshx"
)

const (
	nginxSitesAvailable = "/etc/nginx/sites-available"
	nginxSitesEnabled   = "/etc/nginx/sites-enabled"
	nginxConfD          = "/etc/nginx/conf.d"
)

var (
	reServerName = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	reListen     = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
	reSSLcert    = regexp.MustCompile(`(?m)^\s*ssl_certificate\s+([^;]+);`)
)

// NginxInstalled reports whether nginx is present and returns its version banner.
func NginxInstalled(ctx context.Context, c *sshx.Client) (bool, string) {
	res, err := c.Run(ctx, "command -v nginx >/dev/null 2>&1 && nginx -v 2>&1 || echo NOT_INSTALLED")
	if err != nil {
		return false, ""
	}
	out := strings.TrimSpace(res.Stdout)
	if strings.Contains(out, "NOT_INSTALLED") || out == "" {
		return false, ""
	}
	return true, strings.TrimPrefix(out, "nginx version: ")
}

// NginxStatus returns the service state reported by systemd.
func NginxStatus(ctx context.Context, c *sshx.Client) string {
	res, err := c.Run(ctx, "systemctl is-active nginx 2>/dev/null || echo unknown")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(res.Stdout)
}

// ListNginxConfigs enumerates every site/config file Drydock can manage.
func ListNginxConfigs(ctx context.Context, c *sshx.Client) ([]NginxSite, error) {
	var sites []NginxSite

	available, err := listDirNames(ctx, c, nginxSitesAvailable)
	if err != nil {
		return nil, err
	}
	enabled, _ := listDirNames(ctx, c, nginxSitesEnabled)
	enabledSet := map[string]bool{}
	for _, e := range enabled {
		enabledSet[e] = true
	}

	// Read every available site body in a single round trip.
	if len(available) > 0 {
		bodies, err := readMany(ctx, c, prefixAll(nginxSitesAvailable, available))
		if err != nil {
			return nil, err
		}
		for _, name := range available {
			p := nginxSitesAvailable + "/" + name
			sites = append(sites, buildSite(name, p, enabledSet[name], bodies[p]))
		}
	}

	confFiles, _ := listDirNames(ctx, c, nginxConfD)
	if len(confFiles) > 0 {
		bodies, err := readMany(ctx, c, prefixAll(nginxConfD, confFiles))
		if err != nil {
			return nil, err
		}
		for _, name := range confFiles {
			if !strings.HasSuffix(name, ".conf") {
				continue
			}
			p := nginxConfD + "/" + name
			sites = append(sites, buildSite(name, p, true, bodies[p]))
		}
	}

	sort.Slice(sites, func(i, j int) bool { return sites[i].Name < sites[j].Name })
	return sites, nil
}

func buildSite(name, p string, enabled bool, content string) NginxSite {
	// Listen is initialized so the JSON payload always carries an array rather
	// than null, keeping the frontend contract stable for sites without a
	// listen directive.
	s := NginxSite{Name: name, Path: p, Enabled: enabled, Listen: []string{}}
	if m := reServerName.FindStringSubmatch(content); m != nil {
		s.ServerName = strings.Join(strings.Fields(m[1]), ", ")
	}
	if ms := reListen.FindAllStringSubmatch(content, -1); ms != nil {
		for _, m := range ms {
			s.Listen = append(s.Listen, strings.TrimSpace(m[1]))
		}
	}
	if m := reSSLcert.FindStringSubmatch(content); m != nil {
		s.SSL = true
		s.CertPath = strings.TrimSpace(m[1])
	}
	if !s.SSL && strings.Contains(content, "ssl") {
		s.SSL = true
	}
	return s
}

func prefixAll(dir string, names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, dir+"/"+n)
	}
	return out
}

func listDirNames(ctx context.Context, c *sshx.Client, dir string) ([]string, error) {
	res, err := c.RunSudo(ctx, "ls -1 "+shellQuote(dir)+" 2>/dev/null || true")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// readMany fetches several files in one round trip, keyed by absolute path.
func readMany(ctx context.Context, c *sshx.Client, paths []string) (map[string]string, error) {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = shellQuote(p)
	}
	cmd := fmt.Sprintf(`for f in %s; do echo "@@@DRYDOCK@@@$f"; cat "$f" 2>/dev/null; done`,
		strings.Join(quoted, " "))
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	var current string
	var buf strings.Builder
	flush := func() {
		if current != "" {
			out[current] = buf.String()
		}
		buf.Reset()
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.HasPrefix(line, "@@@DRYDOCK@@@") {
			flush()
			current = strings.TrimPrefix(line, "@@@DRYDOCK@@@")
			continue
		}
		if current != "" {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	flush()
	return out, nil
}

// ReadNginxConfig returns the raw body of a managed configuration file.
func ReadNginxConfig(ctx context.Context, c *sshx.Client, p string) (string, error) {
	clean, err := GuardPath(p, NginxScope)
	if err != nil {
		return "", err
	}
	return c.ReadFile(ctx, clean)
}

// SaveNginxConfig writes a configuration file with validate-and-rollback safety.
// When dryRun is true nothing is written; the caller receives the pending diff.
func SaveNginxConfig(ctx context.Context, a *app.App, c *sshx.Client, sv *models.Server,
	operator, filePath, content string, dryRun bool) (*Outcome, error) {

	clean, err := GuardPath(filePath, NginxScope)
	if err != nil {
		return nil, err
	}
	before, err := c.ReadFile(ctx, clean)
	if err != nil {
		return nil, err
	}

	out := &Outcome{
		DryRun:   dryRun,
		Diff:     UnifiedDiff(clean, before, content),
		Commands: []string{"nginx -t", "systemctl reload nginx"},
	}

	if dryRun {
		out.Message = "Dry-run: yapılandırma doğrulanacak ve nginx yeniden yüklenecek (değişiklik uygulanmadı)."
		if out.Diff == "" {
			out.Message = "Değişiklik yok."
		}
		return out, nil
	}

	// 1. Snapshot the previous content so the change can be rolled back.
	snap := &models.Snapshot{
		ServerID: sv.ID, Kind: "nginx", Path: clean, Content: before,
		Note: "nginx config öncesi otomatik yedek", CreatedBy: operator,
	}
	if err := a.Store.SaveSnapshot(snap); err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	out.Backup = fmt.Sprintf("#%d", snap.ID)

	// 2. Write, validate, reload — rolling back on any failure.
	if err := c.WriteFile(ctx, clean, content); err != nil {
		return nil, err
	}
	testRes, err := c.RunSudo(ctx, "nginx -t")
	if err != nil {
		_ = c.WriteFile(ctx, clean, before)
		return nil, err
	}
	out.Output = strings.TrimSpace(testRes.Stderr + testRes.Stdout)
	if !testRes.OK() {
		_ = c.WriteFile(ctx, clean, before)
		out.Message = "Yapılandırma geçersiz (nginx -t başarısız). Değişiklik geri alındı."
		return out, fmt.Errorf("nginx -t failed: %s", out.Output)
	}

	reloadRes, err := c.RunSudo(ctx, "systemctl reload nginx || nginx -s reload")
	if err != nil || !reloadRes.OK() {
		_ = c.WriteFile(ctx, clean, before)
		_, _ = c.RunSudo(ctx, "systemctl reload nginx || nginx -s reload")
		out.Message = "Reload başarısız, önceki yapılandırmaya dönüldü."
		return out, fmt.Errorf("reload failed: %s", strings.TrimSpace(reloadRes.Stderr))
	}

	out.Message = "Yapılandırma doğrulandı ve nginx yeniden yüklendi."
	return out, nil
}

// EnableNginxSite creates the sites-enabled symlink.
func EnableNginxSite(ctx context.Context, c *sshx.Client, name string, dryRun bool) (*Outcome, error) {
	clean, err := GuardPath(nginxSitesAvailable+"/"+name, NginxScope)
	if err != nil {
		return nil, err
	}
	link := nginxSitesEnabled + "/" + name
	cmd := fmt.Sprintf("ln -sfn %s %s && nginx -t && (systemctl reload nginx || nginx -s reload)",
		shellQuote(clean), shellQuote(link))
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s etkinleştirilecek.", name)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out.Output = strings.TrimSpace(res.Stdout + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("enable failed: %s", out.Output)
	}
	out.Message = fmt.Sprintf("%s etkinleştirildi.", name)
	return out, nil
}

// DisableNginxSite removes the sites-enabled symlink.
func DisableNginxSite(ctx context.Context, c *sshx.Client, name string, dryRun bool) (*Outcome, error) {
	if _, err := GuardPath(nginxSitesAvailable+"/"+name, NginxScope); err != nil {
		return nil, err
	}
	link := nginxSitesEnabled + "/" + name
	cmd := fmt.Sprintf("rm -f %s && nginx -t && (systemctl reload nginx || nginx -s reload)", shellQuote(link))
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s devre dışı bırakılacak.", name)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out.Output = strings.TrimSpace(res.Stdout + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("disable failed: %s", out.Output)
	}
	out.Message = fmt.Sprintf("%s devre dışı bırakıldı.", name)
	return out, nil
}

// TestNginx runs the nginx configuration test.
func TestNginx(ctx context.Context, c *sshx.Client) (string, error) {
	res, err := c.RunSudo(ctx, "nginx -t")
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("nginx -t başarısız")
	}
	return out, nil
}

// ReloadNginx gracefully reloads the nginx service.
func ReloadNginx(ctx context.Context, c *sshx.Client, dryRun bool) (*Outcome, error) {
	cmd := "systemctl reload nginx || nginx -s reload"
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = "Dry-run: nginx yeniden yüklenecek."
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out.Output = strings.TrimSpace(res.Stdout + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("reload failed: %s", out.Output)
	}
	out.Message = "nginx yeniden yüklendi."
	return out, nil
}
