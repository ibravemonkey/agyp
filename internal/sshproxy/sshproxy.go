package sshproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

// Proxy abstracts local HTTP proxy lifecycle.
type Proxy interface {
	Start() (port int, cleanup func(), err error)
}

// Syncer abstracts copying profile tokens and settings to a remote SSH host.
type Syncer interface {
	SyncProfile(ctx context.Context, server string, profileName string) error
}

// SessionRunner abstracts running an interactive remote SSH command.
type SessionRunner interface {
	RunRemoteSession(ctx context.Context, opts SessionOptions) error
}

// SessionOptions encapsulates parameters for remote SSH execution.
type SessionOptions struct {
	Server        string
	RemotePath    string
	TargetProfile string
	AgyArgs       []string
	LocalProxyPort int
	RemotePort    int
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
}

// Service orchestrates SSH tunneling, profile synchronization, and execution.
type Service interface {
	Execute(ctx context.Context, server, remotePath, targetProfile string, agyArgs []string) error
}

type defaultService struct {
	proxy   Proxy
	syncer  Syncer
	runner  SessionRunner
	errOut  io.Writer
}

// NewService creates an SSH service with the provided components.
func NewService(proxy Proxy, syncer Syncer, runner SessionRunner, errOut io.Writer) Service {
	if proxy == nil {
		proxy = NewLocalHTTPProxy()
	}
	if syncer == nil {
		syncer = NewSSHProfileSyncer()
	}
	if runner == nil {
		runner = NewSSHRunner()
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	return &defaultService{
		proxy:  proxy,
		syncer: syncer,
		runner: runner,
		errOut: errOut,
	}
}

func (s *defaultService) Execute(ctx context.Context, server, remotePath, targetProfile string, agyArgs []string) error {
	// 1. Start local HTTP proxy
	proxyPort, cleanupProxy, err := s.proxy.Start()
	if err != nil {
		return fmt.Errorf("failed to start local HTTP proxy: %w", err)
	}
	defer cleanupProxy()

	// 2. Sync profile
	fmt.Fprintf(s.errOut, "[agyp] Syncing local profile %q to %s...\n", targetProfile, server)
	if err := s.syncer.SyncProfile(ctx, server, targetProfile); err != nil {
		return err
	}

	// 3. Run remote session
	remotePort := 10800 + (os.Getpid() % 1000)
	opts := SessionOptions{
		Server:         server,
		RemotePath:     remotePath,
		TargetProfile:  targetProfile,
		AgyArgs:        agyArgs,
		LocalProxyPort: proxyPort,
		RemotePort:     remotePort,
		Stdin:          os.Stdin,
		Stdout:         os.Stdout,
		Stderr:         s.errOut,
	}
	return s.runner.RunRemoteSession(ctx, opts)
}

// LocalHTTPProxy implements Proxy.
type LocalHTTPProxy struct{}

// NewLocalHTTPProxy creates a new LocalHTTPProxy.
func NewLocalHTTPProxy() *LocalHTTPProxy {
	return &LocalHTTPProxy{}
}

// Start spawns a background HTTP CONNECT proxy on a random localhost port.
func (p *LocalHTTPProxy) Start() (int, func(), error) {
	return StartLocalHTTPProxy()
}

// StartLocalHTTPProxy is a standalone helper that creates an HTTP CONNECT tunnel listener.
func StartLocalHTTPProxy() (int, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, func() {}, err
	}
	port := listener.Addr().(*net.TCPAddr).Port

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				destConn, dialErr := net.DialTimeout("tcp", r.Host, 10*time.Second)
				if dialErr != nil {
					http.Error(w, dialErr.Error(), http.StatusServiceUnavailable)
					return
				}
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
					destConn.Close()
					return
				}
				clientConn, bufrw, hijackErr := hijacker.Hijack()
				if hijackErr != nil {
					destConn.Close()
					return
				}
				_, _ = bufrw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
				_ = bufrw.Flush()

				go func() {
					_, _ = io.Copy(destConn, bufrw)
					destConn.Close()
				}()
				go func() {
					_, _ = io.Copy(clientConn, destConn)
					clientConn.Close()
				}()
			} else {
				req, reqErr := http.NewRequest(r.Method, r.URL.String(), r.Body)
				if reqErr != nil {
					http.Error(w, reqErr.Error(), http.StatusBadRequest)
					return
				}
				req.Header = r.Header
				resp, doErr := http.DefaultClient.Do(req)
				if doErr != nil {
					http.Error(w, doErr.Error(), http.StatusBadGateway)
					return
				}
				defer resp.Body.Close()
				for k, vv := range resp.Header {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(resp.StatusCode)
				_, _ = io.Copy(w, resp.Body)
			}
		}),
	}

	go func() {
		_ = server.Serve(listener)
	}()

	cleanup := func() {
		_ = server.Close()
		_ = listener.Close()
	}

	return port, cleanup, nil
}

// SSHProfileSyncer implements Syncer using scp/ssh.
type SSHProfileSyncer struct{}

// NewSSHProfileSyncer creates a new SSHProfileSyncer.
func NewSSHProfileSyncer() *SSHProfileSyncer {
	return &SSHProfileSyncer{}
}

// SyncProfile copies essential tokens and settings from local to remote.
func (s *SSHProfileSyncer) SyncProfile(ctx context.Context, server string, profileName string) error {
	localProfileDir, err := profile.GetProfileDir(profileName)
	if err != nil {
		return fmt.Errorf("failed to get local profile directory for %q: %w", profileName, err)
	}

	remoteCliDir := fmt.Sprintf("~/.agyp/profiles/%s/.gemini/antigravity-cli", profileName)

	mkdirCmd := exec.CommandContext(ctx, "ssh", server,
		fmt.Sprintf("mkdir -p %s && test -f %s/settings.json", remoteCliDir, remoteCliDir))
	settingsExists := mkdirCmd.Run() == nil

	localCliDir := filepath.Join(localProfileDir, ".gemini", "antigravity-cli")

	alwaysSyncFiles := []string{
		"antigravity-oauth-token",
		"jetski-standalone-oauth-token",
		"jetski_state.pbtxt",
		"installation_id",
	}

	if !settingsExists {
		alwaysSyncFiles = append(alwaysSyncFiles, "settings.json")
	}

	var filesToSync []string
	for _, file := range alwaysSyncFiles {
		localPath := filepath.Join(localCliDir, file)
		if info, statErr := os.Stat(localPath); statErr == nil && info.Size() > 0 {
			filesToSync = append(filesToSync, localPath)
		}
	}

	if len(filesToSync) > 0 {
		args := append([]string{"-q"}, filesToSync...)
		args = append(args, fmt.Sprintf("%s:%s/", server, remoteCliDir))
		_ = exec.CommandContext(ctx, "scp", args...).Run()
		_ = exec.CommandContext(ctx, "ssh", server, fmt.Sprintf("chmod 600 %s/*token* 2>/dev/null || true", remoteCliDir)).Run()
	}

	return nil
}

// SSHRunner executes remote commands over SSH with PTY allocation and port forwarding.
type SSHRunner struct{}

// NewSSHRunner creates a new SSHRunner.
func NewSSHRunner() *SSHRunner {
	return &SSHRunner{}
}

// RunRemoteSession sets up remote SSH execution.
func (r *SSHRunner) RunRemoteSession(ctx context.Context, opts SessionOptions) error {
	var agyArgsStr string
	if len(opts.AgyArgs) > 0 {
		var quoted []string
		for _, arg := range opts.AgyArgs {
			quoted = append(quoted, ShellQuote(arg))
		}
		agyArgsStr = " -- " + strings.Join(quoted, " ")
	}

	agysRunCmd := fmt.Sprintf("agyp run %s", ShellQuote(opts.TargetProfile))
	cdPrefix := ""
	if opts.RemotePath != "" {
		cdPrefix = fmt.Sprintf("cd %s && ", ShellQuote(opts.RemotePath))
	}

	proxyEnv := fmt.Sprintf("export HTTP_PROXY=http://127.0.0.1:%d HTTPS_PROXY=http://127.0.0.1:%d http_proxy=http://127.0.0.1:%d https_proxy=http://127.0.0.1:%d ALL_PROXY=http://127.0.0.1:%d all_proxy=http://127.0.0.1:%d;",
		opts.RemotePort, opts.RemotePort, opts.RemotePort, opts.RemotePort, opts.RemotePort, opts.RemotePort)
	sshEnv := fmt.Sprintf("export AGYP_SSH_SERVER=%s; export AGYP_SSH_PATH=%s;", ShellQuote(opts.Server), ShellQuote(opts.RemotePath))

	innerCmd := fmt.Sprintf(
		`export PATH="$HOME/.local/bin:$HOME/bin:$HOME/go/bin:$HOME/.gemini/antigravity-cli/bin:/usr/local/bin:/opt/homebrew/bin:$PATH"; `+
			`%s`+
			`%s`+
			`if ! command -v agy >/dev/null 2>&1; then `+
			`echo "[agyp] Auto-installing agy (Antigravity CLI) on %s (downloading ~70MB release package over SSH tunnel, please wait)..." >&2; `+
			`curl -fsSL https://antigravity.google/cli/install.sh | bash || true; `+
			`export PATH="$HOME/.local/bin:$HOME/bin:$HOME/go/bin:$HOME/.gemini/antigravity-cli/bin:/usr/local/bin:/opt/homebrew/bin:$PATH"; `+
			`fi; `+
			`if ! command -v agyp >/dev/null 2>&1; then `+
			`echo "[agyp] Auto-installing agyp (profile switcher) on %s..." >&2; `+
			`curl -fsSL https://raw.githubusercontent.com/quaywin/agyp/main/install.sh | bash || true; `+
			`export PATH="$HOME/.local/bin:$HOME/bin:$HOME/go/bin:$HOME/.gemini/antigravity-cli/bin:/usr/local/bin:/opt/homebrew/bin:$PATH"; `+
			`fi; `+
			`%sif command -v agyp >/dev/null 2>&1; then exec %s%s; `+
			`elif command -v agy >/dev/null 2>&1; then exec agy%s; `+
			`else `+
			`echo "[agyp] Error: Unable to locate agy or agyp on %s." >&2; exit 127; `+
			`fi`,
		proxyEnv, sshEnv, opts.Server, opts.Server, cdPrefix, agysRunCmd, agyArgsStr, agyArgsStr, opts.Server,
	)

	remoteCmd := fmt.Sprintf("sh -c %s", ShellQuote(innerCmd))

	if opts.RemotePath != "" {
		fmt.Fprintf(opts.Stderr, "[agyp] Connecting to %s (%s) over SSH with PTY (API tunnel active)... \n", opts.Server, opts.RemotePath)
	} else {
		fmt.Fprintf(opts.Stderr, "[agyp] Connecting to %s over SSH with PTY (API tunnel active)...\n", opts.Server)
	}

	if profile.IsInHerdrEnvironment() {
		profile.SetTerminalTitle(fmt.Sprintf("%s (%s)", opts.TargetProfile, opts.Server))
		_ = profile.ReportHerdrMetadata(ctx, opts.TargetProfile)
		defer func() {
			_ = profile.ClearHerdrMetadata(context.Background())
		}()
	}

	sshExecCmd := exec.CommandContext(ctx, "ssh", "-R", fmt.Sprintf("%d:127.0.0.1:%d", opts.RemotePort, opts.LocalProxyPort), "-t", opts.Server, remoteCmd)
	if opts.Stdin != nil {
		sshExecCmd.Stdin = opts.Stdin
	}
	if opts.Stdout != nil {
		sshExecCmd.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		sshExecCmd.Stderr = opts.Stderr
	}

	sigCtx, stopSignal := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stopSignal()

	if err := sshExecCmd.Start(); err != nil {
		return fmt.Errorf("failed to start SSH connection: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- sshExecCmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-sigCtx.Done():
		if sshExecCmd.Process != nil {
			_ = sshExecCmd.Process.Signal(syscall.SIGTERM)
		}
		return <-done
	}
}

// ShellQuote safely wraps argument in single quotes for Unix shell execution.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\r\"'\\$`<>|&;()") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
