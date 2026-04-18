package proxmox

import (
	"context"
	"time"
)

var _ Client = (*MockClient)(nil)

var _ Client = &MockClient{}

// MockClient is a hand-written mock implementing the Client interface.
// Each method records its calls and returns pre-configured responses.
type MockClient struct {
	Version           *VersionResponse
	VersionErr        error
	Resources         []Resource
	ResourcesErr      error
	StoragePools      []Storage
	StoragePoolsErr   error
	Networks          []Network
	NetworksErr       error

	CloneVMUPID       string
	CloneVMErr        error
	ConfigureVMErr    error
	ResizeDiskErr     error
	StartVMUPID       string
	StartVMErr        error
	StopVMUPID        string
	StopVMErr         error
	ShutdownVMUPID    string
	ShutdownVMErr     error
	DeleteVMUPID      string
	DeleteVMErr       error
	VMStatus          *VMStatus
	VMStatusErr       error
	Ifaces            []NetworkInterface
	IfacesErr         error
	WaitForTaskErr    error

	CreateVMUPID      string
	CreateVMErr       error
	ConvertTemplateUPID string
	ConvertTemplateErr  error
	DownloadURLUPID     string
	DownloadURLErr      error

	// Recorded calls for assertions
	Calls []MockCall
}

// MockCall records a method call with its arguments.
type MockCall struct {
	Method string
	Args   []interface{}
}

func (m *MockClient) record(name string, args ...interface{}) {
	if m.Calls == nil {
		m.Calls = []MockCall{}
	}
	m.Calls = append(m.Calls, MockCall{Method: name, Args: args})
}

func (m *MockClient) GetVersion(ctx context.Context) (*VersionResponse, error) {
	m.record("GetVersion")
	return m.Version, m.VersionErr
}

func (m *MockClient) GetClusterResources(ctx context.Context) ([]Resource, error) {
	m.record("GetClusterResources")
	return m.Resources, m.ResourcesErr
}

func (m *MockClient) GetNodeStorage(ctx context.Context, node string) ([]Storage, error) {
	m.record("GetNodeStorage", node)
	return m.StoragePools, m.StoragePoolsErr
}

func (m *MockClient) GetNodeNetworks(ctx context.Context, node string) ([]Network, error) {
	m.record("GetNodeNetworks", node)
	return m.Networks, m.NetworksErr
}

func (m *MockClient) CloneVM(ctx context.Context, node string, templateID int, req CloneRequest) (string, error) {
	m.record("CloneVM", node, templateID, req)
	return m.CloneVMUPID, m.CloneVMErr
}

func (m *MockClient) ConfigureVM(ctx context.Context, node string, vmid int, cfg VMConfig) error {
	m.record("ConfigureVM", node, vmid, cfg)
	return m.ConfigureVMErr
}

func (m *MockClient) ResizeDisk(ctx context.Context, node string, vmid int, disk, size string) error {
	m.record("ResizeDisk", node, vmid, disk, size)
	return m.ResizeDiskErr
}

func (m *MockClient) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	m.record("StartVM", node, vmid)
	return m.StartVMUPID, m.StartVMErr
}

func (m *MockClient) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	m.record("StopVM", node, vmid)
	return m.StopVMUPID, m.StopVMErr
}

func (m *MockClient) ShutdownVM(ctx context.Context, node string, vmid int) (string, error) {
	m.record("ShutdownVM", node, vmid)
	return m.ShutdownVMUPID, m.ShutdownVMErr
}

func (m *MockClient) DeleteVM(ctx context.Context, node string, vmid int, purge bool) (string, error) {
	m.record("DeleteVM", node, vmid, purge)
	return m.DeleteVMUPID, m.DeleteVMErr
}

func (m *MockClient) GetVMStatus(ctx context.Context, node string, vmid int) (*VMStatus, error) {
	m.record("GetVMStatus", node, vmid)
	return m.VMStatus, m.VMStatusErr
}

func (m *MockClient) GetNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error) {
	m.record("GetNetworkInterfaces", node, vmid)
	return m.Ifaces, m.IfacesErr
}

func (m *MockClient) WaitForTask(ctx context.Context, node, upid string, timeout time.Duration) error {
	m.record("WaitForTask", node, upid, timeout)
	return m.WaitForTaskErr
}

func (m *MockClient) CreateVM(ctx context.Context, node string, req CreateVMRequest) (string, error) {
	m.record("CreateVM", node, req)
	return m.CreateVMUPID, m.CreateVMErr
}

func (m *MockClient) ConvertToTemplate(ctx context.Context, node string, vmid int) (string, error) {
	m.record("ConvertToTemplate", node, vmid)
	return m.ConvertTemplateUPID, m.ConvertTemplateErr
}

func (m *MockClient) DownloadURL(ctx context.Context, node, storage, downloadURL, filename string) (string, error) {
	m.record("DownloadURL", node, storage, downloadURL, filename)
	return m.DownloadURLUPID, m.DownloadURLErr
}

// HasCall returns true if the mock received a call with the given method name.
func (m *MockClient) HasCall(method string) bool {
	for _, c := range m.Calls {
		if c.Method == method {
			return true
		}
	}
	return false
}

// CallCount returns the number of times a method was called.
func (m *MockClient) CallCount(method string) int {
	count := 0
	for _, c := range m.Calls {
		if c.Method == method {
			count++
		}
	}
	return count
}
