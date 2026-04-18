package options

import (
	"os"
	"strconv"
	"strings"
)

// Options holds all configuration for the Proxmox provider.
type Options struct {
	// Proxmox connection
	Host string
	Port int
	User string
	Token string
	Node string

	// Proxmox resources
	Template string
	Storage string
	Network string
	VMStartID int

	// VM specs
	Memory int
	Cores int
	DiskSize int
	OSType string

	// DevPod-injected
	MachineID string
	MachineFolder string
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		Port:      8006,
		Storage:   "local-lvm",
		Network:   "vmbr0",
		VMStartID: 2000,
		Memory:    4096,
		Cores:     2,
		DiskSize:  50,
		OSType:    "l26",
	}
}

// FromEnv populates Options from environment variables, overriding defaults.
func FromEnv() Options {
	opts := DefaultOptions()

	if v := os.Getenv("PROXMOX_HOST"); v != "" {
		opts.Host = v
	}
	if v := os.Getenv("PROXMOX_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			opts.Port = p
		}
	}
	if v := os.Getenv("PROXMOX_USER"); v != "" {
		opts.User = v
	}
	if v := os.Getenv("PROXMOX_TOKEN"); v != "" {
		opts.Token = v
	}
	if v := os.Getenv("PROXMOX_NODE"); v != "" {
		opts.Node = v
	}
	if v := os.Getenv("PROXMOX_TEMPLATE"); v != "" {
		opts.Template = v
	}
	if v := os.Getenv("PROXMOX_STORAGE"); v != "" {
		opts.Storage = v
	}
	if v := os.Getenv("PROXMOX_NETWORK"); v != "" {
		opts.Network = v
	}
	if v := os.Getenv("PROXMOX_VM_START_ID"); v != "" {
		if id, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			opts.VMStartID = id
		}
	}
	if v := os.Getenv("VM_MEMORY"); v != "" {
		if m, err := strconv.Atoi(v); err == nil {
			opts.Memory = m
		}
	}
	if v := os.Getenv("VM_CORES"); v != "" {
		if c, err := strconv.Atoi(v); err == nil {
			opts.Cores = c
		}
	}
	if v := os.Getenv("VM_DISK_SIZE"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			opts.DiskSize = d
		}
	}
	if v := os.Getenv("VM_OS_TYPE"); v != "" {
		opts.OSType = v
	}

	// DevPod-injected
	opts.MachineID = os.Getenv("MACHINE_ID")
	opts.MachineFolder = os.Getenv("MACHINE_FOLDER")

	return opts
}

// BaseURL returns the Proxmox API base URL.
func (o *Options) BaseURL() string {
	return "https://" + o.Host + ":" + strconv.Itoa(o.Port) + "/api2/json"
}

// Validate checks that required options are set.
func (o *Options) Validate() error {
	var missing []string
	if o.Host == "" {
		missing = append(missing, "PROXMOX_HOST")
	}
	if o.User == "" {
		missing = append(missing, "PROXMOX_USER")
	}
	if o.Token == "" {
		missing = append(missing, "PROXMOX_TOKEN")
	}
	if o.Node == "" {
		missing = append(missing, "PROXMOX_NODE")
	}
	if o.Template == "" {
		missing = append(missing, "PROXMOX_TEMPLATE")
	}
	if len(missing) > 0 {
		return &ValidationError{Missing: missing}
	}
	return nil
}

// ValidationError is returned when required options are missing.
type ValidationError struct {
	Missing []string
}

func (e *ValidationError) Error() string {
	return "missing required options: " + strings.Join(e.Missing, ", ")
}
