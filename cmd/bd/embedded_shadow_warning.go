package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// warnShadowedEmbeddedDolt warns when a server-mode workspace also has a
// .beads/embeddeddolt directory. The server is authoritative in that case; the
// embedded copy is never read or written and is likely a stray from an older
// bootstrap.
func warnShadowedEmbeddedDolt(w io.Writer, beadsDir string) {
	dir := filepath.Join(beadsDir, "embeddeddolt")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		fmt.Fprintf(w, "warning: %s exists but this workspace is in dolt server mode; using the Dolt server and ignoring the embedded copy (remove the directory if it is stale)\n", dir)
	}
}
