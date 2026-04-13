package configfile

import (
	"os"
	"path/filepath"
	"runtime"
)

// CentralConfigFileName is the filename for the central/global server config.
const CentralConfigFileName = "server.json"

// DefaultCentralConfigPath returns the platform-appropriate path for the
// central beads server config.
// Linux/macOS: ~/.config/beads/server.json
// Windows: %APPDATA%\beads\server.json
//
// This follows the same convention as DefaultCredentialsPath.
func DefaultCentralConfigPath() string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "beads", CentralConfigFileName)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "beads", CentralConfigFileName)
}

// LoadCentralConfig reads the central server config from the given path.
// Returns (nil, nil) if the file does not exist (graceful skip).
// Returns an error only if the file exists but cannot be parsed.
func LoadCentralConfig(path string) (*Config, error) {
	return nil, nil
}

// ApplyCentralDefaults merges server-related fields from the central config
// into the per-project config, but only for fields that are at their zero
// value in the project config. Non-server fields (Database, DoltDatabase,
// DoltDataDir, ProjectID, etc.) are never inherited from the central config
// because they are inherently per-project.
//
// This is a no-op if central is nil.
func ApplyCentralDefaults(project *Config, central *Config) {
}
