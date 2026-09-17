package sshproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStartLocalHTTPProxy_Connect(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello from target server")
	}))
	defer ts.Close()

	proxyPort, cleanup, err := StartLocalHTTPProxy()
	if err != nil {
		t.Fatalf("failed to start local proxy: %v", err)
	}
	defer cleanup()

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to make request through HTTP proxy: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if string(body) != "hello from target server\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "''"},
		{"simple", "simple"},
		{"path/to/file", "path/to/file"},
		{"hello world", "'hello world'"},
		{"user@host:dir", "user@host:dir"},
		{"it's cold", "'it'\"'\"'s cold'"},
		{"$VAR", "'$VAR'"},
		{"file*glob", "'file*glob'"},
		{"file?glob", "'file?glob'"},
		{"~home", "'~home'"},
		{"a;b", "'a;b'"},
		{"a|b", "'a|b'"},
		{"a&b", "'a&b'"},
	}
	for _, tt := range tests {
		got := ShellQuote(tt.input)
		if got != tt.expected {
			t.Errorf("ShellQuote(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestSSH_ServerFlagInjection(t *testing.T) {
	syncer := NewSSHProfileSyncer()
	err := syncer.SyncProfile(context.Background(), "-oProxyCommand=calc", "work")
	if err == nil || !strings.Contains(err.Error(), "cannot start with '-'") {
		t.Errorf("expected error rejecting server flag injection, got %v", err)
	}

	runner := NewSSHRunner()
	opts := SessionOptions{
		Server: "-oProxyCommand=calc",
	}
	err = runner.RunRemoteSession(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "cannot start with '-'") {
		t.Errorf("expected error rejecting server flag injection in runner, got %v", err)
	}
}

type mockProxy struct {
	port    int
	cleanup func()
	err     error
}

func (m *mockProxy) Start() (int, func(), error) {
	return m.port, m.cleanup, m.err
}

type mockSyncer struct {
	calledWith []string
	err        error
}

func (m *mockSyncer) SyncProfile(ctx context.Context, server string, profileName string) error {
	m.calledWith = []string{server, profileName}
	return m.err
}

type mockRunner struct {
	calledOpts *SessionOptions
	err        error
}

func (m *mockRunner) RunRemoteSession(ctx context.Context, opts SessionOptions) error {
	m.calledOpts = &opts
	return m.err
}

func TestService_Execute(t *testing.T) {
	cleaned := false
	proxy := &mockProxy{
		port: 8888,
		cleanup: func() {
			cleaned = true
		},
	}
	syncer := &mockSyncer{}
	runner := &mockRunner{}
	var errBuf bytes.Buffer

	svc := NewService(proxy, syncer, runner, &errBuf)

	err := svc.Execute(context.Background(), "user@box", "/var/app", "work", []string{"--model", "gemini-3.8-flash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cleaned {
		t.Errorf("proxy cleanup was not called")
	}

	if len(syncer.calledWith) != 2 || syncer.calledWith[0] != "user@box" || syncer.calledWith[1] != "work" {
		t.Errorf("syncer called with wrong params: %v", syncer.calledWith)
	}

	if runner.calledOpts == nil || runner.calledOpts.Server != "user@box" || runner.calledOpts.LocalProxyPort != 8888 {
		t.Errorf("runner called with wrong opts: %+v", runner.calledOpts)
	}
}

func TestService_ExecuteProxyFailure(t *testing.T) {
	proxy := &mockProxy{
		err: errors.New("listen failed"),
	}
	svc := NewService(proxy, nil, nil, nil)

	err := svc.Execute(context.Background(), "user@box", "", "work", nil)
	if err == nil || !errors.Is(err, proxy.err) && !strings.Contains(err.Error(), "listen failed") {
		t.Fatalf("expected proxy error, got: %v", err)
	}
}
