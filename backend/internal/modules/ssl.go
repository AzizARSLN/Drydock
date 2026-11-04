package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AzizARSLN/drydock/internal/sshx"
)

const letsEncryptLive = "/etc/letsencrypt/live"

// ListCertificates inspects Let's Encrypt certificates and any certificate
// referenced by the supplied nginx site definitions.
func ListCertificates(ctx context.Context, c *sshx.Client, sites []NginxSite) ([]CertInfo, error) {
	paths := map[string]bool{}

	domains, _ := listDirNames(ctx, c, letsEncryptLive)
	for _, d := range domains {
		paths[letsEncryptLive+"/"+d+"/fullchain.pem"] = true
	}
	for _, s := range sites {
		if s.CertPath != "" && strings.HasPrefix(s.CertPath, "/") {
			paths[s.CertPath] = true
		}
	}
	if len(paths) == 0 {
		return []CertInfo{}, nil
	}

	list := make([]string, 0, len(paths))
	for p := range paths {
		list = append(list, p)
	}

	quoted := make([]string, len(list))
	for i, p := range list {
		quoted[i] = shellQuote(p)
	}
	cmd := fmt.Sprintf(
		`for f in %s; do echo "@@@$f"; openssl x509 -noout -subject -issuer -dates -in "$f" 2>/dev/null || echo MISSING; done`,
		strings.Join(quoted, " "))
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}

	var out []CertInfo
	var current string
	var lines []string
	flush := func() {
		if current == "" {
			return
		}
		info := CertInfo{Path: current, Domain: certDomain(current), AutoRenew: strings.Contains(current, "/letsencrypt/")}
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "issuer="):
				info.Issuer = shortenDN(strings.TrimPrefix(l, "issuer="))
			case strings.HasPrefix(l, "notBefore="):
				info.NotBefore = strings.TrimPrefix(l, "notBefore=")
			case strings.HasPrefix(l, "notAfter="):
				info.NotAfter = strings.TrimPrefix(l, "notAfter=")
			}
		}
		if info.NotAfter != "" {
			if t, err := time.Parse("Jan _2 15:04:05 2006 MST", info.NotAfter); err == nil {
				info.DaysLeft = int(time.Until(t).Hours() / 24)
			}
		}
		out = append(out, info)
		lines = nil
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.HasPrefix(line, "@@@") {
			flush()
			current = strings.TrimPrefix(line, "@@@")
			continue
		}
		if strings.Contains(line, "MISSING") {
			continue
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	flush()
	if out == nil {
		out = []CertInfo{}
	}
	return out, nil
}

func certDomain(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "live" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// Fall back to the file's parent directory name.
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return path
}

func shortenDN(dn string) string {
	for _, seg := range strings.Split(dn, ",") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, "CN") || strings.HasPrefix(seg, "O=") {
			return strings.TrimPrefix(strings.TrimPrefix(seg, "CN = "), "O = ")
		}
	}
	return dn
}

// RenewCertificate triggers a certbot renewal for the given certificate name.
func RenewCertificate(ctx context.Context, c *sshx.Client, domain string, dryRun bool) (*Outcome, error) {
	cmd := fmt.Sprintf(
		"certbot renew --cert-name %s --non-interactive --agree-tos",
		shellQuote(domain))
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s sertifikası için yenileme çalıştırılacak.", domain)
		// Report whether a renewal is due, without changing anything.
		check := fmt.Sprintf("certbot certificates --cert-name %s 2>&1 | head -n 20", shellQuote(domain))
		if res, err := c.RunSudo(ctx, check); err == nil {
			out.Output = strings.TrimSpace(res.Stdout + res.Stderr)
		}
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	out.Output = strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	if !res.OK() {
		return out, fmt.Errorf("certbot renew failed: %s", out.Output)
	}
	if _, err := c.RunSudo(ctx, "systemctl reload nginx || nginx -s reload"); err != nil {
		out.Message = "Sertifika yenilendi ancak nginx reload başarısız oldu."
		return out, nil
	}
	out.Message = "Sertifika yenilendi ve nginx yeniden yüklendi."
	return out, nil
}
