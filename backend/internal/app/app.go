package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/AzizARSLN/drydock/internal/auth"
	"github.com/AzizARSLN/drydock/internal/config"
	"github.com/AzizARSLN/drydock/internal/crypto"
	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/AzizARSLN/drydock/internal/sshx"
	"github.com/AzizARSLN/drydock/internal/store"
)

// App is the shared dependency container passed to every module and handler.
type App struct {
	Cfg    *config.Config
	Store  *store.Store
	Auth   *auth.Service
	Cipher *crypto.Cipher
	Dialer *sshx.Dialer
}

// New wires the dependency container.
func New(cfg *config.Config, st *store.Store, a *auth.Service, c *crypto.Cipher) *App {
	return &App{Cfg: cfg, Store: st, Auth: a, Cipher: c, Dialer: sshx.NewDialer(c)}
}

// Connect opens an SSH connection to the given server and pins the host key on
// first use. Callers must Close the returned client.
func (a *App) Connect(ctx context.Context, serverID int64) (*sshx.Client, *models.Server, error) {
	sv, err := a.Store.GetServer(serverID)
	if err != nil {
		return nil, nil, err
	}
	if sv == nil {
		return nil, nil, errors.New("server not found")
	}
	client, err := a.Dialer.Dial(ctx, sv)
	if err != nil {
		return nil, sv, err
	}
	if sv.HostKeyFingerprint == "" && client.Fingerprint != "" {
		_ = a.Store.SetHostKey(sv.ID, client.Fingerprint)
		sv.HostKeyFingerprint = client.Fingerprint
	}
	return client, sv, nil
}

// MustCommand is a small helper that turns a failed command into an error.
func MustCommand(res sshx.ExecResult, err error, what string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if !res.OK() {
		msg := res.Stderr
		if msg == "" {
			msg = res.Stdout
		}
		if msg == "" {
			msg = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return fmt.Errorf("%s: %s", what, msg)
	}
	return nil
}
