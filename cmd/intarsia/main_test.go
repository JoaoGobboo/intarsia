package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/JoaoGobboo/intarsia/internal/server"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd("1.2.3")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := out.String(), "intarsia 1.2.3\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	cmd := newRootCmd("dev")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"no-such-command"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error")
	}
}

func TestServeAnswersUntilCancelled(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	assets := fstest.MapFS{"index.html": {Data: []byte("<html>ok</html>")}}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, listener, server.New(assets)) }()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+listener.Addr().String()+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close body: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "<html>ok</html>" {
		t.Errorf("GET / = %d %q, want 200 with the index page", resp.StatusCode, body)
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("serve() error = %v, want nil after cancellation", err)
	}
}
