package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
	proxycmd "github.com/edgelesssys/continuum/privatemode-proxy/cmd"
)

const (
	defaultProxyWorkspace = "./privatemode-proxy-data"
	defaultProxyLogLevel  = "error"
	proxyReadyTimeout     = 30 * time.Second
)

// ManagedProxy controls the lifecycle of an in-process privatemode-proxy instance.
type ManagedProxy struct {
	cancel  context.CancelFunc
	errCh   chan error
	LogPath string
}

// StartManagedProxy starts privatemode-proxy in-process, using the cobra command from the
// vendored privatemode-proxy module, and blocks until it responds on its models endpoint.
func StartManagedProxy(parent context.Context, cfg *config.Config) (*ManagedProxy, error) {
	port, err := proxyPort(cfg.LLM.API.BaseURL)
	if err != nil {
		return nil, err
	}

	workspace := cfg.LLM.Proxy.Workspace
	if workspace == "" {
		workspace = defaultProxyWorkspace
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create proxy workspace %q: %w", workspace, err)
	}

	logLevel := cfg.LLM.Proxy.LogLevel
	if logLevel == "" {
		logLevel = defaultProxyLogLevel
	}

	logPath := filepath.Join(workspace, "privatemode-proxy.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open proxy log file %q: %w", logPath, err)
	}

	proxyCmd := proxycmd.New()
	proxyCmd.SetArgs([]string{
		"--apiKey", cfg.LLM.Proxy.APIKey,
		"--port", port,
		"--workspace", workspace,
		"--log-level", logLevel,
		"--log-format", "json",
	})
	// Redirect the proxy's own logging away from our pterm-driven CLI output.
	proxyCmd.SetOut(logFile)
	proxyCmd.SetErr(logFile)

	ctx, cancel := context.WithCancel(parent)
	errCh := make(chan error, 1)
	go func() {
		defer logFile.Close()
		errCh <- proxyCmd.ExecuteContext(ctx)
	}()

	if err := waitForProxyReady(cfg.LLM.API.BaseURL, errCh, proxyReadyTimeout); err != nil {
		cancel()
		return nil, err
	}

	return &ManagedProxy{cancel: cancel, errCh: errCh, LogPath: logPath}, nil
}

// Stop cancels the proxy's context and waits for it to shut down, up to timeout.
func (p *ManagedProxy) Stop(timeout time.Duration) error {
	p.cancel()
	select {
	case err := <-p.errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for privatemode-proxy to stop")
	}
}

func proxyPort(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid llm.api.base_url %q: %w", baseURL, err)
	}
	if port := u.Port(); port != "" {
		return port, nil
	}
	return "8080", nil
}

// waitForProxyReady polls the proxy's models endpoint until it responds, the startup fails, or timeout elapses.
func waitForProxyReady(baseURL string, errCh <-chan error, timeout time.Duration) error {
	modelsURL := strings.TrimSuffix(baseURL, "/") + "/v1/models"
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-errCh:
			if err == nil {
				err = errors.New("privatemode-proxy exited before becoming ready")
			}
			return fmt.Errorf("privatemode-proxy failed to start: %w", err)
		case <-deadline:
			return fmt.Errorf("timed out after %s waiting for privatemode-proxy to become ready", timeout)
		case <-ticker.C:
			resp, err := client.Get(modelsURL)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}
