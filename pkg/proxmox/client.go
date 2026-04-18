package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient is a Proxmox API client using net/http.
type HTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// APIResponse wraps the standard Proxmox API JSON envelope.
type APIResponse struct {
	Data   interface{}       `json:"data"`
	Errors map[string]string `json:"errors,omitempty"`
}

// NewHTTPClient creates a new Proxmox API client.
func NewHTTPClient(host string, port int, token string) *HTTPClient {
	return &HTTPClient{
		baseURL: fmt.Sprintf("https://%s:%d/api2/json", host, port),
		token:   token,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 30 * time.Second,
		},
	}
}

// NewHTTPClientWithClient creates a client with a custom http.Client (for testing).
func NewHTTPClientWithClient(baseURL, token string, hc *http.Client) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		token:   token,
		client:  hc,
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body url.Values) ([]byte, error) {
	var req *http.Request
	var err error

	if method == http.MethodGet {
		query := ""
		if body != nil {
			query = body.Encode()
		}
		fullURL := c.baseURL + path
		if query != "" {
			fullURL += "?" + query
		}
		req, err = http.NewRequestWithContext(ctx, method, fullURL, nil)
	} else {
		fullURL := c.baseURL + path
		reqBody := strings.NewReader(body.Encode())
		req, err = http.NewRequestWithContext(ctx, method, fullURL, reqBody)
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", "PVEAPIToken="+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIResponse
		if json.Unmarshal(respBody, &apiErr) == nil && len(apiErr.Errors) > 0 {
			return nil, fmt.Errorf("API error (HTTP %d): %v", resp.StatusCode, apiErr.Errors)
		}
		return nil, fmt.Errorf("API error: HTTP %d — %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *HTTPClient) unmarshal(data []byte, v interface{}) error {
	var resp APIResponse
	resp.Data = v
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}
	return nil
}

func (c *HTTPClient) unmarshalTask(data []byte) (string, error) {
	var resp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("unmarshaling task response: %w", err)
	}
	return resp.Data, nil
}

// GetVersion returns the Proxmox API version info.
func (c *HTTPClient) GetVersion(ctx context.Context) (*VersionResponse, error) {
	data, err := c.do(ctx, http.MethodGet, "/version", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data VersionResponse `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling version response: %w", err)
	}
	return &resp.Data, nil
}

// GetClusterResources returns all cluster resources (VMs, nodes, storage, etc.).
func (c *HTTPClient) GetClusterResources(ctx context.Context) ([]Resource, error) {
	data, err := c.do(ctx, http.MethodGet, "/cluster/resources", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Resource `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling cluster resources: %w", err)
	}
	return resp.Data, nil
}

// GetNodeStorage lists storage pools on a node.
func (c *HTTPClient) GetNodeStorage(ctx context.Context, node string) ([]Storage, error) {
	data, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/storage", node), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Storage `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling node storage: %w", err)
	}
	return resp.Data, nil
}

// GetNodeNetworks lists network interfaces on a node.
func (c *HTTPClient) GetNodeNetworks(ctx context.Context, node string) ([]Network, error) {
	data, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/network", node), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []Network `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling node networks: %w", err)
	}
	return resp.Data, nil
}

// CloneVM clones a template VM to a new VM.
func (c *HTTPClient) CloneVM(ctx context.Context, node string, templateID int, req CloneRequest) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, templateID)
	params := url.Values{}
	params.Set("newid", strconv.Itoa(req.NewID))
	if req.Name != "" {
		params.Set("name", req.Name)
	}
	if req.Node != "" {
		params.Set("node", req.Node)
	}
	params.Set("full", strconv.Itoa(req.Full))
	if req.Storage != "" {
		params.Set("storage", req.Storage)
	}
	data, err := c.do(ctx, http.MethodPost, path, params)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// ConfigureVM updates a VM's configuration.
func (c *HTTPClient) ConfigureVM(ctx context.Context, node string, vmid int, cfg VMConfig) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)
	params := url.Values{}
	if cfg.Name != "" {
		params.Set("name", cfg.Name)
	}
	if cfg.SSHKeys != "" {
		params.Set("sshkeys", cfg.SSHKeys)
	}
	if cfg.IPConfig0 != "" {
		params.Set("ipconfig0", cfg.IPConfig0)
	}
	if cfg.CIUser != "" {
		params.Set("ciuser", cfg.CIUser)
	}
	if cfg.Tags != "" {
		params.Set("tags", cfg.Tags)
	}
	if cfg.Cores > 0 {
		params.Set("cores", strconv.Itoa(cfg.Cores))
	}
	if cfg.Memory > 0 {
		params.Set("memory", strconv.Itoa(cfg.Memory))
	}
	if cfg.OSType != "" {
		params.Set("ostype", cfg.OSType)
	}
	if cfg.Agent != "" {
		params.Set("agent", cfg.Agent)
	}
	if cfg.Serial0 != "" {
		params.Set("serial0", cfg.Serial0)
	}
	if cfg.VGA != "" {
		params.Set("vga", cfg.VGA)
	}
	if cfg.Boot != "" {
		params.Set("boot", cfg.Boot)
	}
	if cfg.Net0 != "" {
		params.Set("net0", cfg.Net0)
	}
	if cfg.SCSI0 != "" {
		params.Set("scsi0", cfg.SCSI0)
	}
	if cfg.IDE2 != "" {
		params.Set("ide2", cfg.IDE2)
	}
	if cfg.Delete != "" {
		params.Set("delete", cfg.Delete)
	}
	_, err := c.do(ctx, http.MethodPut, path, params)
	return err
}

// ResizeDisk resizes a VM disk.
func (c *HTTPClient) ResizeDisk(ctx context.Context, node string, vmid int, disk, size string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/resize", node, vmid)
	params := url.Values{}
	params.Set("disk", disk)
	params.Set("size", size)
	_, err := c.do(ctx, http.MethodPost, path, params)
	return err
}

// StartVM starts a VM.
func (c *HTTPClient) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmid)
	data, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// StopVM stops a VM immediately.
func (c *HTTPClient) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, vmid)
	data, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// ShutdownVM sends ACPI shutdown to a VM.
func (c *HTTPClient) ShutdownVM(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", node, vmid)
	data, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// DeleteVM destroys a VM and its disks.
func (c *HTTPClient) DeleteVM(ctx context.Context, node string, vmid int, purge bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/destroy", node, vmid)
	params := url.Values{}
	if purge {
		params.Set("purge", "1")
	}
	data, err := c.do(ctx, http.MethodPost, path, params)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// GetVMStatus returns the current status of a VM.
func (c *HTTPClient) GetVMStatus(ctx context.Context, node string, vmid int) (*VMStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data VMStatus `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling VM status: %w", err)
	}
	return &resp.Data, nil
}

// GetNetworkInterfaces returns network interfaces via the QEMU agent.
func (c *HTTPClient) GetNetworkInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data NetworkInterfacesResponse `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling network interfaces: %w", err)
	}
	return resp.Data.Result, nil
}

// WaitForTask polls a task's UPID until it completes or times out.
func (c *HTTPClient) WaitForTask(ctx context.Context, node, upid string, timeout time.Duration) error {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, url.PathEscape(upid))
	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("task %s timed out after %s", upid, timeout)
		case <-ticker.C:
			data, err := c.do(ctx, http.MethodGet, path, nil)
			if err != nil {
				return fmt.Errorf("polling task status: %w", err)
			}
			var resp struct {
				Data TaskStatus `json:"data"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("unmarshaling task status: %w", err)
			}
			if resp.Data.Status == "stopped" {
				if resp.Data.ExitStatus == "OK" {
					return nil
				}
				return fmt.Errorf("task %s failed: %s", upid, resp.Data.ExitStatus)
			}
		}
	}
}

// CreateVM creates a new VM from scratch (used by setup).
func (c *HTTPClient) CreateVM(ctx context.Context, node string, req CreateVMRequest) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", node)
	params := url.Values{}
	params.Set("vmid", strconv.Itoa(req.VMID))
	if req.Name != "" {
		params.Set("name", req.Name)
	}
	if req.Memory > 0 {
		params.Set("memory", strconv.Itoa(req.Memory))
	}
	if req.Cores > 0 {
		params.Set("cores", strconv.Itoa(req.Cores))
	}
	if req.OSType != "" {
		params.Set("ostype", req.OSType)
	}
	data, err := c.do(ctx, http.MethodPost, path, params)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// ConvertToTemplate converts a VM to a template.
func (c *HTTPClient) ConvertToTemplate(ctx context.Context, node string, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/template", node, vmid)
	data, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}

// DownloadURL downloads a file to Proxmox storage (server-side).
func (c *HTTPClient) DownloadURL(ctx context.Context, node, storage, downloadURL, filename string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/download-url", node, storage)
	params := url.Values{}
	params.Set("url", downloadURL)
	params.Set("filename", filename)
	params.Set("content", "import")
	data, err := c.do(ctx, http.MethodPost, path, params)
	if err != nil {
		return "", err
	}
	return c.unmarshalTask(data)
}
