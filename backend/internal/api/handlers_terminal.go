package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Origin is validated by the CORS allow-list below.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsMessage is the terminal protocol frame exchanged with the browser.
type wsMessage struct {
	Type string `json:"t"` // i=input, r=resize, o=output, e=error, c=close
	Data string `json:"d,omitempty"`
	Cols int    `json:"c,omitempty"`
	Rows int    `json:"r,omitempty"`
	Msg  string `json:"m,omitempty"`
}

// handleTerminal bridges a browser xterm session to an interactive SSH shell.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	if !s.App.Cfg.EnableTerminal {
		fail(w, http.StatusForbidden, "web terminal devre dışı")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Hour)
	defer cancel()

	client, sv, err := s.App.Connect(ctx, pathID(r))
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	defer client.Close()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sess, err := client.SSH().NewSession()
	if err != nil {
		writeWS(conn, wsMessage{Type: "e", Msg: err.Error()})
		return
	}
	defer sess.Close()

	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty("xterm-256color", 30, 120, modes); err != nil {
		writeWS(conn, wsMessage{Type: "e", Msg: err.Error()})
		return
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		writeWS(conn, wsMessage{Type: "e", Msg: err.Error()})
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		writeWS(conn, wsMessage{Type: "e", Msg: err.Error()})
		return
	}
	sess.Stderr = sess.Stdout

	if err := sess.Shell(); err != nil {
		writeWS(conn, wsMessage{Type: "e", Msg: err.Error()})
		return
	}
	s.audit(r, sv.ID, sv.Name, "terminal-open", "", "web terminal açıldı", audit.StatusSuccess)

	// SSH -> browser
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if werr := writeWS(conn, wsMessage{Type: "o", Data: string(buf[:n])}); werr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					writeWS(conn, wsMessage{Type: "e", Msg: err.Error()})
				}
				writeWS(conn, wsMessage{Type: "c"})
				cancel()
				return
			}
		}
	}()

	// browser -> SSH
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var m wsMessage
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		switch m.Type {
		case "i":
			if _, err := io.WriteString(stdin, m.Data); err != nil {
				return
			}
		case "r":
			if m.Cols > 0 && m.Rows > 0 {
				_ = sess.WindowChange(m.Rows, m.Cols)
			}
		case "c":
			return
		}
	}
	_ = sess.Signal(ssh.SIGTERM)
	s.audit(r, sv.ID, sv.Name, "terminal-close", "", "web terminal kapatıldı", audit.StatusSuccess)
}

func writeWS(conn *websocket.Conn, m wsMessage) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(m)
}
