package modules

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/sshx"
)

// ListPorts returns every listening socket on the host.
func ListPorts(ctx context.Context, c *sshx.Client) ([]ListeningPort, error) {
	res, err := c.RunSudo(ctx, "ss -tlunpH 2>/dev/null || netstat -tulnp 2>/dev/null || true")
	if err != nil {
		return nil, err
	}
	var out []ListeningPort
	for _, line := range strings.Split(res.Stdout, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 5 {
			continue
		}
		proto := strings.ToLower(f[0])
		if proto != "tcp" && proto != "udp" {
			continue
		}
		local := f[4]
		addr, port := splitHostPort(local)
		if port == 0 {
			continue
		}
		out = append(out, ListeningPort{
			Proto:   proto,
			Address: addr,
			Port:    port,
			Process: extractProcess(strings.Join(f[5:], " ")),
		})
	}
	if out == nil {
		out = []ListeningPort{}
	}
	return out, nil
}

func splitHostPort(s string) (string, int) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, 0
	}
	host := strings.Trim(s[:i], "[]")
	port, _ := strconv.Atoi(strings.TrimSpace(s[i+1:]))
	return host, port
}

var reProcName = regexp.MustCompile(`\(\("([^"]+)"`)

func extractProcess(s string) string {
	if m := reProcName.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if strings.Contains(s, "users:") {
		return "unknown"
	}
	return ""
}

var reListenPort = regexp.MustCompile(`(?m)^(\s*listen\s+)(\[[^\]]*\]:|[\d\.\*:]*:)?(\d+)([^;]*);`)

// ChangeNginxListenPort rewrites the listen directive of a site config and
// applies it through the validate-and-rollback pipeline.
func ChangeNginxListenPort(ctx context.Context, a *app.App, c *sshx.Client, sv *models.Server,
	operator, filePath string, oldPort, newPort int, dryRun bool) (*Outcome, error) {

	clean, err := GuardPath(filePath, NginxScope)
	if err != nil {
		return nil, err
	}
	content, err := c.ReadFile(ctx, clean)
	if err != nil {
		return nil, err
	}
	updated, count := replaceListenPort(content, oldPort, newPort)
	if count == 0 {
		return nil, fmt.Errorf("%d portu %s dosyasında bulunamadı", oldPort, clean)
	}
	return SaveNginxConfig(ctx, a, c, sv, operator, clean, updated, dryRun)
}

func replaceListenPort(content string, oldPort, newPort int) (string, int) {
	count := 0
	result := reListenPort.ReplaceAllStringFunc(content, func(match string) string {
		sub := reListenPort.FindStringSubmatch(match)
		if len(sub) < 5 {
			return match
		}
		if sub[3] != strconv.Itoa(oldPort) {
			return match
		}
		count++
		return sub[1] + sub[2] + strconv.Itoa(newPort) + sub[4] + ";"
	})
	return result, count
}
