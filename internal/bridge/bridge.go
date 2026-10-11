// Package bridge connects one IRC conversation to Gas City's extmsg ingress.
package bridge

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRetryInitial = time.Second
	defaultRetryMax     = 30 * time.Second
)

// Config contains the non-secret settings for the bridge. Password is read
// from the environment by LoadConfig and is never included in errors.
type Config struct {
	IRCServer       string
	IRCPort         int
	IRCNick         string
	IRCUsername     string
	IRCRealName     string
	IRCSASLUsername string
	IRCPassword     string
	IRCChannel      string

	GCAPIBaseURL string
	GCCityName   string
	GCTarget     string
	GCProvider   string
	GCAccountID  string
	GCScopeID    string

	RegisterAdapter bool
	RetryInitial    time.Duration
	RetryMax        time.Duration
}

// Validate checks configuration without inspecting or echoing secrets.
func (c Config) Validate() error {
	if strings.TrimSpace(c.IRCServer) == "" {
		return errors.New("IRC_SERVER is required")
	}
	if c.IRCPort < 1 || c.IRCPort > 65535 {
		return errors.New("IRC_PORT must be between 1 and 65535")
	}
	for name, value := range map[string]string{
		"IRC_NICK": c.IRCNick, "IRC_CHANNEL": c.IRCChannel,
		"GC_API_BASE_URL": c.GCAPIBaseURL, "GC_CITY_NAME": c.GCCityName,
		"GC_TARGET": c.GCTarget,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if _, err := url.ParseRequestURI(c.GCAPIBaseURL); err != nil {
		return fmt.Errorf("GC_API_BASE_URL: %w", err)
	}
	if c.RetryInitial < 0 || c.RetryMax < 0 {
		return errors.New("retry delays cannot be negative")
	}
	return nil
}

// ConnDialer is injectable so protocol tests do not need a network service.
type ConnDialer func(context.Context) (net.Conn, error)

// HTTPDoer is the subset of http.Client used by Bridge.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Bridge delivers the first matching IRC PRIVMSG to extmsg, then returns.
type Bridge struct {
	Config Config
	HTTP   HTTPDoer
	Dial   ConnDialer
	Now    func() time.Time
}

// Run connects, reconnecting after a dropped connection until one message is
// delivered or ctx is cancelled. A successful delivery is the tracer bullet's
// completion condition.
func (b *Bridge) Run(ctx context.Context) error {
	if err := b.Config.Validate(); err != nil {
		return err
	}
	if b.HTTP == nil {
		b.HTTP = http.DefaultClient
	}
	if b.Now == nil {
		b.Now = time.Now
	}
	if b.Config.RetryInitial == 0 {
		b.Config.RetryInitial = defaultRetryInitial
	}
	if b.Config.RetryMax == 0 {
		b.Config.RetryMax = defaultRetryMax
	}
	if b.Dial == nil {
		b.Dial = b.tlsDial
	}

	backoff := b.Config.RetryInitial
	for {
		err := b.runConnection(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := wait(ctx, backoff); err != nil {
			return err
		}
		backoff *= 2
		if backoff > b.Config.RetryMax {
			backoff = b.Config.RetryMax
		}
	}
}

func (b *Bridge) tlsDial(ctx context.Context) (net.Conn, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 15 * time.Second},
		Config:    &tls.Config{ServerName: b.Config.IRCServer, MinVersion: tls.VersionTLS12},
	}
	return dialer.DialContext(ctx, "tcp", net.JoinHostPort(b.Config.IRCServer, strconv.Itoa(b.Config.IRCPort)))
}

func (b *Bridge) runConnection(ctx context.Context) error {
	conn, err := b.Dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-closed:
		}
	}()
	if err := writeIRC(conn, "CAP LS 302"); err != nil {
		return err
	}
	if err := writeIRC(conn, "NICK "+b.Config.IRCNick); err != nil {
		return err
	}
	username := b.Config.IRCUsername
	if username == "" {
		username = b.Config.IRCNick
	}
	realName := b.Config.IRCRealName
	if realName == "" {
		realName = "Gas City IRC bridge"
	}
	if err := writeIRC(conn, "USER "+username+" 0 * :"+realName); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	capNegotiating := true
	joined := false
	for {
		if err := setReadDeadline(ctx, conn); err != nil {
			return err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		parsed, ok := parseLine(line)
		if !ok {
			continue
		}
		if parsed.Command == "PING" {
			if err := writeIRC(conn, "PONG :"+parsed.Trailing); err != nil {
				return err
			}
			continue
		}
		if parsed.Command == "CAP" {
			if err := b.handleCAP(conn, parsed, &capNegotiating); err != nil {
				return err
			}
			continue
		}
		if parsed.Command == "AUTHENTICATE" && (parsed.Trailing == "+" || (len(parsed.Params) > 0 && parsed.Params[0] == "+")) {
			username := b.Config.IRCSASLUsername
			if username == "" {
				username = b.Config.IRCNick
			}
			if err := writeIRC(conn, "AUTHENTICATE "+EncodeSASLPlain(username, b.Config.IRCPassword)); err != nil {
				return err
			}
			continue
		}
		if parsed.Command == "903" {
			if err := writeIRC(conn, "CAP END"); err != nil {
				return err
			}
			capNegotiating = false
			continue
		}
		if parsed.Command == "904" || parsed.Command == "905" || parsed.Command == "906" {
			return errors.New("IRC SASL authentication failed")
		}
		if parsed.Command == "001" && !joined {
			if err := writeIRC(conn, "JOIN "+b.Config.IRCChannel); err != nil {
				return err
			}
			joined = true
			continue
		}
		if parsed.Command != "PRIVMSG" || len(parsed.Params) < 1 || parsed.Params[0] != b.Config.IRCChannel {
			continue
		}
		if strings.TrimSpace(parsed.Trailing) == "" {
			continue
		}
		message := inboundFromIRC(b.Config, parsed, b.Now())
		if err := b.postInbound(ctx, message); err != nil {
			return err
		}
		_ = writeIRC(conn, "QUIT :message delivered")
		return nil
	}
}

func (b *Bridge) handleCAP(conn net.Conn, line ircLine, negotiating *bool) error {
	if len(line.Params) < 2 {
		return nil
	}
	subcommand := strings.ToUpper(line.Params[1])
	capabilities := strings.ToLower(line.Trailing)
	switch subcommand {
	case "LS":
		if b.Config.IRCPassword != "" && strings.Contains(capabilities, "sasl") {
			return writeIRC(conn, "CAP REQ :sasl")
		}
		*negotiating = false
		return writeIRC(conn, "CAP END")
	case "ACK":
		if strings.Contains(capabilities, "sasl") && b.Config.IRCPassword != "" {
			return writeIRC(conn, "AUTHENTICATE PLAIN")
		}
		*negotiating = false
		return writeIRC(conn, "CAP END")
	}
	return nil
}

// RegisterAdapter announces this process to Gas City before inbound
// delivery. Registration is idempotent and lets the city validate the
// provider/account pair used in each inbound envelope.
func (b *Bridge) RegisterAdapter(ctx context.Context) error {
	if err := b.Config.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(adapterRegisterRequest{
		Provider:  b.Config.GCProvider,
		AccountID: b.Config.GCAccountID,
		Name:      "gascity-irc-bridge",
		Capabilities: adapterCapabilities{
			SupportsChildConversations: false,
			SupportsAttachments:        false,
			MaxMessageLength:           400,
		},
	})
	if err != nil {
		return err
	}
	target := strings.TrimRight(b.Config.GCAPIBaseURL, "/") + "/v0/city/" + url.PathEscape(b.Config.GCCityName) + "/extmsg/adapters"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GC-Request", "gc-irc-bridge")
	client := b.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("extmsg adapter registration: %s", resp.Status)
	}
	return nil
}

func (b *Bridge) postInbound(ctx context.Context, message externalInboundMessage) error {
	body, err := json.Marshal(struct {
		Message externalInboundMessage `json:"message"`
	}{Message: message})
	if err != nil {
		return err
	}
	target := strings.TrimRight(b.Config.GCAPIBaseURL, "/") + "/v0/city/" + url.PathEscape(b.Config.GCCityName) + "/extmsg/inbound"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GC-Request", "gc-irc-bridge")
	resp, err := b.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("extmsg inbound: %s", resp.Status)
	}
	return nil
}

type ircLine struct {
	Prefix   string
	Command  string
	Params   []string
	Trailing string
	Raw      string
}

func parseLine(raw string) (ircLine, bool) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return ircLine{}, false
	}
	result := ircLine{Raw: raw}
	if line[0] == ':' {
		space := strings.IndexByte(line, ' ')
		if space < 0 {
			return ircLine{}, false
		}
		result.Prefix, line = line[1:space], strings.TrimSpace(line[space+1:])
	}
	if colon := strings.IndexByte(line, ':'); colon >= 0 {
		result.Trailing = strings.TrimSpace(line[colon+1:])
		line = strings.TrimSpace(line[:colon])
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ircLine{}, false
	}
	result.Command = strings.ToUpper(parts[0])
	result.Params = parts[1:]
	return result, true
}

func inboundFromIRC(cfg Config, line ircLine, receivedAt time.Time) externalInboundMessage {
	actor := line.Prefix
	if bang := strings.IndexByte(actor, '!'); bang >= 0 {
		actor = actor[:bang]
	}
	digest := sha256.Sum256([]byte(line.Raw))
	id := "irc-" + hex.EncodeToString(digest[:8])
	scope := cfg.GCScopeID
	if scope == "" {
		scope = cfg.GCCityName
	}
	return externalInboundMessage{
		ProviderMessageID: id,
		Conversation: conversationRef{
			ScopeID: scope, Provider: cfg.GCProvider, AccountID: cfg.GCAccountID,
			ConversationID: cfg.IRCChannel, Kind: "room",
		},
		Actor: externalActor{ID: actor, DisplayName: actor},
		Text:  line.Trailing, ExplicitTarget: cfg.GCTarget, DedupKey: id,
		ReceivedAt: receivedAt.UTC(),
	}
}

func writeIRC(w io.Writer, line string) error {
	_, err := io.WriteString(w, line+"\r\n")
	return err
}

func setReadDeadline(ctx context.Context, conn net.Conn) error {
	if deadline, ok := ctx.Deadline(); ok {
		return conn.SetReadDeadline(deadline)
	}
	return conn.SetReadDeadline(time.Time{})
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// External extmsg wire types intentionally mirror Gas City's API types.
type conversationRef struct {
	ScopeID        string `json:"scope_id"`
	Provider       string `json:"provider"`
	AccountID      string `json:"account_id"`
	ConversationID string `json:"conversation_id"`
	Kind           string `json:"kind"`
}

type externalActor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	IsBot       bool   `json:"is_bot"`
}

type adapterCapabilities struct {
	SupportsChildConversations bool `json:"SupportsChildConversations"`
	SupportsAttachments        bool `json:"SupportsAttachments"`
	MaxMessageLength           int  `json:"MaxMessageLength"`
}

type adapterRegisterRequest struct {
	Provider     string              `json:"provider"`
	AccountID    string              `json:"account_id"`
	Name         string              `json:"name,omitempty"`
	Capabilities adapterCapabilities `json:"capabilities,omitempty"`
}

type externalInboundMessage struct {
	ProviderMessageID string          `json:"provider_message_id"`
	Conversation      conversationRef `json:"conversation"`
	Actor             externalActor   `json:"actor"`
	Text              string          `json:"text"`
	ExplicitTarget    string          `json:"explicit_target,omitempty"`
	ReplyToMessageID  string          `json:"reply_to_message_id,omitempty"`
	DedupKey          string          `json:"dedup_key,omitempty"`
	ReceivedAt        time.Time       `json:"received_at"`
}

// EncodeSASLPlain is kept small and public for callers that need to inspect
// the protocol without handling a credential themselves.
func EncodeSASLPlain(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte("\x00" + username + "\x00" + password))
}
