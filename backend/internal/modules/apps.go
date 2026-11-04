package modules

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/sshx"
)

// ListServices returns managed units. kind selects "docker", "systemd" or "all".
func ListServices(ctx context.Context, c *sshx.Client, kind, filter string) ([]Service, error) {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var out []Service

	if kind == "" || kind == "all" || kind == "docker" {
		docker, err := listDocker(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, docker...)
	}
	if kind == "" || kind == "all" || kind == "systemd" {
		units, err := listSystemd(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, units...)
	}

	if filter == "" {
		return out, nil
	}
	var filtered []Service
	for _, s := range out {
		if strings.Contains(strings.ToLower(s.Name), filter) ||
			strings.Contains(strings.ToLower(s.Image), filter) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func listDocker(ctx context.Context, c *sshx.Client) ([]Service, error) {
	if res, _ := c.Run(ctx, "command -v docker >/dev/null 2>&1 && echo yes"); strings.TrimSpace(res.Stdout) != "yes" {
		return nil, nil
	}
	format := `{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.RunningFor}}\t{{.Label "com.docker.compose.project"}}\t{{.Label "com.docker.compose.service"}}\t{{.Label "com.docker.compose.project.working_dir"}}\t{{.Label "com.docker.compose.project.config_files"}}`
	res, err := c.RunSudo(ctx, "docker ps --format '"+format+"' 2>/dev/null")
	if err != nil {
		return nil, err
	}
	var out []Service
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		for len(f) < 8 {
			f = append(f, "")
		}
		out = append(out, Service{
			Name:        f[0],
			Kind:        "docker",
			Status:      f[2],
			Image:       f[1],
			Version:     imageTag(f[1]),
			Uptime:      f[3],
			Detail:      composeDetail(f[7], f[5]),
			Project:     f[4],
			ComposeSvc:  f[5],
			WorkingDir:  f[6],
			ComposeFile: firstComposeFile(f[7]),
		})
	}
	return out, nil
}

// firstComposeFile returns the first entry of the comma separated config_files
// label that compose records on every container.
func firstComposeFile(files string) string {
	if i := strings.Index(files, ","); i >= 0 {
		files = files[:i]
	}
	return strings.TrimSpace(files)
}

func imageTag(image string) string {
	if i := strings.LastIndex(image, ":"); i >= 0 {
		return image[i+1:]
	}
	return "latest"
}

func composeDetail(file, service string) string {
	if file == "" {
		return "standalone"
	}
	return fmt.Sprintf("compose: %s (%s)", file, service)
}

func listSystemd(ctx context.Context, c *sshx.Client) ([]Service, error) {
	res, err := c.Run(ctx,
		`systemctl list-units --type=service --state=running --no-pager --no-legend --plain 2>/dev/null | head -n 200`)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		names = append(names, fields[0])
	}
	if len(names) == 0 {
		return nil, nil
	}
	// Fetch versions for all units in one round trip.
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = shellQuote(n)
	}
	vres, _ := c.Run(ctx, fmt.Sprintf(
		`for u in %s; do v=$(systemctl show -p Version --value "$u" 2>/dev/null); echo "$u|${v:-}"; done`,
		strings.Join(quoted, " ")))
	versions := map[string]string{}
	for _, line := range strings.Split(vres.Stdout, "\n") {
		if parts := strings.SplitN(strings.TrimSpace(line), "|", 2); len(parts) == 2 {
			versions[parts[0]] = parts[1]
		}
	}

	out := make([]Service, 0, len(names))
	for _, n := range names {
		out = append(out, Service{
			Name:    strings.TrimSuffix(n, ".service"),
			Kind:    "systemd",
			Status:  "active",
			Version: versions[n],
		})
	}
	return out, nil
}

// ServiceAction performs start/stop/restart/reload on a managed unit.
func ServiceAction(ctx context.Context, c *sshx.Client, kind, name, action string, dryRun bool) (*Outcome, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("service name is required")
	}
	allowed := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
	if !allowed[action] {
		return nil, fmt.Errorf("unsupported action %q", action)
	}

	var cmd string
	switch kind {
	case "docker":
		if action == "reload" {
			return nil, fmt.Errorf("docker containers do not support reload; use restart")
		}
		cmd = fmt.Sprintf("docker %s %s", action, shellQuote(name))
	case "systemd":
		cmd = fmt.Sprintf("systemctl %s %s", action, shellQuote(name))
	default:
		return nil, fmt.Errorf("unknown service kind %q", kind)
	}

	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s (%s) için '%s' çalıştırılacak.", name, kind, action)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out.Output = strings.TrimSpace(res.Stdout + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("%s %s failed: %s", kind, action, out.Output)
	}
	out.Message = fmt.Sprintf("%s başarıyla %s.", name, action)
	return out, nil
}

// UpdateService pulls a newer image (docker) or upgrades the package (systemd).
func UpdateService(ctx context.Context, c *sshx.Client, kind, name, image, composeFile string, dryRun bool) (*Outcome, error) {
	var cmds []string
	switch kind {
	case "docker":
		if image == "" {
			return nil, fmt.Errorf("image reference is required for docker updates")
		}
		cmds = append(cmds, "docker pull "+shellQuote(image))
		if composeFile != "" {
			cmds = append(cmds, fmt.Sprintf("docker compose -f %s up -d --pull always", shellQuote(composeFile)))
		} else {
			cmds = append(cmds, fmt.Sprintf("docker restart %s", shellQuote(name)))
		}
	case "systemd":
		pkg := strings.TrimSuffix(name, ".service")
		pm := detectPackageManager(ctx, c)
		switch pm {
		case "apt":
			cmds = append(cmds, "apt-get update",
				fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install --only-upgrade -y %s", shellQuote(pkg)))
		case "dnf":
			cmds = append(cmds, fmt.Sprintf("dnf upgrade -y %s", shellQuote(pkg)))
		case "yum":
			cmds = append(cmds, fmt.Sprintf("yum update -y %s", shellQuote(pkg)))
		case "apk":
			cmds = append(cmds, fmt.Sprintf("apk upgrade %s", shellQuote(pkg)))
		default:
			return nil, fmt.Errorf("paket yöneticisi tespit edilemedi; güncelleme desteklenmiyor")
		}
		cmds = append(cmds, fmt.Sprintf("systemctl restart %s", shellQuote(pkg)))
	default:
		return nil, fmt.Errorf("unknown service kind %q", kind)
	}

	out := &Outcome{DryRun: dryRun, Commands: cmds}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s için %d adımlı güncelleme planlandı.", name, len(cmds))
		return out, nil
	}

	var log strings.Builder
	for _, cmd := range cmds {
		res, err := c.RunSudo(ctx, cmd)
		if err != nil {
			return out, err
		}
		log.WriteString("$ " + cmd + "\n" + res.Stdout + res.Stderr + "\n")
		if !res.OK() {
			out.Output = log.String()
			return out, fmt.Errorf("update step failed: %s", cmd)
		}
	}
	out.Output = log.String()
	out.Message = fmt.Sprintf("%s güncellendi.", name)
	return out, nil
}

func detectPackageManager(ctx context.Context, c *sshx.Client) string {
	res, _ := c.Run(ctx,
		`if command -v apt-get >/dev/null 2>&1; then echo apt; elif command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v yum >/dev/null 2>&1; then echo yum; elif command -v apk >/dev/null 2>&1; then echo apk; else echo none; fi`)
	return strings.TrimSpace(res.Stdout)
}

// composeActions maps the project-level actions the UI can trigger to the
// docker compose subcommands they expand to, run in order.
var composeActions = map[string][]string{
	"up":      {"pull", "up -d"},
	"restart": {"restart"},
	"down":    {"down"},
	"ps":      {"ps"},
	"logs":    {"logs --tail=200"},
}

// ComposeProjectAction runs a docker compose subcommand inside the project's
// root directory — the working directory recorded on the containers, which is
// exactly where the project must be pulled and (re)started. The directory is
// validated against the filesystem scope before any command runs.
func ComposeProjectAction(ctx context.Context, c *sshx.Client, workingDir, composeFile, action string, dryRun bool) (*Outcome, error) {
	subs, ok := composeActions[action]
	if !ok {
		return nil, fmt.Errorf("unsupported compose action %q", action)
	}
	dir, file, err := resolveComposeDir(workingDir, composeFile)
	if err != nil {
		return nil, err
	}

	base := "docker compose"
	if file != "" {
		base += " -f " + shellQuote(file)
	}
	parts := make([]string, len(subs))
	for i, s := range subs {
		parts[i] = base + " " + s
	}
	cmd := fmt.Sprintf("cd %s && %s", shellQuote(dir), strings.Join(parts, " && "))

	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s dizininde 'docker compose %s' çalıştırılacak.", dir, strings.Join(subs, " && "))
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out.Output = strings.TrimSpace(res.Stdout + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("docker compose %s başarısız: %s", action, out.Output)
	}
	out.Message = fmt.Sprintf("%s dizininde 'docker compose %s' tamamlandı.", dir, strings.Join(subs, " && "))
	return out, nil
}

// resolveComposeDir determines and guards the project root and, when the
// recorded config file is also inside the permitted scope, returns it so the
// command can pin the exact compose file.
func resolveComposeDir(workingDir, composeFile string) (string, string, error) {
	dir := strings.TrimSpace(workingDir)
	if dir == "" && composeFile != "" {
		dir = path.Dir(composeFile)
	}
	if dir == "" {
		return "", "", fmt.Errorf("proje kök dizini bilinmiyor")
	}
	cleanDir, err := GuardPath(dir, DefaultFSScope)
	if err != nil {
		return "", "", err
	}
	file := ""
	if composeFile != "" {
		if cf, err := GuardPath(composeFile, DefaultFSScope); err == nil {
			file = cf
		}
	}
	return cleanDir, file, nil
}

// composeScanRoots are the directories searched for compose files. Every entry
// is part of DefaultFSScope so any discovered path already passes GuardPath.
var composeScanRoots = []string{"/opt", "/srv", "/var/www", "/home", "/root"}

// ListComposeProjects scans the host for compose files and returns one entry per
// containing folder (the project root). This surfaces projects that are laid
// out as folders on disk even when their containers are stopped or were never
// started through the compose labels docker records.
func ListComposeProjects(ctx context.Context, c *sshx.Client) ([]ComposeProject, error) {
	if res, _ := c.Run(ctx, "command -v docker >/dev/null 2>&1 && echo yes"); strings.TrimSpace(res.Stdout) != "yes" {
		return []ComposeProject{}, nil
	}
	quoted := make([]string, len(composeScanRoots))
	for i, r := range composeScanRoots {
		quoted[i] = shellQuote(r)
	}
	cmd := fmt.Sprintf(
		`find %s -maxdepth 5 -type f \( -name docker-compose.yml -o -name docker-compose.yaml -o -name compose.yml -o -name compose.yaml \) 2>/dev/null`,
		strings.Join(quoted, " "))
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}

	out := []ComposeProject{}
	seen := map[string]bool{}
	for _, line := range strings.Split(res.Stdout, "\n") {
		file := strings.TrimSpace(line)
		if file == "" || !strings.HasPrefix(file, "/") {
			continue
		}
		dir := path.Dir(file)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, ComposeProject{Name: path.Base(dir), Dir: dir, File: file})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

// DeployRequest is a new application assembled from a pasted compose file. The
// optional nginx block wires a reverse proxy in front of the published port.
type DeployRequest struct {
	Name         string `json:"name"`
	Root         string `json:"root"`
	Compose      string `json:"compose"`
	Env          string `json:"env,omitempty"`
	Nginx        bool   `json:"nginx"`
	ServerName   string `json:"server_name,omitempty"`
	UpstreamPort int    `json:"upstream_port,omitempty"`
	CertPath     string `json:"cert_path,omitempty"`
	KeyPath      string `json:"key_path,omitempty"`
	DryRun       bool   `json:"dry_run"`
}

// reProjectName restricts project folder names to a safe, predictable set.
var reProjectName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// deployPlan is the validated, scope-guarded set of paths a deployment touches.
type deployPlan struct {
	dir         string
	composeFile string
	envFile     string
	nginxSite   string
	nginxLink   string
}

// DeployApp creates the project folder, writes the compose (and optional .env)
// file, brings the stack up and, when requested, installs and enables an nginx
// reverse-proxy vhost with validate-and-rollback safety.
func DeployApp(ctx context.Context, a *app.App, c *sshx.Client, sv *models.Server,
	operator string, in DeployRequest) (*Outcome, error) {

	plan, err := planDeploy(in)
	if err != nil {
		return nil, err
	}

	env := strings.TrimSpace(in.Env)
	commands := []string{
		"mkdir -p " + shellQuote(plan.dir),
		"cat > " + plan.composeFile + "  # docker-compose.yml",
	}
	if env != "" {
		commands = append(commands, "cat > "+plan.envFile+"  # .env")
	}
	commands = append(commands,
		fmt.Sprintf("cd %s && docker compose pull && docker compose up -d", shellQuote(plan.dir)))
	if in.Nginx {
		commands = append(commands,
			"cat > "+plan.nginxSite+"  # nginx vhost",
			fmt.Sprintf("ln -sfn %s %s && nginx -t && (systemctl reload nginx || nginx -s reload)",
				shellQuote(plan.nginxSite), shellQuote(plan.nginxLink)))
	}

	out := &Outcome{DryRun: in.DryRun, Commands: commands}
	if in.DryRun {
		out.Message = fmt.Sprintf("Dry-run: %s dizininde %d adımlı kurulum planlandı.", plan.dir, len(commands))
		if in.Nginx {
			out.Output = renderVhost(in)
		}
		return out, nil
	}

	var log strings.Builder
	run := func(desc, cmd string) error {
		res, err := c.RunSudo(ctx, cmd)
		if err != nil {
			return err
		}
		log.WriteString("$ " + cmd + "\n" + res.Stdout + res.Stderr + "\n")
		if !res.OK() {
			return fmt.Errorf("%s başarısız: %s", desc, strings.TrimSpace(res.Stderr+res.Stdout))
		}
		return nil
	}

	if err := run("klasör oluşturma", "mkdir -p "+shellQuote(plan.dir)); err != nil {
		out.Output = log.String()
		return out, err
	}
	if err := c.WriteFile(ctx, plan.composeFile, in.Compose); err != nil {
		out.Output = log.String()
		return out, fmt.Errorf("docker-compose.yml yazılamadı: %w", err)
	}
	log.WriteString("$ cat > " + plan.composeFile + "  # docker-compose.yml\n")
	if env != "" {
		if err := c.WriteFile(ctx, plan.envFile, in.Env); err != nil {
			out.Output = log.String()
			return out, fmt.Errorf(".env yazılamadı: %w", err)
		}
		log.WriteString("$ cat > " + plan.envFile + "  # .env\n")
	}

	if err := run("docker compose up",
		fmt.Sprintf("cd %s && docker compose pull && docker compose up -d", shellQuote(plan.dir))); err != nil {
		out.Output = log.String()
		return out, err
	}

	if in.Nginx {
		if err := applyVhost(ctx, a, c, sv, operator, plan, in, &log, out); err != nil {
			out.Output = log.String()
			return out, err
		}
	}

	out.Output = log.String()
	out.Message = fmt.Sprintf("%s kuruldu ve çalıştırıldı.", in.Name)
	return out, nil
}

// applyVhost writes and enables the nginx vhost, rolling back to the previous
// state when validation or reload fails.
func applyVhost(ctx context.Context, a *app.App, c *sshx.Client, sv *models.Server, operator string,
	plan *deployPlan, in DeployRequest, log *strings.Builder, out *Outcome) error {

	existed := c.FileExists(ctx, plan.nginxSite)
	before := ""
	if existed {
		before, _ = c.ReadFile(ctx, plan.nginxSite)
		snap := &models.Snapshot{ServerID: sv.ID, Kind: "nginx", Path: plan.nginxSite, Content: before,
			Note: "uygulama kurulumu öncesi nginx yedeği", CreatedBy: operator}
		if err := a.Store.SaveSnapshot(snap); err == nil {
			out.Backup = fmt.Sprintf("#%d", snap.ID)
		}
	}

	if err := c.WriteFile(ctx, plan.nginxSite, renderVhost(in)); err != nil {
		return fmt.Errorf("nginx vhost yazılamadı: %w", err)
	}
	log.WriteString("$ cat > " + plan.nginxSite + "  # nginx vhost\n")

	cmd := fmt.Sprintf("ln -sfn %s %s && nginx -t && (systemctl reload nginx || nginx -s reload)",
		shellQuote(plan.nginxSite), shellQuote(plan.nginxLink))
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return err
	}
	log.WriteString("$ " + cmd + "\n" + res.Stdout + res.Stderr + "\n")
	if !res.OK() {
		// Roll back so a broken vhost never reaches production.
		if existed {
			_ = c.WriteFile(ctx, plan.nginxSite, before)
		} else {
			_, _ = c.RunSudo(ctx, fmt.Sprintf("rm -f %s %s", shellQuote(plan.nginxSite), shellQuote(plan.nginxLink)))
		}
		out.Message = "Nginx doğrulaması başarısız; nginx değişiklikleri geri alındı."
		return fmt.Errorf("nginx -t failed: %s", strings.TrimSpace(res.Stderr+res.Stdout))
	}
	return nil
}

// planDeploy validates the request and returns the guarded paths to operate on.
func planDeploy(in DeployRequest) (*deployPlan, error) {
	name := strings.TrimSpace(in.Name)
	if !reProjectName.MatchString(name) {
		return nil, fmt.Errorf("geçersiz uygulama adı: yalnızca harf, rakam, nokta, tire ve alt çizgi kullanın")
	}
	if strings.TrimSpace(in.Compose) == "" {
		return nil, fmt.Errorf("docker-compose içeriği boş olamaz")
	}
	root, err := GuardPath(strings.TrimSpace(in.Root), DefaultFSScope)
	if err != nil {
		return nil, err
	}
	dir, err := GuardPath(path.Join(root, name), DefaultFSScope)
	if err != nil {
		return nil, err
	}

	plan := &deployPlan{
		dir:         dir,
		composeFile: dir + "/docker-compose.yml",
		envFile:     dir + "/.env",
		nginxSite:   nginxSitesAvailable + "/" + name + ".conf",
		nginxLink:   nginxSitesEnabled + "/" + name + ".conf",
	}
	if in.Nginx {
		if strings.TrimSpace(in.ServerName) == "" {
			return nil, fmt.Errorf("nginx için alan adı (server_name) gerekli")
		}
		if in.UpstreamPort < 1 || in.UpstreamPort > 65535 {
			return nil, fmt.Errorf("geçersiz upstream portu (1-65535)")
		}
		if (in.CertPath == "") != (in.KeyPath == "") {
			return nil, fmt.Errorf("SSL için hem sertifika hem anahtar yolu girilmeli")
		}
		if _, err := GuardPath(plan.nginxSite, NginxScope); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// renderVhost builds the reverse-proxy server block for the new application.
func renderVhost(in DeployRequest) string {
	tls := in.CertPath != "" && in.KeyPath != ""
	var b strings.Builder
	b.WriteString("server {\n")
	if tls {
		b.WriteString("    listen 443 ssl http2;\n")
	} else {
		b.WriteString("    listen 80;\n")
	}
	b.WriteString("    server_name " + strings.Join(strings.Fields(in.ServerName), " ") + ";\n\n")
	if tls {
		b.WriteString("    ssl_certificate " + strings.TrimSpace(in.CertPath) + ";\n")
		b.WriteString("    ssl_certificate_key " + strings.TrimSpace(in.KeyPath) + ";\n\n")
	}
	b.WriteString("    location / {\n")
	fmt.Fprintf(&b, "        proxy_pass http://127.0.0.1:%d;\n", in.UpstreamPort)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	b.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	b.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	b.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
	b.WriteString("        proxy_set_header Connection \"upgrade\";\n")
	b.WriteString("    }\n}\n")
	return b.String()
}
