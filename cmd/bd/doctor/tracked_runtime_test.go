package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldFlagTrackedFile(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		want bool
	}{
		// Lock files
		{"jsonl lock", ".jsonl.lock", true},
		{"dolt-monitor pid lock", "dolt-monitor.pid.lock", true},
		{"dolt-server lock", "dolt-server.lock", true},
		{"dolt-access lock", "dolt-access.lock", true},

		// Dolt server runtime
		{"dolt-server pid", "dolt-server.pid", true},
		{"dolt-server log", "dolt-server.log", true},
		{"dolt-server port", "dolt-server.port", true},

		// Runtime state
		{"interactions jsonl", "interactions.jsonl", true},
		{"push-state json", "push-state.json", true},
		{"sync-state json", "sync-state.json", true},
		{"last-touched", "last-touched", true},
		{"local version", ".local_version", true},
		{"redirect", "redirect", true},
		{"sync lock", ".sync.lock", true},

		// Ephemeral SQLite
		{"ephemeral sqlite", "ephemeral.sqlite3", true},
		{"ephemeral wal", "ephemeral.sqlite3-wal", true},

		// Dolt directory contents
		{"dolt dir file", "dolt/config.yaml", true},
		{"dolt nested", "dolt/noms/LOCK", true},

		// Backup directory
		{"backup file", "backup/issues.jsonl", true},

		// Export state
		{"export state", "export-state/data.json", true},

		// Corrupt backups
		{"corrupt backup file", "dolt.20260312T123507Z.corrupt.backup/.bd-dolt-ok", true},
		{"corrupt backup config", "dolt.20260312T123507Z.corrupt.backup/config.yaml", true},

		// Sensitive files
		{"credential key", ".beads-credential-key", true},
		{"credential in backup", "dolt.20260312T135310Z.corrupt.backup/.beads-credential-key", true},

		// Files that SHOULD be tracked (not flagged)
		{"gitignore", ".gitignore", false},
		{"readme", "README.md", false},
		{"config yaml", "config.yaml", false},
		{"metadata json", "metadata.json", false},
		{"issues jsonl", "issues.jsonl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFlagTrackedFile(tt.rel)
			if got != tt.want {
				t.Errorf("shouldFlagTrackedFile(%q) = %v, want %v", tt.rel, got, tt.want)
			}
		})
	}
}

func TestCheckTrackedRuntimeFiles_NoGitRepo(t *testing.T) {
	dir := mkTmpDirInTmp(t, "bd-tracked-nogit-*")
	check := CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusOK {
		t.Fatalf("status=%q want %q", check.Status, StatusOK)
	}
	if !strings.Contains(check.Message, "N/A") {
		t.Fatalf("message=%q want N/A", check.Message)
	}
}

func TestCheckTrackedRuntimeFiles_Clean(t *testing.T) {
	dir := mkTmpDirInTmp(t, "bd-tracked-clean-*")
	initRepo(t, dir, "main")

	// Commit only files that should be tracked
	commitFile(t, dir, ".beads/config.yaml", "backend: dolt\n", "add config")
	commitFile(t, dir, ".beads/metadata.json", "{}\n", "add metadata")

	check := CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusOK {
		t.Fatalf("status=%q want %q (msg=%q)", check.Status, StatusOK, check.Message)
	}
}

func TestCheckTrackedRuntimeFiles_RuntimeFiles(t *testing.T) {
	dir := mkTmpDirInTmp(t, "bd-tracked-runtime-*")
	initRepo(t, dir, "main")

	// Commit runtime files that should not be tracked
	commitFile(t, dir, ".beads/config.yaml", "backend: dolt\n", "add config")
	commitFile(t, dir, ".beads/dolt-server.pid", "12345\n", "add server pid")
	commitFile(t, dir, ".beads/dolt-server.log", "log data\n", "add server log")
	commitFile(t, dir, ".beads/.jsonl.lock", "", "add lock")

	check := CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusWarning {
		t.Fatalf("status=%q want %q (msg=%q)", check.Status, StatusWarning, check.Message)
	}
	if !strings.Contains(check.Message, "3") {
		t.Fatalf("message=%q want to mention 3 files", check.Message)
	}
}

func TestCheckTrackedRuntimeFiles_SensitiveFiles(t *testing.T) {
	dir := mkTmpDirInTmp(t, "bd-tracked-sensitive-*")
	initRepo(t, dir, "main")

	// Commit a sensitive file (credential key in corrupt backup)
	backupDir := filepath.Join(dir, ".beads", "dolt.20260312T135310Z.corrupt.backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, ".beads/dolt.20260312T135310Z.corrupt.backup/.beads-credential-key", "secret-key-data", "add credential")

	check := CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusError {
		t.Fatalf("status=%q want %q (msg=%q)", check.Status, StatusError, check.Message)
	}
	if !strings.Contains(check.Message, "sensitive") {
		t.Fatalf("message=%q want to mention sensitive", check.Message)
	}
}

func TestCheckTrackedRuntimeFiles_CorruptBackup(t *testing.T) {
	dir := mkTmpDirInTmp(t, "bd-tracked-corrupt-*")
	initRepo(t, dir, "main")

	backupDir := filepath.Join(dir, ".beads", "dolt.20260312T123507Z.corrupt.backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, ".beads/dolt.20260312T123507Z.corrupt.backup/.bd-dolt-ok", "", "add backup marker")
	commitFile(t, dir, ".beads/dolt.20260312T123507Z.corrupt.backup/config.yaml", "backend: dolt\n", "add backup config")

	check := CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusWarning {
		t.Fatalf("status=%q want %q (msg=%q)", check.Status, StatusWarning, check.Message)
	}
	if !strings.Contains(check.Message, "2") {
		t.Fatalf("message=%q want to mention 2 files", check.Message)
	}
}

func TestFixTrackedRuntimeFiles(t *testing.T) {
	dir := mkTmpDirInTmp(t, "bd-fix-tracked-*")
	initRepo(t, dir, "main")

	// Commit runtime files
	commitFile(t, dir, ".beads/config.yaml", "backend: dolt\n", "add config")
	commitFile(t, dir, ".beads/dolt-server.pid", "12345\n", "add server pid")
	commitFile(t, dir, ".beads/dolt-server.log", "log data\n", "add server log")

	// Verify they're flagged
	check := CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusWarning {
		t.Fatalf("pre-fix status=%q want %q", check.Status, StatusWarning)
	}

	// Fix
	if err := FixTrackedRuntimeFiles(dir); err != nil {
		t.Fatalf("FixTrackedRuntimeFiles: %v", err)
	}

	// Commit the untracking
	runGit(t, dir, "commit", "-m", "untrack runtime files")

	// Verify fix worked
	check = CheckTrackedRuntimeFiles(dir)
	if check.Status != StatusOK {
		t.Fatalf("post-fix status=%q want %q (msg=%q)", check.Status, StatusOK, check.Message)
	}

	// Verify local files still exist
	if _, err := os.Stat(filepath.Join(dir, ".beads", "dolt-server.pid")); os.IsNotExist(err) {
		t.Fatal("dolt-server.pid should still exist locally after untracking")
	}
}
