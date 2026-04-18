package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iamveen/devpod-proxmox-provider/pkg/options"
	"github.com/iamveen/devpod-proxmox-provider/pkg/proxmox"
	"github.com/spf13/cobra"
)

const (
	defaultImageURL     = "https://cloud-images.ubuntu.com/releases/noble/release-20250601/ubuntu-24.04-server-cloudimg-amd64.img"
	defaultTemplateVMID = 9000
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "One-time Proxmox environment setup (creates cloud-init template)",
	Long: `Prepares Proxmox for use with the DevPod provider.
Creates a reusable cloud-init VM template that all workspace VMs are cloned from.
Idempotent — safe to re-run; skips steps already complete.`,
	RunE: runSetup,
}

var (
	setupImageURL   string
	setupImagePath  string
	setupTemplateID int
	setupDryRun     bool
)

func init() {
	setupCmd.Flags().StringVar(&setupImageURL, "image-url", defaultImageURL, "URL to download Ubuntu cloud image")
	setupCmd.Flags().StringVar(&setupImagePath, "image-path", "", "Path to pre-downloaded cloud image (skips download)")
	setupCmd.Flags().IntVar(&setupTemplateID, "template-vmid", defaultTemplateVMID, "VMID for the template")
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "Print what would be done without making changes")
	RootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	opts := options.FromEnv()
	if err := opts.Validate(); err != nil {
		return err
	}

	client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)

	// Step 1: Verify connectivity
	fmt.Fprintln(os.Stderr, "Step 1: Verifying Proxmox connectivity...")
	v, err := client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("connectivity check failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  Connected to Proxmox %s (%s)\n", v.Version, v.Release)

	// Step 2: Verify storage
	fmt.Fprintf(os.Stderr, "Step 2: Verifying storage pool '%s'...\n", opts.Storage)
	storages, err := client.GetNodeStorage(ctx, opts.Node)
	if err != nil {
		return fmt.Errorf("failed to list storage: %w", err)
	}
	found := false
	for _, s := range storages {
		if s.Storage == opts.Storage {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("storage pool '%s' not found on node '%s'", opts.Storage, opts.Node)
	}
	fmt.Fprintf(os.Stderr, "  Storage '%s' exists\n", opts.Storage)

	// Step 3: Verify network
	fmt.Fprintf(os.Stderr, "Step 3: Verifying network bridge '%s'...\n", opts.Network)
	networks, err := client.GetNodeNetworks(ctx, opts.Node)
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}
	found = false
	for _, n := range networks {
		if n.Iface == opts.Network {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("network bridge '%s' not found on node '%s'", opts.Network, opts.Node)
	}
	fmt.Fprintf(os.Stderr, "  Network '%s' exists\n", opts.Network)

	// Step 4: Check if template already exists
	fmt.Fprintf(os.Stderr, "Step 4: Checking for existing template '%s'...\n", opts.Template)
	templateExists, err := templateAlreadyExists(ctx, client, opts.Node, opts.Template)
	if err != nil {
		return fmt.Errorf("failed to check for existing template: %w", err)
	}
	if templateExists {
		fmt.Fprintf(os.Stderr, "  Template '%s' already exists — skipping template creation\n", opts.Template)
		return nil
	}

	// Image filename
	imageFilename := "ubuntu-24.04-cloudimg.img"
	if setupImagePath != "" {
		imageFilename = filepath.Base(setupImagePath)
	}

	if setupDryRun {
		fmt.Fprintln(os.Stderr, "[dry-run] Would download image and create template")
		return nil
	}

	// Step 5: Download/import image
	if setupImagePath != "" {
		fmt.Fprintf(os.Stderr, "Step 5: Importing image from local path '%s'...\n", setupImagePath)
		// For local images, we need to upload via the API or use qm importdisk
		// The download-url endpoint can accept local files via the import content type
		// For simplicity, we try server-side download URL if it's accessible
		fmt.Fprintln(os.Stderr, "  ⚠ Local image import requires manual upload or SSH access")
		fmt.Fprintln(os.Stderr, "  Use 'qm importdisk' manually, then re-run setup")
		return fmt.Errorf("local image path provided but server-side import not supported yet")
	}
	fmt.Fprintf(os.Stderr, "Step 5: Downloading cloud image (server-side)...\n")
	upid, err := client.DownloadURL(ctx, opts.Node, opts.Storage, setupImageURL, imageFilename)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  Download task: %s\n", upid)
	if err := client.WaitForTask(ctx, opts.Node, upid, 30*time.Minute); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "  Image downloaded")

	// Step 6: Create VM
	fmt.Fprintf(os.Stderr, "Step 6: Creating template VM (ID: %d)...\n", setupTemplateID)
	upid, err = client.CreateVM(ctx, opts.Node, proxmox.CreateVMRequest{
		VMID:   setupTemplateID,
		Name:   opts.Template,
		Memory: 2048,
		Cores:  2,
		OSType: "l26",
		Node:   opts.Node,
	})
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  CreateVM task: %s\n", upid)
	if err := client.WaitForTask(ctx, opts.Node, upid, 5*time.Minute); err != nil {
		return fmt.Errorf("VM creation failed: %w", err)
	}

	// Step 7: Configure VM — attach disk, cloud-init drive, agent, serial, boot
	fmt.Fprintln(os.Stderr, "Step 7: Configuring template VM...")
	volid := fmt.Sprintf("%s:import/%s", opts.Storage, imageFilename)
	if err := client.ConfigureVM(ctx, opts.Node, setupTemplateID, proxmox.VMConfig{
		SCSI0:   volid,
		IDE2:    opts.Storage + ":cloudinit",
		Boot:    "order=scsi0",
		Agent:   "enabled=1",
		Serial0: "socket",
		VGA:     "serial0",
	}); err != nil {
		return fmt.Errorf("failed to configure VM: %w", err)
	}
	fmt.Fprintln(os.Stderr, "  VM configured")

	// Step 8: Convert to template
	fmt.Fprintln(os.Stderr, "Step 8: Converting VM to template...")
	upid, err = client.ConvertToTemplate(ctx, opts.Node, setupTemplateID)
	if err != nil {
		return fmt.Errorf("failed to convert to template: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  Convert task: %s\n", upid)
	if err := client.WaitForTask(ctx, opts.Node, upid, 2*time.Minute); err != nil {
		return fmt.Errorf("template conversion failed: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Template created successfully")
	return nil
}

func templateAlreadyExists(ctx context.Context, client proxmox.Client, node, template string) (bool, error) {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return false, err
	}
	for _, r := range resources {
		if r.Type == "qemu" && r.Node == node && r.Template && r.Name == template {
			return true, nil
		}
	}
	return false, nil
}
