package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseLinePrivmsg(t *testing.T) {
	line, ok := parseLine(":alice!u@example PRIVMSG #city :hello Gas City\r\n")
	if !ok {
		t.Fatal("parseLine returned false")
	}
	if line.Command != "PRIVMSG" || len(line.Params) != 1 || line.Params[0] != "#city" {
		t.Fatalf("unexpected parsed line: %#v", line)
	}
	if line.Prefix != "alice!u@example" || line.Trailing != "hello Gas City" {
		t.Fatalf("unexpected prefix/trailing: %#v", line)
	}
}

func TestEncodeSASLPlain(t *testing.T) {
	if got, want := EncodeSASLPlain("alice", "secret"), "AGFsaWNlAHNlY3JldA=="; got != want {
		t.Fatalf("EncodeSASLPlain() = %q, want %q", got, want)
	}
}

func TestBridgeDeliversOneMessage(t *testing.T) {
	var delivered externalInboundMessage
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/city/demo/extmsg/inbound" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-GC-Request"); got != "gc-irc-bridge" {
			t.Errorf("unexpected request marker %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		var envelope struct {
			Message externalInboundMessage `json:"message"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		delivered = envelope.Message
		w.WriteHeader(http.StatusAccepted)
	}))
	defer api.Close()

	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	serverDone := make(chan error, 1)
	go func() {
		defer func() { _ = clientConn.Close() }()
		reader := bufio.NewReader(clientConn)
		for i := 0; i < 3; i++ {
			if _, err := reader.ReadString('\n'); err != nil {
				serverDone <- err
				return
			}
		}
		_, err := io.WriteString(clientConn, ":irc.example.com CAP nick LS :multi-prefix\r\n:irc.example.com 001 nick :welcome\r\n")
		if err != nil {
			serverDone <- err
			return
		}
		if _, err := reader.ReadString('\n'); err != nil { // CAP END
			serverDone <- err
			return
		}
		join, err := reader.ReadString('\n')
		if err != nil {
			serverDone <- err
			return
		}
		if !strings.HasPrefix(join, "JOIN #city") {
			serverDone <- &testError{message: "client did not join configured channel"}
			return
		}
		_, err = io.WriteString(clientConn, ":alice!user@host PRIVMSG #city :hello Gas City\r\n")
		serverDone <- err
	}()

	cfg := Config{
		IRCServer: "irc.example.com", IRCPort: 6697, IRCNick: "nick", IRCChannel: "#city",
		GCAPIBaseURL: api.URL, GCCityName: "demo", GCTarget: "mayor", GCProvider: "irc", GCAccountID: "example-account",
		RetryInitial: time.Nanosecond, RetryMax: time.Nanosecond,
	}
	b := &Bridge{Config: cfg, HTTP: api.Client(), Dial: func(context.Context) (net.Conn, error) { return serverConn, nil }, Now: func() time.Time { return time.Date(2026, 10, 11, 17, 0, 0, 0, time.UTC) }}
	if err := b.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("IRC server: %v", err)
	}
	if delivered.Text != "hello Gas City" || delivered.Actor.ID != "alice" || delivered.ExplicitTarget != "mayor" {
		t.Fatalf("unexpected delivered message: %#v", delivered)
	}
	if delivered.Conversation.ConversationID != "#city" || delivered.Conversation.Kind != "room" {
		t.Fatalf("unexpected conversation: %#v", delivered.Conversation)
	}
}

type testError struct{ message string }

func (e *testError) Error() string { return e.message }
