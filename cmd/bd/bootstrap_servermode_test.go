//go:build cgo

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/beads/internal/configfile"
)

// Regression test for robots-jfqb: bootstrap on a dolt_mode=server workspace
// must open the Dolt server, not an embedded store under .beads/embeddeddolt/.
func TestBootstrapDoltConfig_HonorsServerMode(t *testing.T) {
	restore := snapshotBootstrapEnv(t)
	defer restore()

	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	server := &configfile.Config{Backend: configfile.BackendDolt, DoltMode: configfile.DoltModeServer, DoltDatabase: "travel"}
	got := bootstrapDoltConfig(beadsDir, server, "travel")
	if !got.ServerMode {
		t.Fatal("server-mode metadata must set ServerMode so bootstrap does not create embeddeddolt")
	}
	if got.Database != "travel" || !got.CreateIfMissing {
		t.Fatalf("unexpected config: %+v", got)
	}

	embedded := &configfile.Config{Backend: configfile.BackendDolt, DoltMode: configfile.DoltModeEmbedded}
	if bootstrapDoltConfig(beadsDir, embedded, "beads").ServerMode {
		t.Fatal("embedded metadata must not set ServerMode")
	}
}

func TestWarnShadowedEmbeddedDolt(t *testing.T) {
	beadsDir := t.TempDir()
	var buf bytes.Buffer
	warnShadowedEmbeddedDolt(&buf, beadsDir)
	if buf.Len() != 0 {
		t.Fatalf("no warning expected without embeddeddolt dir, got %q", buf.String())
	}
	if err := os.MkdirAll(filepath.Join(beadsDir, "embeddeddolt", "travel", ".dolt"), 0o750); err != nil {
		t.Fatal(err)
	}
	warnShadowedEmbeddedDolt(&buf, beadsDir)
	if !strings.Contains(buf.String(), "dolt server mode") {
		t.Fatalf("expected shadow warning, got %q", buf.String())
	}
}
