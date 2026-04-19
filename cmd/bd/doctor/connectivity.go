package doctor

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/steveyegge/beads/internal/config"
	"github.com/steveyegge/beads/internal/configfile"
	"github.com/steveyegge/beads/internal/doltserver"
	"github.com/steveyegge/beads/internal/storage/doltutil"
)

// ConnectivitySource represents a single config source in the resolution chain.
type ConnectivitySource struct {
	Name   string `json:"name"`
	Value  string `json:"value"`  // The value from this source, or "(not set)"
	Winner bool   `json:"winner"` // True if this source provided the resolved value
	Source string `json:"source"` // e.g., "env", "port-file", "config.yaml", "metadata.json", "default"
}

// ConnectivityResult holds the full Dolt connection resolution report.
type ConnectivityResult struct {
	HostSources  []ConnectivitySource `json:"host_sources"`
	PortSources  []ConnectivitySource `json:"port_sources"`
	ResolvedHost string               `json:"resolved_host"`
	ResolvedPort int                  `json:"resolved_port"`
	ResolvedDSN  string               `json:"resolved_dsn"` // host:port
	Connected    bool                 `json:"connected"`
	Error        string               `json:"error,omitempty"`
	ServerMode   string               `json:"server_mode"`
	Database     string               `json:"database"`
}

// RunConnectivityCheck performs a full Dolt connection resolution diagnostic.
// It reports every config source, which one wins, and tests the actual connection.
func RunConnectivityCheck(repoPath string) *ConnectivityResult {
	beadsDir := ResolveBeadsDirForRepo(repoPath)
	return runConnectivityCheckForDir(beadsDir)
}

func runConnectivityCheckForDir(beadsDir string) *ConnectivityResult {
	result := &ConnectivityResult{}

	// Load metadata.json
	fileCfg, _ := configfile.Load(beadsDir)
	if fileCfg == nil {
		fileCfg = configfile.DefaultConfig()
	}

	// --- Host resolution ---
	// Priority: BEADS_DOLT_SERVER_HOST env > metadata.json dolt_server_host > default
	result.HostSources = resolveHostSources(fileCfg)
	result.ResolvedHost = fileCfg.GetDoltServerHost()

	// Mark the winner
	markHostWinner(result)

	// --- Port resolution ---
	// Priority: BEADS_DOLT_SERVER_PORT env > port file > config.yaml dolt.port > metadata.json dolt_server_port > default/shared
	result.PortSources = resolvePortSources(beadsDir, fileCfg)

	// Use doltserver.DefaultConfig for the authoritative resolved port
	dsCfg := doltserver.DefaultConfig(beadsDir)
	result.ResolvedPort = dsCfg.Port
	result.ServerMode = dsCfg.Mode.String()

	// Mark the winner
	markPortWinner(result)

	// Database
	result.Database = fileCfg.GetDoltDatabase()

	// Resolved DSN
	if result.ResolvedPort > 0 {
		result.ResolvedDSN = fmt.Sprintf("%s:%d", result.ResolvedHost, result.ResolvedPort)
	} else {
		result.ResolvedDSN = fmt.Sprintf("%s:(no port)", result.ResolvedHost)
	}

	// --- Test connection ---
	if result.ResolvedPort == 0 {
		result.Connected = false
		result.Error = "no port configured and no server running"
		return result
	}

	connStr := doltutil.ServerDSN{
		Host:     result.ResolvedHost,
		Port:     result.ResolvedPort,
		User:     fileCfg.GetDoltServerUser(),
		Password: os.Getenv("BEADS_DOLT_PASSWORD"),
		Database: result.Database,
		TLS:      fileCfg.GetDoltServerTLS(),
	}.String()

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		result.Connected = false
		result.Error = fmt.Sprintf("failed to open connection: %v", err)
		return result
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		result.Connected = false
		result.Error = fmt.Sprintf("ping failed: %v", err)
		return result
	}

	result.Connected = true
	return result
}

func resolveHostSources(fileCfg *configfile.Config) []ConnectivitySource {
	var sources []ConnectivitySource

	// 1. BEADS_DOLT_SERVER_HOST env var
	envHost := os.Getenv("BEADS_DOLT_SERVER_HOST")
	sources = append(sources, ConnectivitySource{
		Name:   "env BEADS_DOLT_SERVER_HOST",
		Value:  valueOrNotSet(envHost),
		Source: "env",
	})

	// 2. metadata.json dolt_server_host
	metaHost := fileCfg.DoltServerHost
	sources = append(sources, ConnectivitySource{
		Name:   "metadata.json dolt_server_host",
		Value:  valueOrNotSet(metaHost),
		Source: "metadata.json",
	})

	// 3. Default
	sources = append(sources, ConnectivitySource{
		Name:   "default",
		Value:  configfile.DefaultDoltServerHost,
		Source: "default",
	})

	return sources
}

func resolvePortSources(beadsDir string, fileCfg *configfile.Config) []ConnectivitySource {
	var sources []ConnectivitySource

	// 1. BEADS_DOLT_SERVER_PORT env var
	envPort := os.Getenv("BEADS_DOLT_SERVER_PORT")
	sources = append(sources, ConnectivitySource{
		Name:   "env BEADS_DOLT_SERVER_PORT",
		Value:  valueOrNotSet(envPort),
		Source: "env",
	})

	// 2. dolt-server.port file
	portFileDir := beadsDir
	if doltserver.IsSharedServerMode() {
		if sharedDir, err := doltserver.SharedServerDir(); err == nil {
			portFileDir = sharedDir
		}
	}
	portFileVal := doltserver.ReadPortFile(portFileDir)
	portFileStr := "(not set)"
	if portFileVal > 0 {
		portFileStr = strconv.Itoa(portFileVal)
	}
	sources = append(sources, ConnectivitySource{
		Name:   "dolt-server.port file",
		Value:  portFileStr,
		Source: "port-file",
	})

	// 3. config.yaml dolt.port
	yamlPort := config.GetYamlConfig("dolt.port")
	sources = append(sources, ConnectivitySource{
		Name:   "config.yaml dolt.port",
		Value:  valueOrNotSet(yamlPort),
		Source: "config.yaml",
	})

	// 4. metadata.json dolt_server_port
	metaPort := "(not set)"
	if fileCfg.DoltServerPort > 0 {
		metaPort = strconv.Itoa(fileCfg.DoltServerPort)
	}
	sources = append(sources, ConnectivitySource{
		Name:   "metadata.json dolt_server_port",
		Value:  metaPort,
		Source: "metadata.json",
	})

	// 5. Shared server default port
	sharedDefault := "(not applicable)"
	if doltserver.IsSharedServerMode() {
		sharedDefault = strconv.Itoa(doltserver.DefaultSharedServerPort)
	}
	sources = append(sources, ConnectivitySource{
		Name:   "shared server default",
		Value:  sharedDefault,
		Source: "default",
	})

	return sources
}

func markHostWinner(result *ConnectivityResult) {
	resolved := result.ResolvedHost
	for i := range result.HostSources {
		s := &result.HostSources[i]
		if s.Value != "(not set)" && s.Value == resolved {
			s.Winner = true
			return
		}
	}
}

func markPortWinner(result *ConnectivityResult) {
	if result.ResolvedPort == 0 {
		return
	}
	resolved := strconv.Itoa(result.ResolvedPort)
	for i := range result.PortSources {
		s := &result.PortSources[i]
		if s.Value != "(not set)" && s.Value != "(not applicable)" && s.Value == resolved {
			s.Winner = true
			return
		}
	}
}

func valueOrNotSet(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}

// FormatConnectivityReport produces a human-readable report from a ConnectivityResult.
func FormatConnectivityReport(r *ConnectivityResult) string {
	var b strings.Builder

	b.WriteString("Dolt connection resolution:\n")
	b.WriteString("  Host:\n")
	for _, s := range r.HostSources {
		marker := "  "
		if s.Winner {
			marker = "<-"
		}
		b.WriteString(fmt.Sprintf("    %-40s %s %s\n", s.Name+":", s.Value, marker))
	}

	b.WriteString("  Port:\n")
	for _, s := range r.PortSources {
		marker := "  "
		if s.Winner {
			marker = "<-"
		}
		b.WriteString(fmt.Sprintf("    %-40s %s %s\n", s.Name+":", s.Value, marker))
	}

	b.WriteString(fmt.Sprintf("  Server mode: %s\n", r.ServerMode))
	b.WriteString(fmt.Sprintf("  Database:    %s\n", r.Database))
	b.WriteString(fmt.Sprintf("  Resolved:    %s\n", r.ResolvedDSN))

	if r.Connected {
		b.WriteString("  Status:      connected\n")
	} else {
		b.WriteString(fmt.Sprintf("  Status:      FAILED (%s)\n", r.Error))
	}

	return b.String()
}
