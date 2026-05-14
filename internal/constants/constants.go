package constants

import (
	"os"
	"time"
)

// Default Paths
const (
	DefaultSocketPath = "/run/tunneld/tunneld.sock"
	DefaultKeyDir     = "/var/lib/tunneld/keys"
	DefaultTunnelsDir = "/etc/tunneld/tunnels.d"
	DefaultConfigFile = "tunnels.yaml"
	LogDirPrefix      = "tunneld-logs"
)

// Network Defaults
const (
	DefaultListenAddress = "127.0.0.1"
)

// Timeouts and Intervals
const (
	DefaultStartupTimeout  = 30 * time.Second
	DefaultShutdownTimeout = 5 * time.Second
	DefaultHealthInterval  = 2 * time.Second
	DefaultHealthTimeout   = 2 * time.Second
	DefaultWaitTimeout     = 30 * time.Second
	DefaultGRPCRequestTimeout = 2 * time.Second
)

// File and Directory Permissions
const (
	PermDirPrivate    os.FileMode = 0700
	PermDirPublic     os.FileMode = 0755
	PermFilePrivate   os.FileMode = 0600
	PermFilePublic    os.FileMode = 0644
	PermSocketPrivate os.FileMode = 0660
)
