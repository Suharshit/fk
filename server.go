package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

//go:embed web
var webFiles embed.FS

func runServe(args []string) error {
	fset := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fset.Int("port", 7700, "port to listen on")
	dir := fset.String("dir", ".", "folder the terminal starts in")
	noOpen := fset.Bool("no-open", false, "don't open the browser")
	if err := fset.Parse(args); err != nil {
		return err
	}
	startDir, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	if st, err := os.Stat(startDir); err != nil || !st.IsDir() {
		return fmt.Errorf("%s is not a folder", startDir)
	}

	// A random token in the URL stops other websites open in your browser
	// from talking to this terminal.
	tok := make([]byte, 16)
	rand.Read(tok)
	token := hex.EncodeToString(tok)

	mux := newHandler(startDir, token)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on port %d (is it in use? try --port %d): %w", *port, *port+1, err)
	}
	link := fmt.Sprintf("http://localhost:%d/?t=%s", *port, token)

	fmt.Printf("\n  %s fk %s web terminal is running\n\n", bold+inverse+cyan, reset)
	fmt.Printf("  Open:    %s%s%s\n", bold, link, reset)
	fmt.Printf("  Folder:  %s\n", startDir)
	fmt.Printf("  %sOnly this computer can connect. Press Ctrl+C to stop.%s\n\n", dim, reset)

	if !*noOpen {
		go openBrowser(link)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}

// newHandler serves the page, the xterm.js files and the terminal socket.
func newHandler(startDir, token string) http.Handler {
	static, _ := fs.Sub(webFiles, "web")
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page, _ := fs.ReadFile(static, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(page)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(token)) != 1 {
			http.Error(w, "bad token: open the link printed by `fk serve`", http.StatusForbidden)
			return
		}
		serveTerminal(w, r, startDir)
	})

	return mux
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 32768,
	CheckOrigin: func(r *http.Request) bool {
		u, err := url.Parse(r.Header.Get("Origin"))
		if err != nil {
			return false
		}
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	},
}

type clientMsg struct {
	Type string `json:"type"` // "input" or "resize"
	Data string `json:"data"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func serveTerminal(w http.ResponseWriter, r *http.Request, dir string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell, "-i")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "FK_WEB=1")
	// make sure `fk` itself can be found inside the web terminal
	if exe, err := os.Executable(); err == nil {
		cmd.Env = append(cmd.Env, "PATH="+filepath.Dir(exe)+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 100, Rows: 30})
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("could not start shell: "+err.Error()))
		return
	}
	defer func() {
		ptmx.Close()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
	}()

	var writeMu sync.Mutex
	done := make(chan struct{})

	// shell -> browser
	go func() {
		defer close(done)
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				writeMu.Lock()
				werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				writeMu.Unlock()
				if werr != nil {
					return
				}
			}
			if err != nil {
				writeMu.Lock()
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shell exited"))
				writeMu.Unlock()
				return
			}
		}
	}()

	// browser -> shell
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				ptmx.Close()
				return
			}
			var m clientMsg
			if json.Unmarshal(data, &m) != nil {
				continue
			}
			switch m.Type {
			case "input":
				if _, err := io.WriteString(ptmx, m.Data); err != nil && !errors.Is(err, os.ErrClosed) {
					return
				}
			case "resize":
				if m.Cols > 0 && m.Rows > 0 {
					pty.Setsize(ptmx, &pty.Winsize{Cols: m.Cols, Rows: m.Rows})
				}
			}
		}
	}()

	<-done
}

// openBrowser tries the usual ways to open a link, including from WSL.
func openBrowser(link string) {
	time.Sleep(300 * time.Millisecond)
	tries := [][]string{
		{"wslview", link},
		{"xdg-open", link},
		{"open", link},
	}
	if isWSL() {
		// cmd.exe treats & specially; our link has none, but quote anyway
		tries = append([][]string{{"wslview", link}, {"cmd.exe", "/c", "start", "", link}}, tries...)
	}
	for _, t := range tries {
		if _, err := exec.LookPath(t[0]); err != nil {
			continue
		}
		c := exec.Command(t[0], t[1:]...)
		c.Stdout, c.Stderr = nil, nil
		if c.Start() == nil {
			go c.Wait()
			return
		}
	}
}

func isWSL() bool {
	b, err := os.ReadFile("/proc/version")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}
