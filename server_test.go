package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestServerPageAndAssets(t *testing.T) {
	srv := httptest.NewServer(newHandler(t.TempDir(), "secret"))
	defer srv.Close()

	for path, want := range map[string]string{
		"/":                    "fk terminal",
		"/static/xterm.js":     "Terminal",
		"/static/addon-fit.js": "FitAddon",
		"/static/xterm.css":    ".xterm",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(string(body), want) {
			t.Fatalf("%s: status %d, missing %q", path, res.StatusCode, want)
		}
	}
}

func TestTerminalSocket(t *testing.T) {
	srv := httptest.NewServer(newHandler(t.TempDir(), "secret"))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?t="

	good := http.Header{"Origin": {"http://localhost:7700"}}

	// wrong token
	if _, res, err := websocket.DefaultDialer.Dial(wsURL+"nope", good); err == nil || res.StatusCode != 403 {
		t.Fatal("wrong token must be refused")
	}
	// another website
	evil := http.Header{"Origin": {"https://evil.example"}}
	if _, _, err := websocket.DefaultDialer.Dial(wsURL+"secret", evil); err == nil {
		t.Fatal("foreign origin must be refused")
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"secret", good)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	send := func(v any) {
		b, _ := json.Marshal(v)
		conn.WriteMessage(websocket.TextMessage, b)
	}
	send(map[string]any{"type": "resize", "cols": 120, "rows": 40})
	send(map[string]any{"type": "input", "data": "echo fk-$((40+2)) $(tput cols)\r"})

	var out strings.Builder
	deadline := time.Now().Add(10 * time.Second)
	conn.SetReadDeadline(deadline)
	for !strings.Contains(out.String(), "fk-42 120") {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("no echo from shell; got %q (%v)", out.String(), err)
		}
		out.Write(data)
	}
}
