package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConnectivityCheck_NoBeadsDir(t *testing.T) {
	tmpDir := t.TempDir()
	result := RunConnectivityCheck(tmpDir)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should report host sources
	if len(result.HostSources) == 0 {
		t.Error("expected host sources")
	}

	// Should report port sources
	if len(result.PortSources) == 0 {
		t.Error("expected port sources")
	}

	// Should have a resolved host (at least the default)
	if result.ResolvedHost == "" {
		t.Error("expected resolved host")
	}

	// Without a running server, should not be connected
	if result.Connected {
		t.Error("expected not connected without a server")
	}
}

func TestRunConnectivityCheck_WithMetadataJSON(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write a metadata.json with explicit host
	metadata := `{"dolt_server_host": "10.0.0.1", "dolt_server_port": 3309}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}

	result := RunConnectivityCheck(tmpDir)

	// metadata.json host should appear in sources
	found := false
	for _, s := range result.HostSources {
		if s.Name == "metadata.json dolt_server_host" && s.Value == "10.0.0.1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected metadata.json host source with value 10.0.0.1")
	}

	// metadata.json port should appear in sources
	foundPort := false
	for _, s := range result.PortSources {
		if s.Name == "metadata.json dolt_server_port" && s.Value == "3309" {
			foundPort = true
			break
		}
	}
	if !foundPort {
		t.Error("expected metadata.json port source with value 3309")
	}
}

func TestRunConnectivityCheck_EnvVarOverride(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write metadata.json with one host
	metadata := `{"dolt_server_host": "10.0.0.1"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}

	// Set env var to override
	t.Setenv("BEADS_DOLT_SERVER_HOST", "192.168.1.100")

	result := RunConnectivityCheck(tmpDir)

	// Env var should win
	if result.ResolvedHost != "192.168.1.100" {
		t.Errorf("expected resolved host 192.168.1.100, got %s", result.ResolvedHost)
	}

	// Env source should be marked winner
	for _, s := range result.HostSources {
		if s.Name == "env BEADS_DOLT_SERVER_HOST" {
			if !s.Winner {
				t.Error("expected env var source to be winner")
			}
			if s.Value != "192.168.1.100" {
				t.Errorf("expected env value 192.168.1.100, got %s", s.Value)
			}
		}
	}
}

func TestRunConnectivityCheck_PortFileWins(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write metadata.json with an explicit port
	metadata := `{"dolt_server_host": "127.0.0.1"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}

	// Write port file
	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.port"), []byte("54321"), 0600); err != nil {
		t.Fatal(err)
	}

	// Clear env var to avoid interference
	t.Setenv("BEADS_DOLT_SERVER_PORT", "")

	result := RunConnectivityCheck(tmpDir)

	// Port file should provide the resolved port
	if result.ResolvedPort != 54321 {
		t.Errorf("expected resolved port 54321, got %d", result.ResolvedPort)
	}

	// Port file source should be marked winner
	for _, s := range result.PortSources {
		if s.Name == "dolt-server.port file" {
			if !s.Winner {
				t.Error("expected port file source to be winner")
			}
		}
	}
}

func TestFormatConnectivityReport(t *testing.T) {
	result := &ConnectivityResult{
		HostSources: []ConnectivitySource{
			{Name: "env BEADS_DOLT_SERVER_HOST", Value: "192.168.86.29", Winner: true, Source: "env"},
			{Name: "metadata.json dolt_server_host", Value: "127.0.0.1", Source: "metadata.json"},
			{Name: "default", Value: "127.0.0.1", Source: "default"},
		},
		PortSources: []ConnectivitySource{
			{Name: "env BEADS_DOLT_SERVER_PORT", Value: "(not set)", Source: "env"},
			{Name: "dolt-server.port file", Value: "3307", Winner: true, Source: "port-file"},
			{Name: "config.yaml dolt.port", Value: "(not set)", Source: "config.yaml"},
			{Name: "metadata.json dolt_server_port", Value: "(not set)", Source: "metadata.json"},
			{Name: "shared server default", Value: "(not applicable)", Source: "default"},
		},
		ResolvedHost: "192.168.86.29",
		ResolvedPort: 3307,
		ResolvedDSN:  "192.168.86.29:3307",
		Connected:    true,
		ServerMode:   "owned",
		Database:     "beads",
	}

	report := FormatConnectivityReport(result)

	// Check key elements are present
	if !strings.Contains(report, "Dolt connection resolution:") {
		t.Error("expected header in report")
	}
	if !strings.Contains(report, "192.168.86.29") {
		t.Error("expected resolved host in report")
	}
	if !strings.Contains(report, "3307") {
		t.Error("expected resolved port in report")
	}
	if !strings.Contains(report, "<-") {
		t.Error("expected winner marker in report")
	}
	if !strings.Contains(report, "connected") {
		t.Error("expected connected status in report")
	}
}

func TestFormatConnectivityReport_Failed(t *testing.T) {
	result := &ConnectivityResult{
		HostSources:  []ConnectivitySource{{Name: "default", Value: "127.0.0.1", Winner: true, Source: "default"}},
		PortSources:  []ConnectivitySource{{Name: "env BEADS_DOLT_SERVER_PORT", Value: "(not set)", Source: "env"}},
		ResolvedHost: "127.0.0.1",
		ResolvedPort: 0,
		ResolvedDSN:  "127.0.0.1:(no port)",
		Connected:    false,
		Error:        "no port configured and no server running",
		ServerMode:   "owned",
		Database:     "beads",
	}

	report := FormatConnectivityReport(result)
	if !strings.Contains(report, "FAILED") {
		t.Error("expected FAILED in report")
	}
}

func TestConnectivityResultJSON(t *testing.T) {
	result := &ConnectivityResult{
		HostSources: []ConnectivitySource{
			{Name: "env BEADS_DOLT_SERVER_HOST", Value: "(not set)", Source: "env"},
			{Name: "default", Value: "127.0.0.1", Winner: true, Source: "default"},
		},
		PortSources:  []ConnectivitySource{},
		ResolvedHost: "127.0.0.1",
		ResolvedPort: 3307,
		ResolvedDSN:  "127.0.0.1:3307",
		Connected:    false,
		Error:        "connection refused",
		ServerMode:   "owned",
		Database:     "beads",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	var decoded ConnectivityResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if decoded.ResolvedHost != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", decoded.ResolvedHost)
	}
	if decoded.Error != "connection refused" {
		t.Errorf("expected error 'connection refused', got %s", decoded.Error)
	}
	if len(decoded.HostSources) != 2 {
		t.Errorf("expected 2 host sources, got %d", len(decoded.HostSources))
	}
}

func TestValueOrNotSet(t *testing.T) {
	if valueOrNotSet("") != "(not set)" {
		t.Error("expected (not set) for empty string")
	}
	if valueOrNotSet("hello") != "hello" {
		t.Error("expected hello for non-empty string")
	}
}
