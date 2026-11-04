package modules

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

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
