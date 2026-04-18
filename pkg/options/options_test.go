package options_test

import (
	"testing"

	"github.com/iamveen/devpod-proxmox-provider/pkg/options"
)

func TestDefaultOptions(t *testing.T) {
	opts := options.DefaultOptions()
	if opts.Port != 8006 {
		t.Errorf("expected port 8006, got %d", opts.Port)
	}
	if opts.Storage != "local-lvm" {
		t.Errorf("expected storage 'local-lvm', got '%s'", opts.Storage)
	}
	if opts.Network != "vmbr0" {
		t.Errorf("expected network 'vmbr0', got '%s'", opts.Network)
	}
	if opts.VMStartID != 2000 {
		t.Errorf("expected VMStartID 2000, got %d", opts.VMStartID)
	}
	if opts.Memory != 4096 {
		t.Errorf("expected memory 4096, got %d", opts.Memory)
	}
	if opts.Cores != 2 {
		t.Errorf("expected cores 2, got %d", opts.Cores)
	}
	if opts.DiskSize != 50 {
		t.Errorf("expected disk size 50, got %d", opts.DiskSize)
	}
	if opts.OSType != "l26" {
		t.Errorf("expected OS type 'l26', got '%s'", opts.OSType)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("PROXMOX_HOST", "pve.example.com")
	t.Setenv("PROXMOX_PORT", "8007")
	t.Setenv("PROXMOX_USER", "root@pam")
	t.Setenv("PROXMOX_TOKEN", "root@pam!test=secret")
	t.Setenv("PROXMOX_NODE", "pve1")
	t.Setenv("PROXMOX_TEMPLATE", "devpod-template")
	t.Setenv("PROXMOX_STORAGE", "fast-ssd")
	t.Setenv("PROXMOX_NETWORK", "vmbr1")
	t.Setenv("PROXMOX_VM_START_ID", "3000")
	t.Setenv("VM_MEMORY", "8192")
	t.Setenv("VM_CORES", "4")
	t.Setenv("VM_DISK_SIZE", "100")
	t.Setenv("VM_OS_TYPE", "other")
	t.Setenv("MACHINE_ID", "test-machine-123")
	t.Setenv("MACHINE_FOLDER", "/tmp/test")

	opts := options.FromEnv()

	if opts.Host != "pve.example.com" {
		t.Errorf("expected host 'pve.example.com', got '%s'", opts.Host)
	}
	if opts.Port != 8007 {
		t.Errorf("expected port 8007, got %d", opts.Port)
	}
	if opts.User != "root@pam" {
		t.Errorf("expected user 'root@pam', got '%s'", opts.User)
	}
	if opts.Token != "root@pam!test=secret" {
		t.Errorf("expected token 'root@pam!test=secret', got '%s'", opts.Token)
	}
	if opts.Node != "pve1" {
		t.Errorf("expected node 'pve1', got '%s'", opts.Node)
	}
	if opts.Template != "devpod-template" {
		t.Errorf("expected template 'devpod-template', got '%s'", opts.Template)
	}
	if opts.Storage != "fast-ssd" {
		t.Errorf("expected storage 'fast-ssd', got '%s'", opts.Storage)
	}
	if opts.Network != "vmbr1" {
		t.Errorf("expected network 'vmbr1', got '%s'", opts.Network)
	}
	if opts.VMStartID != 3000 {
		t.Errorf("expected VMStartID 3000, got %d", opts.VMStartID)
	}
	if opts.Memory != 8192 {
		t.Errorf("expected memory 8192, got %d", opts.Memory)
	}
	if opts.Cores != 4 {
		t.Errorf("expected cores 4, got %d", opts.Cores)
	}
	if opts.DiskSize != 100 {
		t.Errorf("expected disk size 100, got %d", opts.DiskSize)
	}
	if opts.OSType != "other" {
		t.Errorf("expected OS type 'other', got '%s'", opts.OSType)
	}
	if opts.MachineID != "test-machine-123" {
		t.Errorf("expected machine ID 'test-machine-123', got '%s'", opts.MachineID)
	}
	if opts.MachineFolder != "/tmp/test" {
		t.Errorf("expected machine folder '/tmp/test', got '%s'", opts.MachineFolder)
	}
}

func TestFromEnv_DevPodInjected(t *testing.T) {
	t.Setenv("MACHINE_ID", "abc-123")
	t.Setenv("MACHINE_FOLDER", "/devpod/machines/abc-123")

	opts := options.FromEnv()
	if opts.MachineID != "abc-123" {
		t.Errorf("expected machine ID 'abc-123', got '%s'", opts.MachineID)
	}
	if opts.MachineFolder != "/devpod/machines/abc-123" {
		t.Errorf("expected machine folder '/devpod/machines/abc-123', got '%s'", opts.MachineFolder)
	}
}

func TestBaseURL(t *testing.T) {
	opts := options.DefaultOptions()
	opts.Host = "pve.local"
	opts.Port = 8006
	expected := "https://pve.local:8006/api2/json"
	if opts.BaseURL() != expected {
		t.Errorf("expected '%s', got '%s'", expected, opts.BaseURL())
	}
}

func TestBaseURL_CustomPort(t *testing.T) {
	opts := options.DefaultOptions()
	opts.Host = "pve.example.com"
	opts.Port = 9000
	expected := "https://pve.example.com:9000/api2/json"
	if opts.BaseURL() != expected {
		t.Errorf("expected '%s', got '%s'", expected, opts.BaseURL())
	}
}

func TestValidate_MissingRequired(t *testing.T) {
	opts := options.DefaultOptions()
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	verr := err.(*options.ValidationError)
	expected := []string{"PROXMOX_HOST", "PROXMOX_USER", "PROXMOX_TOKEN", "PROXMOX_NODE", "PROXMOX_TEMPLATE"}
	if len(verr.Missing) != len(expected) {
		t.Fatalf("expected %d missing, got %d: %v", len(expected), len(verr.Missing), verr.Missing)
	}
}

func TestValidate_AllSet(t *testing.T) {
	opts := options.DefaultOptions()
	opts.Host = "pve.local"
	opts.User = "root@pam"
	opts.Token = "root@pam!t=secret"
	opts.Node = "pve1"
	opts.Template = "template"
	err := opts.Validate()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestFromEnv_EmptyDefaults(t *testing.T) {
	// Ensure FromEnv doesn't pull stale values from other tests
	opts := options.DefaultOptions()
	if opts.Host != "" {
		t.Errorf("expected empty host by default, got '%s'", opts.Host)
	}
	if opts.User != "" {
		t.Errorf("expected empty user by default, got '%s'", opts.User)
	}
}
