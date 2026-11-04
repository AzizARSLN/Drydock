package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/sshx"
)

// maxFileRead bounds how much of a remote file is transferred to the UI.
const maxFileRead = 512 * 1024

// ListFiles returns the entries of a remote directory.
func ListFiles(ctx context.Context, c *sshx.Client, dir string) ([]FileEntry, error) {
	clean, err := GuardPath(dir, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	cmd := fmt.Sprintf(
		`find %s -maxdepth 1 -mindepth 1 -printf '%%y\t%%s\t%%TY-%%Tm-%%Td %%TH:%%TM\t%%M\t%%u\t%%f\n' 2>/dev/null | sort -k7`,
		shellQuote(clean))
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	var out []FileEntry
	for _, line := range strings.Split(res.Stdout, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 7 {
			continue
		}
		size, _ := strconv.ParseInt(f[1], 10, 64)
		out = append(out, FileEntry{
			Name:    f[6],
			Path:    clean + "/" + f[6],
			IsDir:   f[0] == "d",
			Size:    size,
			Mode:    f[3],
			ModTime: f[2],
			Owner:   f[4],
		})
	}
	if out == nil {
		out = []FileEntry{}
	}
	return out, nil
}

// ReadFileContent returns up to maxFileRead bytes of a remote file.
func ReadFileContent(ctx context.Context, c *sshx.Client, filePath string) (string, error) {
	clean, err := GuardPath(filePath, DefaultFSScope)
	if err != nil {
		return "", err
	}
	res, err := c.RunSudo(ctx, fmt.Sprintf("head -c %d %s", maxFileRead, shellQuote(clean)))
	if err != nil {
		return "", err
	}
	if !res.OK() {
		return "", fmt.Errorf("read %s: %s", clean, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

// WriteFileContent saves an edited file, snapshotting the previous revision.
func WriteFileContent(ctx context.Context, a *app.App, c *sshx.Client, sv *models.Server,
	operator, filePath, content string, dryRun bool) (*Outcome, error) {

	clean, err := GuardPath(filePath, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	before, _ := c.ReadFile(ctx, clean)

	out := &Outcome{DryRun: dryRun, Diff: UnifiedDiff(clean, before, content)}
	if dryRun {
		out.Message = "Dry-run: dosya içeriği değiştirilecek (yazılmadı)."
		return out, nil
	}
	snap := &models.Snapshot{ServerID: sv.ID, Kind: "file", Path: clean, Content: before,
		Note: "dosya düzenlemesi öncesi yedek", CreatedBy: operator}
	if err := a.Store.SaveSnapshot(snap); err != nil {
		return nil, err
	}
	out.Backup = fmt.Sprintf("#%d", snap.ID)
	if err := c.WriteFile(ctx, clean, content); err != nil {
		return nil, err
	}
	out.Message = "Dosya kaydedildi."
	return out, nil
}

// DeletePath removes a file or directory tree.
func DeletePath(ctx context.Context, c *sshx.Client, target string, recursive, dryRun bool) (*Outcome, error) {
	clean, err := GuardPath(target, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	flag := ""
	if recursive {
		flag = "-r "
	}
	cmd := fmt.Sprintf("rm %s-f %s", flag, shellQuote(clean))
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s silinecek.", clean)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return out, fmt.Errorf("delete failed: %s", strings.TrimSpace(res.Stderr))
	}
	out.Message = fmt.Sprintf("%s silindi.", clean)
	return out, nil
}

// CreateDirectory creates a directory (and parents).
func CreateDirectory(ctx context.Context, c *sshx.Client, dir string, dryRun bool) (*Outcome, error) {
	clean, err := GuardPath(dir, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	cmd := "mkdir -p " + shellQuote(clean)
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s oluşturulacak.", clean)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return out, fmt.Errorf("mkdir failed: %s", strings.TrimSpace(res.Stderr))
	}
	out.Message = fmt.Sprintf("%s oluşturuldu.", clean)
	return out, nil
}

// RenamePath moves or renames a file/directory.
func RenamePath(ctx context.Context, c *sshx.Client, from, to string, dryRun bool) (*Outcome, error) {
	src, err := GuardPath(from, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	dst, err := GuardPath(to, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	cmd := fmt.Sprintf("mv %s %s", shellQuote(src), shellQuote(dst))
	out := &Outcome{DryRun: dryRun, Commands: []string{cmd}}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s -> %s", src, dst)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return out, fmt.Errorf("move failed: %s", strings.TrimSpace(res.Stderr))
	}
	out.Message = fmt.Sprintf("%s -> %s taşındı.", src, dst)
	return out, nil
}

// DownloadFile returns the raw bytes of a remote file as base64 for the browser.
func DownloadFile(ctx context.Context, c *sshx.Client, filePath string) (string, error) {
	clean, err := GuardPath(filePath, DefaultFSScope)
	if err != nil {
		return "", err
	}
	res, err := c.RunSudo(ctx, "base64 -w0 "+shellQuote(clean))
	if err != nil {
		return "", err
	}
	if !res.OK() {
		return "", fmt.Errorf("download failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout), nil
}

// UploadFile writes base64 content received from the browser to a remote path.
func UploadFile(ctx context.Context, c *sshx.Client, filePath, base64Content string, dryRun bool) (*Outcome, error) {
	clean, err := GuardPath(filePath, DefaultFSScope)
	if err != nil {
		return nil, err
	}
	cmd := fmt.Sprintf("printf %%s %s | base64 -d > %s", shellQuote(base64Content), shellQuote(clean))
	out := &Outcome{DryRun: dryRun}
	if dryRun {
		out.Message = fmt.Sprintf("Dry-run: %s konumuna yüklenecek.", clean)
		return out, nil
	}
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return out, fmt.Errorf("upload failed: %s", strings.TrimSpace(res.Stderr))
	}
	out.Message = fmt.Sprintf("%s yüklendi.", clean)
	return out, nil
}
