package cmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/iamveen/devpod-proxmox-provider/pkg/options"
	"github.com/iamveen/devpod-proxmox-provider/pkg/proxmox"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workspace VM",
	RunE:  runCreate,
}

func init() {
	RootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	opts := options.FromEnv()
	if err := opts.Validate(); err != nil {
		return err
	}
	if opts.MachineID == "" {
		return fmt.Errorf("MACHINE_ID not set")
	}

	client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)

	// Step 1: Ensure SSH keypair exists in the machine folder, generating it if needed
	privKeyPath := fmt.Sprintf("%s/id_ed25519", opts.MachineFolder)
	pubKeyPath := fmt.Sprintf("%s/id_ed25519.pub", opts.MachineFolder)
	if err := ensureSSHKeypair(privKeyPath, pubKeyPath); err != nil {
		return fmt.Errorf("generating SSH keypair: %w", err)
	}
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("reading SSH public key: %w", err)
	}
	sshKeys := strings.ReplaceAll(url.QueryEscape(strings.TrimSpace(string(pubKey))), "+", "%20")

	// Step 2: Find the template VMID
	templateID, err := findTemplateID(ctx, client, opts.Node, opts.Template)
	if err != nil {
		return fmt.Errorf("finding template: %w", err)
	}

	// Step 3: Find a free VMID and clone
	vmid := opts.VMStartID
	vmName := "devpod-" + opts.MachineID

	for {
		selectedVMID := vmid
		err := tryClone(ctx, client, opts, templateID, selectedVMID, vmName, sshKeys)
		if err == nil {
			vmid = selectedVMID
			break
		}
		// If the error indicates the VMID is taken, try the next one
		if !isVMIDConflict(err) {
			return fmt.Errorf("clone failed: %w", err)
		}
		vmid++
		if vmid > opts.VMStartID+100 {
			return fmt.Errorf("could not find free VMID after 100 attempts")
		}
	}

	// Step 4: If disk size exceeds template (default template ~17GB for cloud image), resize
	templateDiskGB := 17 // Ubuntu cloud image default
	if opts.DiskSize > templateDiskGB {
		increase := opts.DiskSize - templateDiskGB
		diskSize := fmt.Sprintf("+%dG", increase)
		if err := client.ResizeDisk(ctx, opts.Node, vmid, "virtio0", diskSize); err != nil {
			return fmt.Errorf("resizing disk: %w", err)
		}
	}

	// Step 5: Start the VM
	upid, err := client.StartVM(ctx, opts.Node, vmid)
	if err != nil {
		return fmt.Errorf("starting VM: %w", err)
	}
	if err := client.WaitForTask(ctx, opts.Node, upid, 5*time.Minute); err != nil {
		return fmt.Errorf("waiting for VM start: %w", err)
	}

	// Step 6: Wait for IP address
	fmt.Fprintf(os.Stderr, "Waiting for VM to get an IP address...\n")
	ip, err := proxmox.WaitForIP(ctx, client, opts.Node, vmid, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("waiting for IP: %w", err)
	}

	// Step 7: Output JSON with connection info
	result := map[string]string{
		"ipAddress": ip,
		"sshUser":   "devpod",
	}
	out, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func tryClone(ctx context.Context, client proxmox.Client, opts options.Options, templateID, vmid int, vmName, sshKeys string) error {
	upid, err := client.CloneVM(ctx, opts.Node, templateID, proxmox.CloneRequest{
		NewID:   vmid,
		Name:    vmName,
		Node:    opts.Node,
		Full:    1,
		Storage: opts.Storage,
	})
	if err != nil {
		return err
	}
	if err := client.WaitForTask(ctx, opts.Node, upid, 5*time.Minute); err != nil {
		return err
	}

	// Configure cloud-init; clean up the clone if configuration fails
	if err := client.ConfigureVM(ctx, opts.Node, vmid, proxmox.VMConfig{
		Name:      vmName,
		CIUser:    "devpod",
		SSHKeys:   sshKeys,
		IPConfig0: "ip=dhcp",
		Tags:      "devpod",
		Cores:     opts.Cores,
		Memory:    opts.Memory,
	}); err != nil {
		if upid, delErr := client.DeleteVM(ctx, opts.Node, vmid, true); delErr == nil {
			_ = client.WaitForTask(ctx, opts.Node, upid, 2*time.Minute)
		}
		return err
	}
	return nil
}

func isVMIDConflict(err error) bool {
	msg := err.Error()
	return contains(msg, "already exists") || contains(msg, "already defined")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func findTemplateID(ctx context.Context, client proxmox.Client, node, template string) (int, error) {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return 0, err
	}
	for _, r := range resources {
		if r.Type == "qemu" && r.Node == node && r.Template != 0 && r.Name == template {
			return r.VMID, nil
		}
	}
	return 0, fmt.Errorf("template '%s' not found on node '%s'", template, node)
}

// ensureSSHKeypair generates an ECDSA P-256 keypair at the given paths if they don't already exist.
func ensureSSHKeypair(privPath, pubPath string) error {
	if _, err := os.Stat(pubPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(strings.TrimSuffix(privPath, "/id_ed25519"), 0700); err != nil {
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	privDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	privFile, err := os.OpenFile(privPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer privFile.Close()
	if err := pem.Encode(privFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER}); err != nil {
		return err
	}
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	return os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(pub), 0644)
}
