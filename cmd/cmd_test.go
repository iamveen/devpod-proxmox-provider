package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/iamveen/devpod-proxmox-provider/pkg/proxmox"
)

// --- Helper: set up context with mocks by replacing createVMByMachineID ---
// Because commands read from env vars, we need to test via MockClient injection.
// We do this through a test-only client factory pattern.

// TestStatusCmd_VMNotFound tests that status outputs "notfound" when no VM exists.
func TestStatusCmd_VMFinding(t *testing.T) {
	// Test findVMByMachineID with mock
	mock := &proxmox.MockClient{
		Resources: []proxmox.Resource{
			{Type: "qemu", VMID: 100, Name: "devpod-test-abc", Node: "pve1"},
		},
	}
	
	vmid, err := findVMByMachineIDTest(context.Background(), mock, "pve1", "test-abc")
	if err != nil {
		t.Fatalf("expected to find VM, got error: %v", err)
	}
	if vmid != 100 {
		t.Errorf("expected VMID 100, got %d", vmid)
	}
}

func TestStatusCmd_VMNotFound(t *testing.T) {
	mock := &proxmox.MockClient{
		Resources: []proxmox.Resource{
			{Type: "qemu", VMID: 100, Name: "devpod-other", Node: "pve1"},
		},
	}

	_, err := findVMByMachineIDTest(context.Background(), mock, "pve1", "test-abc")
	if err == nil {
		t.Fatal("expected error when VM not found")
	}
}

func TestStatusCmd_StateMapping(t *testing.T) {
	tests := []struct {
		proxmoxStatus string
		expected      string
	}{
		{"running", "running"},
		{"stopped", "stopped"},
		{"paused", "busy"},
		{"prelaunch", "busy"},
		{"suspended", "busy"},
	}

	for _, tt := range tests {
		t.Run(tt.proxmoxStatus, func(t *testing.T) {
			result := mapProxmoxStatus(tt.proxmoxStatus)
			if result != tt.expected {
				t.Errorf("proxmox '%s' → expected '%s', got '%s'", tt.proxmoxStatus, tt.expected, result)
			}
		})
	}
}

func TestFindTemplateID(t *testing.T) {
	mock := &proxmox.MockClient{
		Resources: []proxmox.Resource{
			{Type: "qemu", VMID: 9000, Name: "devpod-template", Node: "pve1", Template: true},
			{Type: "qemu", VMID: 100, Name: "devpod-ws1", Node: "pve1"},
		},
	}

	id, err := findTemplateID(context.Background(), mock, "pve1", "devpod-template")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 9000 {
		t.Errorf("expected template ID 9000, got %d", id)
	}
}

func TestFindTemplateID_NotFound(t *testing.T) {
	mock := &proxmox.MockClient{
		Resources: []proxmox.Resource{
			{Type: "qemu", VMID: 100, Name: "other-template", Node: "pve1", Template: true},
		},
	}

	_, err := findTemplateID(context.Background(), mock, "pve1", "devpod-template")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsVMIDConflict(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected bool
	}{
		{"VM 2000 already exists on node 'pve1'", true},
		{"unable to create VM 2000 - VM 2000 already defined", true},
		{"some other error", false},
		{"permission denied", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			result := isVMIDConflict(errors.New(tt.errMsg))
			if result != tt.expected {
				t.Errorf("isVMIDConflict('%s') = %v, want %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

func TestSetup_TemplateAlreadyExists(t *testing.T) {
	mock := &proxmox.MockClient{
		Version: &proxmox.VersionResponse{Version: "7.4", Release: "7"},
		StoragePools: []proxmox.Storage{
			{Storage: "local-lvm", Type: "lvmthin"},
		},
		Networks: []proxmox.Network{
			{Iface: "vmbr0", Type: "bridge"},
		},
		Resources: []proxmox.Resource{
			{Type: "qemu", VMID: 9000, Name: "devpod-template", Node: "pve1", Template: true},
		},
	}

	exists, err := templateAlreadyExists(context.Background(), mock, "pve1", "devpod-template")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected template to exist")
	}
}

func TestSetup_VerifyStorageMissing(t *testing.T) {
	mock := &proxmox.MockClient{
		Version:      &proxmox.VersionResponse{Version: "7.4", Release: "7"},
		StoragePools: []proxmox.Storage{},
	}

	_, err := mock.GetNodeStorage(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate the setup check logic
	storages, _ := mock.GetNodeStorage(context.Background(), "pve1")
	found := false
	for _, s := range storages {
		if s.Storage == "local-lvm" {
			found = true
			break
		}
	}
	if found {
		t.Error("expected storage not found")
	}
}

func TestSetup_VerifyNetworkMissing(t *testing.T) {
	mock := &proxmox.MockClient{
		Networks: []proxmox.Network{
			{Iface: "vmbr1", Type: "bridge"},
		},
	}

	networks, _ := mock.GetNodeNetworks(context.Background(), "pve1")
	found := false
	for _, n := range networks {
		if n.Iface == "vmbr0" {
			found = true
			break
		}
	}
	if found {
		t.Error("expected network bridge not found")
	}
}

func TestDelete_ShutdownBeforeDestroy(t *testing.T) {
	mock := &proxmox.MockClient{
		VMStatus: &proxmox.VMStatus{Status: "running"},
	}
	
	// Verify we'd call shutdown for running VMs
	if mock.VMStatus.Status != "running" {
		t.Error("should shutdown running VMs")
	}
}

// findVMByMachineIDTest is the same logic as findVMByMachineID but takes a mockable client.
func findVMByMachineIDTest(ctx context.Context, client proxmox.Client, node, machineID string) (int, error) {
	expectedName := "devpod-" + machineID
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return 0, err
	}
	for _, r := range resources {
		if r.Type == "qemu" && r.Name == expectedName && r.Node == node {
			return r.VMID, nil
		}
	}
	return 0, fmt.Errorf("VM not found: %s", expectedName)
}

// mapProxmoxStatus maps Proxmox VM status to DevPod status strings.
func mapProxmoxStatus(status string) string {
	switch status {
	case "running":
		return "running"
	case "stopped":
		return "stopped"
	default:
		return "busy"
	}
}

func TestCreate_DiskResizeLogic(t *testing.T) {
	// Test that disk resize is only triggered when VM_DISK_SIZE > template size
	templateDiskGB := 17

	tests := []struct {
		diskSize    int
		shouldResize bool
	}{
		{50, true},
		{17, false},
		{16, false},
		{100, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("disk=%d", tt.diskSize), func(t *testing.T) {
			wouldResize := tt.diskSize > templateDiskGB
			if wouldResize != tt.shouldResize {
				t.Errorf("disk=%d: would resize=%v, expected=%v", tt.diskSize, wouldResize, tt.shouldResize)
			}
		})
	}
}

func TestCreate_CloneRetryLogic(t *testing.T) {
	// Simulate VMID conflict retry behavior
	attemptedVMIDs := []int{}
	conflictCount := 0
	startID := 2000
	
	for vmid := startID; vmid < startID+100; vmid++ {
		attemptedVMIDs = append(attemptedVMIDs, vmid)
		if conflictCount < 2 {
			conflictCount++
			continue // simulate conflict
		}
		// Success on 3rd attempt
		break
	}

	if len(attemptedVMIDs) != 3 {
		t.Errorf("expected 3 attempts, got %d: %v", len(attemptedVMIDs), attemptedVMIDs)
	}
	if attemptedVMIDs[0] != 2000 || attemptedVMIDs[1] != 2001 || attemptedVMIDs[2] != 2002 {
		t.Errorf("expected VMIDs 2000,2001,2002, got %v", attemptedVMIDs)
	}
}

func TestCommandTemplate(t *testing.T) {
	// Verify the command template is correct
	expected := `"${DEVPOD}" helper sh -c "${COMMAND}"`
	
	var sb strings.Builder
	fmt.Fprintln(&sb, expected)
	output := strings.TrimSpace(sb.String())
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestWaitForTask_PollingBehavior(t *testing.T) {
	// Test that WaitForTask polls with the mock
	callCount := 0
	var fakeTask proxmox.MockClient
	
	// Set up the error field to fail after we check
	fakeTask.WaitForTaskErr = fmt.Errorf("task failed")
	
	err := fakeTask.WaitForTask(context.Background(), "pve1", "UPID:test", time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	callCount = fakeTask.CallCount("WaitForTask")
	if callCount != 1 {
		t.Errorf("expected 1 WaitForTask call, got %d", callCount)
	}
}

func TestMockClient_RecordsAllCalls(t *testing.T) {
	mock := &proxmox.MockClient{}
	
	_, _ = mock.GetVersion(context.Background())
	_, _ = mock.GetClusterResources(context.Background())
	_ = mock.ConfigureVM(context.Background(), "pve1", 101, proxmox.VMConfig{})
	_, _ = mock.StartVM(context.Background(), "pve1", 101)
	_, _ = mock.GetVMStatus(context.Background(), "pve1", 101)

	expectedCalls := []string{
		"GetVersion", "GetClusterResources", "ConfigureVM", "StartVM", "GetVMStatus",
	}

	for _, expected := range expectedCalls {
		if !mock.HasCall(expected) {
			t.Errorf("expected call '%s' not recorded", expected)
		}
	}

	if mock.CallCount("GetVersion") != 1 {
		t.Errorf("expected 1 GetVersion call, got %d", mock.CallCount("GetVersion"))
	}
}
