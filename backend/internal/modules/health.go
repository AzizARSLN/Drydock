package modules

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/AzizARSLN/drydock/internal/sshx"
)

const healthScript = `echo "===HOSTNAME==="; hostname 2>/dev/null
echo "===OS==="; (. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME") || echo unknown
echo "===KERNEL==="; uname -r 2>/dev/null
echo "===UPTIME==="; uptime -p 2>/dev/null || uptime 2>/dev/null
echo "===LOAD==="; cat /proc/loadavg 2>/dev/null
echo "===MEM==="; free -m 2>/dev/null | awk '/^Mem:/{print $2, $3}'
echo "===CPU==="; (vmstat 1 2 2>/dev/null | tail -1) || echo ""
echo "===DISK==="; df -P -x tmpfs -x devtmpfs -x squashfs -x overlay 2>/dev/null | tail -n +2
echo "===FAILED==="; systemctl --failed --no-legend --plain 2>/dev/null | head -20
echo "===NGINX==="; systemctl is-active nginx 2>/dev/null || echo unknown
echo "===DOCKER==="; (command -v docker >/dev/null 2>&1 && echo yes) || echo no
echo "===END==="`

// CollectHealth gathers a health snapshot for a host in a single round trip.
func CollectHealth(ctx context.Context, c *sshx.Client, serverID int64) *HealthReport {
	rep := &HealthReport{
		ServerID:  serverID,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Disks:     []DiskMount{},
		FailedUnits: []string{},
		CertExpiring: []CertInfo{},
	}
	res, err := c.Run(ctx, healthScript)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	rep.Reachable = true

	for section, lines := range splitSections(res.Stdout) {
		switch section {
		case "HOSTNAME":
			rep.Hostname = firstLine(lines)
		case "OS":
			rep.OS = firstLine(lines)
		case "KERNEL":
			rep.Kernel = firstLine(lines)
		case "UPTIME":
			rep.Uptime = strings.TrimPrefix(firstLine(lines), "up ")
		case "LOAD":
			f := strings.Fields(firstLine(lines))
			if len(f) >= 3 {
				rep.LoadAvg = strings.Join(f[:3], " / ")
			}
		case "MEM":
			f := strings.Fields(firstLine(lines))
			if len(f) == 2 {
				rep.MemTotalMB, _ = strconv.ParseFloat(f[0], 64)
				rep.MemUsedMB, _ = strconv.ParseFloat(f[1], 64)
				if rep.MemTotalMB > 0 {
					rep.MemPercent = round1(rep.MemUsedMB / rep.MemTotalMB * 100)
				}
			}
		case "CPU":
			f := strings.Fields(firstLine(lines))
			if len(f) >= 15 {
				if idle, err := strconv.ParseFloat(f[14], 64); err == nil {
					rep.CPUPercent = round1(100 - idle)
				}
			}
		case "DISK":
			rep.Disks = parseDisks(lines)
		case "FAILED":
			for _, l := range lines {
				if n := strings.Fields(l); len(n) > 0 && strings.HasSuffix(n[0], ".service") {
					rep.FailedUnits = append(rep.FailedUnits, n[0])
				}
			}
		case "NGINX":
			rep.NginxState = firstLine(lines)
		case "DOCKER":
			rep.DockerOn = firstLine(lines) == "yes"
		}
	}
	return rep
}

func splitSections(out string) map[string][]string {
	sections := map[string][]string{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "===") && strings.HasSuffix(trimmed, "===") {
			current = strings.Trim(trimmed, "=")
			if current == "END" {
				current = ""
				continue
			}
			sections[current] = []string{}
			continue
		}
		if current != "" {
			sections[current] = append(sections[current], line)
		}
	}
	return sections
}

func firstLine(lines []string) string {
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

func parseDisks(lines []string) []DiskMount {
	var out []DiskMount
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) < 6 {
			continue
		}
		if !strings.HasPrefix(f[0], "/") {
			continue
		}
		total, _ := strconv.ParseFloat(f[1], 64)
		used, _ := strconv.ParseFloat(f[2], 64)
		pct := round1(pctFromString(f[4]))
		out = append(out, DiskMount{
			Device:  f[0],
			TotalGB: round2(total / 1024 / 1024),
			UsedGB:  round2(used / 1024 / 1024),
			UsedPct: pct,
			Mount:   f[5],
		})
	}
	return out
}

func pctFromString(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
