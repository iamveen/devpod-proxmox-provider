package proxmox_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/iamveen/devpod-proxmox-provider/pkg/proxmox"
)

func newTestServer(handler http.HandlerFunc) (*httptest.Server, *proxmox.HTTPClient) {
	server := httptest.NewTLSServer(handler)
	client := proxmox.NewHTTPClientWithClient(server.URL, "token", server.Client())
	return server, client
}

func TestGetVersion(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/version" {
			t.Errorf("expected /version, got %s", r.URL.Path)
		}
		checkAuth(t, r, "token")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{
				"version": "7.4.5",
				"release": "7",
				"repoid":  "abc123",
			},
		})
	})
	defer server.Close()

	v, err := client.GetVersion(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Version != "7.4.5" {
		t.Errorf("expected version '7.4.5', got '%s'", v.Version)
	}
}

func TestGetClusterResources(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"type": "qemu", "vmid": 100, "name": "my-vm", "status": "running", "node": "pve1"},
				{"type": "node", "node": "pve1", "status": "online"},
			},
		})
	})
	defer server.Close()

	resources, err := client.GetClusterResources(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	if resources[0].Type != "qemu" || resources[0].VMID != 100 {
		t.Errorf("expected qemu vmid 100, got %+v", resources[0])
	}
}

func TestCloneVM(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/qemu/9000/clone") {
			t.Errorf("expected clone path, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		if params.Get("newid") != "101" {
			t.Errorf("expected newid 101, got %s", params.Get("newid"))
		}
		if params.Get("name") != "devpod-test" {
			t.Errorf("expected name 'devpod-test', got %s", params.Get("name"))
		}
		if params.Get("full") != "1" {
			t.Errorf("expected full=1, got %s", params.Get("full"))
		}
		json.NewEncoder(w).Encode(map[string]string{
			"data": "UPID:pve1:00001234:ABCDEFGH:65f1a2b3:qmclone:100:root@pam:",
		})
	})
	defer server.Close()

	upid, err := client.CloneVM(t.Context(), "pve1", 9000, proxmox.CloneRequest{
		NewID:   101,
		Name:    "devpod-test",
		Node:    "pve1",
		Full:    1,
		Storage: "local-lvm",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedUPID := "UPID:pve1:00001234:ABCDEFGH:65f1a2b3:qmclone:100:root@pam:"
	if upid != expectedUPID {
		t.Errorf("expected upid '%s', got '%s'", expectedUPID, upid)
	}
}

func TestConfigureVM(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		if params.Get("ciuser") != "devpod" {
			t.Errorf("expected ciuser 'devpod', got '%s'", params.Get("ciuser"))
		}
		if params.Get("ipconfig0") != "ip=dhcp" {
			t.Errorf("expected ipconfig0 'ip=dhcp', got '%s'", params.Get("ipconfig0"))
		}
		if params.Get("tags") != "devpod" {
			t.Errorf("expected tags 'devpod', got '%s'", params.Get("tags"))
		}
		if params.Get("cores") != "2" {
			t.Errorf("expected cores '2', got '%s'", params.Get("cores"))
		}
		if params.Get("memory") != "4096" {
			t.Errorf("expected memory '4096', got '%s'", params.Get("memory"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": nil})
	})
	defer server.Close()

	err := client.ConfigureVM(t.Context(), "pve1", 101, proxmox.VMConfig{
		CIUser:    "devpod",
		IPConfig0: "ip=dhcp",
		Tags:      "devpod",
		Cores:     2,
		Memory:    4096,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureVM_URLEncodedSSHKeys(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		key := params.Get("sshkeys")
		if !strings.Contains(key, "%0A") {
			t.Errorf("expected URL-encoded newline in sshkeys, got '%s'", key)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": nil})
	})
	defer server.Close()

	err := client.ConfigureVM(t.Context(), "pve1", 101, proxmox.VMConfig{
		SSHKeys: url.QueryEscape("ssh-ed25519 AAAA key1\nssh-ed25519 BBBB key2"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResizeDisk(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/qemu/101/resize") {
			t.Errorf("expected resize path, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		if params.Get("disk") != "scsi0" {
			t.Errorf("expected disk 'scsi0', got '%s'", params.Get("disk"))
		}
		if params.Get("size") != "+50G" {
			t.Errorf("expected size '+50G', got '%s'", params.Get("size"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": nil})
	})
	defer server.Close()

	err := client.ResizeDisk(t.Context(), "pve1", 101, "scsi0", "+50G")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartVM(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"data": "UPID:pve1:00001234:start:qmstart:101:root@pam:",
		})
	})
	defer server.Close()

	upid, err := client.StartVM(t.Context(), "pve1", 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upid != "UPID:pve1:00001234:start:qmstart:101:root@pam:" {
		t.Errorf("unexpected upid: %s", upid)
	}
}

func TestStopVM(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"data": "UPID:pve1:00001234:stop:qmstop:101:root@pam:",
		})
	})
	defer server.Close()

	upid, err := client.StopVM(t.Context(), "pve1", 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upid != "UPID:pve1:00001234:stop:qmstop:101:root@pam:" {
		t.Errorf("unexpected upid: %s", upid)
	}
}

func TestShutdownVM(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/status/shutdown") {
			t.Errorf("expected shutdown path, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"data": "UPID:pve1:00001234:shutdown:qmshutdown:101:root@pam:",
		})
	})
	defer server.Close()

	upid, err := client.ShutdownVM(t.Context(), "pve1", 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(upid, "UPID:") {
		t.Errorf("expected UPID, got: %s", upid)
	}
}

func TestDeleteVM(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/qemu/101/destroy") {
			t.Errorf("expected destroy path, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		if params.Get("purge") != "1" {
			t.Errorf("expected purge=1, got '%s'", params.Get("purge"))
		}
		json.NewEncoder(w).Encode(map[string]string{
			"data": "UPID:pve1:00001234:destroy:qmdestroy:101:root@pam:",
		})
	})
	defer server.Close()

	upid, err := client.DeleteVM(t.Context(), "pve1", 101, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(upid, "UPID:") {
		t.Errorf("expected UPID, got: %s", upid)
	}
}

func TestGetVMStatus(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/status/current") {
			t.Errorf("expected status/current path, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"status":    "running",
				"qmpstatus": "running",
				"name":      "devpod-test",
				"cpus":      2,
				"maxmem":    4294967296,
			},
		})
	})
	defer server.Close()

	status, err := client.GetVMStatus(t.Context(), "pve1", 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", status.Status)
	}
	if status.Name != "devpod-test" {
		t.Errorf("expected name 'devpod-test', got '%s'", status.Name)
	}
}

func TestGetNetworkInterfaces(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/agent/network-get-interfaces") {
			t.Errorf("expected agent path, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"result": []map[string]interface{}{
					{
						"name":              "lo",
						"hardware-address":  "",
						"ip-addresses": []map[string]interface{}{
							{"ip-address": "127.0.0.1", "ip-address-type": "ipv4", "prefix": 8},
						},
					},
					{
						"name":              "eth0",
						"hardware-address":  "00:11:22:33:44:55",
						"ip-addresses": []map[string]interface{}{
							{"ip-address": "192.168.1.100", "ip-address-type": "ipv4", "prefix": 24},
						},
					},
				},
			},
		})
	})
	defer server.Close()

	ifaces, err := client.GetNetworkInterfaces(t.Context(), "pve1", 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(ifaces))
	}
	// Find non-loopback
	for _, iface := range ifaces {
		if iface.Name == "eth0" {
			if len(iface.IPAddresses) == 0 || iface.IPAddresses[0].IPAddress != "192.168.1.100" {
				t.Errorf("expected eth0 IP 192.168.1.100, got %+v", iface.IPAddresses)
			}
		}
	}
}

func TestWaitForTask_Success(t *testing.T) {
	callCount := 0
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"status": "running", "exitstatus": ""},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]string{"status": "stopped", "exitstatus": "OK"},
			})
		}
	})
	defer server.Close()

	err := client.WaitForTask(t.Context(), "pve1", "UPID:test", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 polling calls, got %d", callCount)
	}
}

func TestWaitForTask_Failure(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"status": "stopped", "exitstatus": "command failed"},
		})
	})
	defer server.Close()

	err := client.WaitForTask(t.Context(), "pve1", "UPID:test", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Errorf("expected error to contain 'command failed', got '%v'", err)
	}
}

func TestWaitForTask_Timeout(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]string{"status": "running", "exitstatus": ""},
		})
	})
	defer server.Close()

	err := client.WaitForTask(t.Context(), "pve1", "UPID:test", 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got '%v'", err)
	}
}

func TestErrorMapping_Unauthorized(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": map[string]string{"message": "permission denied - invalid token"},
		})
	})
	defer server.Close()

	_, err := client.GetVersion(t.Context())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain '401', got '%v'", err)
	}
}

func TestErrorMapping_Forbidden(t *testing.T) {
	server, client := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errors":{"message":"permission denied"}}`))
	})
	defer server.Close()

	_, err := client.GetVersion(t.Context())
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to contain '403', got '%v'", err)
	}
}

func TestAuthHeader(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuth(t, r, "myauth")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": nil})
	}))
	defer server.Close()

	client := proxmox.NewHTTPClientWithClient(server.URL, "myauth", server.Client())
	_, _ = client.GetVersion(t.Context())
}

func checkAuth(t *testing.T, r *http.Request, expectedToken string) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	expected := "PVEAPIToken=" + expectedToken
	if auth != expected {
		t.Errorf("expected auth header '%s', got '%s'", expected, auth)
	}
}
