package sshx

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/AzizARSLN/drydock/internal/crypto"
	"github.com/AzizARSLN/drydock/internal/models"
	"golang.org/x/crypto/ssh"
)

// ErrHostKeyMismatch is returned when the presented host key differs from the pinned one.
var ErrHostKeyMismatch = errors.New("ssh host key mismatch")

// ExecResult is the outcome of a remote command.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// OK reports whether the command finished successfully.
func (r ExecResult) OK() bool { return r.ExitCode == 0 }

// secrets holds decrypted credentials for a single connection attempt.
type secrets struct {
	privateKey   string
	password     string
	passphrase   string
	sudoPassword string
}

// Dialer opens authenticated SSH connections on behalf of a managed server.
type Dialer struct {
	cipher  *crypto.Cipher
	timeout time.Duration
}

// NewDialer builds a Dialer able to decrypt stored credentials.
func NewDialer(c *crypto.Cipher) *Dialer {
	return &Dialer{cipher: c, timeout: 15 * time.Second}
}

func (d *Dialer) resolve(sv *models.Server) (*secrets, error) {
	s := &secrets{}
	var err error
	if s.privateKey, err = d.cipher.Decrypt(sv.PrivateKeyEnc); err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}
	if s.password, err = d.cipher.Decrypt(sv.PasswordEnc); err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}
	if s.passphrase, err = d.cipher.Decrypt(sv.PassphraseEnc); err != nil {
		return nil, fmt.Errorf("decrypt passphrase: %w", err)
	}
	if s.sudoPassword, err = d.cipher.Decrypt(sv.SudoPasswordEnc); err != nil {
		return nil, fmt.Errorf("decrypt sudo password: %w", err)
	}
	return s, nil
}

// Client is an authenticated SSH connection to a single host.
type Client struct {
	Server      *models.Server
	Fingerprint string
	conn        *ssh.Client
	secrets     *secrets
}

// Dial connects to the server, verifying (or pinning) the SSH host key.
// The returned fingerprint must be persisted by the caller when the server had
// no pinned key yet.
func (d *Dialer) Dial(ctx context.Context, sv *models.Server) (*Client, error) {
	sec, err := d.resolve(sv)
	if err != nil {
		return nil, err
	}

	pinned := sv.HostKeyFingerprint
	var captured string
	hostKeyCb := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		captured = fp
		if pinned == "" {
			return nil // trust on first use
		}
		if fp != pinned {
			return fmt.Errorf("%w: expected %s, got %s", ErrHostKeyMismatch, pinned, fp)
		}
		return nil
	}

	var auths []ssh.AuthMethod
	if pk := strings.TrimSpace(sec.privateKey); pk != "" {
		var signer ssh.Signer
		if sec.passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(pk), []byte(sec.passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(pk))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if sec.password != "" {
		auths = append(auths, ssh.Password(sec.password))
	}
	if len(auths) == 0 {
		return nil, errors.New("no usable credentials: provide a private key or a password")
	}

	cfg := &ssh.ClientConfig{
		User:            sv.Username,
		Auth:            auths,
		HostKeyCallback: hostKeyCb,
		Timeout:         d.timeout,
		ClientVersion:   "SSH-2.0-Drydock",
	}

	addr := net.JoinHostPort(sv.Host, strconv.Itoa(sv.Port))
	nd := net.Dialer{Timeout: d.timeout}
	netConn, err := nd.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, cfg)
	if err != nil {
		_ = netConn.Close()
		if strings.Contains(err.Error(), "host key mismatch") {
			return nil, ErrHostKeyMismatch
		}
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return &Client{
		Server:      sv,
		Fingerprint: captured,
		conn:        ssh.NewClient(sshConn, chans, reqs),
		secrets:     sec,
	}, nil
}

// Close terminates the connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// SSH returns the raw underlying connection (used by the web terminal).
func (c *Client) SSH() *ssh.Client { return c.conn }

// SudoPassword exposes the decrypted sudo password for interactive flows.
func (c *Client) SudoPassword() string { return c.secrets.sudoPassword }

// needsSudo reports whether ordinary commands must be elevated.
func (c *Client) needsSudo() bool { return c.Server.Username != "root" }

func (c *Client) sudoPrefix() string {
	if !c.needsSudo() {
		return ""
	}
	if c.secrets.sudoPassword != "" {
		return "sudo -S -p '' "
	}
	return "sudo -n "
}

// Run executes a command with a bounded timeout and captures its output.
func (c *Client) Run(ctx context.Context, cmd string) (ExecResult, error) {
	return c.run(ctx, cmd, false)
}

// RunSudo executes a privileged command (no-op prefix when already root).
func (c *Client) RunSudo(ctx context.Context, cmd string) (ExecResult, error) {
	return c.run(ctx, cmd, true)
}

func (c *Client) run(ctx context.Context, cmd string, sudo bool) (ExecResult, error) {
	if c.conn == nil {
		return ExecResult{}, errors.New("connection is closed")
	}
	sess, err := c.conn.NewSession()
	if err != nil {
		return ExecResult{}, err
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	full := "bash -lc " + shellQuote(cmd)
	if sudo {
		full = c.sudoPrefix() + "bash -lc " + shellQuote(cmd)
		if c.needsSudo() && c.secrets.sudoPassword != "" {
			sess.Stdin = strings.NewReader(c.secrets.sudoPassword + "\n")
		}
	} else if c.needsSudo() {
		// Allow the remote user to inspect system state without a password prompt.
		full = "bash -lc " + shellQuote(cmd)
	}

	done := make(chan error, 1)
	go func() { done <- sess.Run(full) }()

	select {
	case <-ctx.Done():
		_ = sess.Signal(ssh.SIGKILL)
		_ = sess.Close()
		return ExecResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: -1},
			context.DeadlineExceeded
	case err := <-done:
		res := ExecResult{Stdout: stdout.String(), Stderr: stderr.String()}
		if err == nil {
			return res, nil
		}
		var ee *ssh.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitStatus()
			return res, nil // non-zero exit is a valid result, not a transport error
		}
		return res, err
	}
}

// ReadFile reads a (possibly privileged) remote file as text.
func (c *Client) ReadFile(ctx context.Context, path string) (string, error) {
	res, err := c.RunSudo(ctx, "cat "+shellQuote(path))
	if err != nil {
		return "", err
	}
	if !res.OK() {
		return "", fmt.Errorf("read %s: %s", path, firstNonEmpty(res.Stderr, "exit code "+strconv.Itoa(res.ExitCode)))
	}
	return res.Stdout, nil
}

// FileExists reports whether a remote path is present.
func (c *Client) FileExists(ctx context.Context, path string) bool {
	res, err := c.RunSudo(ctx, "test -e "+shellQuote(path)+" && echo yes || echo no")
	return err == nil && strings.TrimSpace(res.Stdout) == "yes"
}

// WriteFile atomically writes text to a remote path using base64 transport.
func (c *Client) WriteFile(ctx context.Context, path, content string) error {
	enc := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf(
		"printf %%s %s | base64 -d > /tmp/drydock.tmp && "+
			"mv /tmp/drydock.tmp %s",
		shellQuote(enc), shellQuote(path))
	res, err := c.RunSudo(ctx, cmd)
	if err != nil {
		return err
	}
	if !res.OK() {
		return fmt.Errorf("write %s: %s", path, firstNonEmpty(res.Stderr, "exit code "+strconv.Itoa(res.ExitCode)))
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// shellQuote wraps a string in single quotes, escaping embedded quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
