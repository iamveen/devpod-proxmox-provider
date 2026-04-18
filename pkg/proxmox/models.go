package proxmox

import (
	"context"
	"time"
)

// VersionResponse is the response from GET /version.
type VersionResponse struct {
	Version string `json:"version"`
	Release string `json:"release"`
	RepoID  string `json:"repoid"`
}

// Resource represents an entry in GET /cluster/resources.
type Resource struct {
	Type   string `json:"type"`
	VMID   int    `json:"vmid"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status"`
	Node   string `json:"node,omitempty"`
	MaxMem uint64 `json:"maxmem,omitempty"`
	MaxDisk uint64 `json:"maxdisk,omitempty"`
	Tags   string `json:"tags,omitempty"`
	Template bool  `json:"template,omitempty"`
}

// Storage represents a storage pool on a node.
type Storage struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Shared  int    `json:"shared,omitempty"`
}

// Network represents a network bridge on a node.
type Network struct {
	Iface   string `json:"iface"`
	Type    string `json:"type"`
	Active  int    `json:"active,omitempty"`
		Address string `json:"address,omitempty"`
}

// VMStatus is the response from GET .../status/current.
type VMStatus struct {
	Status     string `json:"status"`
	QMPStatus  string `json:"qmpstatus"`
	Name       string `json:"name,omitempty"`
	CPU        float64 `json:"cpu"`
	CPUs       int     `json:"cpus,omitempty"`
	MaxMem     uint64  `json:"maxmem"`
	Mem        uint64  `json:"mem,omitempty"`
	Uptime     int     `json:"uptime,omitempty"`
}

// CloneRequest holds parameters for cloning a VM.
type CloneRequest struct {
	NewID   int
	Name    string
	Node    string
	Full    int
	Storage string
}

// VMConfig holds parameters for configuring a VM.
type VMConfig struct {
	Name        string
	SSHKeys     string // URL-encoded public key(s)
	IPConfig0   string // e.g. "ip=dhcp"
	CIUser      string
	Tags        string
	Cores       int
	Memory      int
	OSType      string
	Agent       string // e.g. "enabled=1"
	Serial0     string
	VGA         string
	Boot        string
	Net0        string
	SCSI0       string
	IDE2        string
	Delete      string // comma-separated keys to delete
}

// NetworkInterface represents a network interface reported by the QEMU agent.
type NetworkInterface struct {
	Name            string   `json:"name"`
	HardwareAddress string   `json:"hardware-address"`
	IPAddresses     []IPAddr `json:"ip-addresses"`
}

// IPAddr is an IP address reported by the QEMU agent.
type IPAddr struct {
	IPAddress string `json:"ip-address"`
	Type      string `json:"ip-address-type"` // ipv4 or ipv6
	Prefix    int    `json:"prefix"`
}

// NetworkInterfacesResponse is the response from GET .../agent/network-get-interfaces.
type NetworkInterfacesResponse struct {
	Result []NetworkInterface `json:"result"`
}

// TaskStatus is the response from GET .../tasks/{upid}/status.
type TaskStatus struct {
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
}

// CreateVMRequest holds parameters for creating a new VM.
type CreateVMRequest struct {
	VMID   int
	Name   string
	Memory int
	Cores  int
	OSType string
	Node   string
}

// APITaskResponse is the wrapped data field from async Proxmox API responses.
type APITaskResponse struct {
	UPID string `json:"data"`
}

// Client defines the API surface for interacting with Proxmox VE.
type Client interface {
	// Cluster / node
	GetVersion(ctx context.Context) (*VersionResponse, error)
	GetClusterResources(ctx context.Context) ([]Resource, error)
	GetNodeStorage(ctx context.Context, node string) ([]Storage, error)
	GetNodeNetworks(ctx context.Context, node string) ([]Network, error)

	// VM lifecycle
	CloneVM(ctx context.Context, node string, templateID int, req CloneRequest) (string, error)
	ConfigureVM(ctx context.Context, node string, vmid int, cfg VMConfig) error
	ResizeDisk(ctx context.Context, node string, vmid int, disk, size string) error
	StartVM(ctx context.Context, node string, vmid int) (string, error)
	StopVM(ctx context.Context, node string, vmid int) (string, error)
	ShutdownVM(ctx context.Context, node string, vmid int) (string, error)
	DeleteVM(ctx context.Context, node string, vmid int, purge bool) (string, error)
	GetVMStatus(ctx context.Context, node string, vmid int) (*VMStatus, error)
	GetNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error)
	WaitForTask(ctx context.Context, node, upid string, timeout time.Duration) error

	// Setup-only
	CreateVM(ctx context.Context, node string, req CreateVMRequest) (string, error)
	ConvertToTemplate(ctx context.Context, node string, vmid int) (string, error)
	DownloadURL(ctx context.Context, node, storage, url, filename string) (string, error)
}
