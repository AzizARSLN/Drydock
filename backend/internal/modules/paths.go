package modules

import (
	"fmt"
	"path"
	"strings"
)

// DefaultFSScope lists the directory roots the file manager and config editors
// are allowed to touch. Anything outside is rejected before a command runs.
var DefaultFSScope = []string{
	"/etc", "/opt", "/srv", "/var/www", "/var/log", "/home", "/root", "/tmp",
}

// NginxScope restricts nginx configuration edits to the expected directories.
var NginxScope = []string{"/etc/nginx"}

// GuardPath ensures p is absolute, free of traversal segments and contained in
// one of the allowed roots. It returns the cleaned path on success.
func GuardPath(p string, roots []string) (string, error) {
	if p == "" || !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path must be absolute: %q", p)
	}
	clean := path.Clean(p)
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path traversal is not allowed: %q", p)
	}
	for _, r := range roots {
		if clean == r || strings.HasPrefix(clean, r+"/") {
			return clean, nil
		}
	}
	return "", fmt.Errorf("path %q is outside the permitted scope", clean)
}

// shellQuote wraps s in single quotes, escaping embedded quotes for safe use in
// remote shell commands.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
