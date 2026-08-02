//go:build integration
// +build integration

package internal

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	cfg "github.com/dhcgn/paperless-ngx-privatemode-ai/config"
)

// TestStartManagedProxy starts a real privatemode-proxy in-process and verifies it becomes reachable.
// Requires PRIVATE_MODE_API_KEY to be set; skipped otherwise.
func TestStartManagedProxy(t *testing.T) {
	apiKey := os.Getenv("PRIVATE_MODE_API_KEY")
	if apiKey == "" {
		t.Skip("PRIVATE_MODE_API_KEY not set, skipping managed proxy integration test")
	}

	config := &cfg.Config{
		LLM: cfg.LLMConfig{
			API: struct {
				BaseURL     string `yaml:"base_url"`
				Endpoint    string `yaml:"endpoint"`
				Timeout     int    `yaml:"timeout"`
				Debug       bool   `yaml:"debug"`
				DebugFolder string `yaml:"debug_folder"`
			}{
				BaseURL: "http://localhost:18080", // distinct port so this doesn't clash with a manually run proxy on 8080
			},
			Proxy: cfg.ProxyConfig{
				APIKey:    apiKey,
				Workspace: t.TempDir(),
			},
		},
	}

	proxy, err := StartManagedProxy(context.Background(), config)
	if err != nil {
		t.Fatalf("StartManagedProxy() error = %v", err)
	}
	t.Logf("privatemode-proxy log file: %s", proxy.LogPath)

	resp, err := http.Get(strings.TrimSuffix(config.LLM.API.BaseURL, "/") + "/v1/models")
	if err != nil {
		t.Fatalf("failed to reach managed proxy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code from managed proxy: %d", resp.StatusCode)
	}

	if err := proxy.Stop(10 * time.Second); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
