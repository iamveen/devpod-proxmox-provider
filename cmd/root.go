package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/iamveen/devpod-proxmox-provider/pkg/options"
	"github.com/iamveen/devpod-proxmox-provider/pkg/proxmox"
	"github.com/spf13/cobra"
)

var (
	versionStr = "dev"

	// Root command is exported for testing.
	RootCmd = &cobra.Command{
		Use:   "proxmox-provider",
		Short: "DevPod provider for Proxmox VE",
		Long:  "Provisions KVM virtual machines on Proxmox VE to host DevPod development workspaces.",
		Version: versionStr,
	}
)

// Execute is the entry point called from main.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.SetOut(os.Stdout)
	RootCmd.SetErr(os.Stderr)
}

// versionCmd prints the binary version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the provider version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), versionStr)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

// initCmd validates Proxmox connectivity.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize and validate the Proxmox provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		opts := options.FromEnv()
		if err := opts.Validate(); err != nil {
			return err
		}
		client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)
		v, err := client.GetVersion(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to Proxmox: %w", err)
		}
		_, err = client.GetClusterResources(ctx)
		if err != nil {
			return fmt.Errorf("failed to list cluster resources: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Connected to Proxmox %s (%s)\n", v.Version, v.Release)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

// statusCmd reports the VM status to DevPod.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report the status of a workspace VM",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		opts := options.FromEnv()
		if err := opts.Validate(); err != nil {
			return err
		}

		// Find the VM by machine ID
		vmid, err := findVMByMachineID(ctx, opts)
		if err != nil {
			// VM not found
			fmt.Fprintln(cmd.OutOrStdout(), "notfound")
			return nil
		}

		client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)
		s, err := client.GetVMStatus(ctx, opts.Node, vmid)
		if err != nil {
			// 404 / not found from API
			fmt.Fprintln(cmd.OutOrStdout(), "notfound")
			return nil
		}

		switch s.Status {
		case "running":
			fmt.Fprintln(cmd.OutOrStdout(), "running")
		case "stopped":
			fmt.Fprintln(cmd.OutOrStdout(), "stopped")
		default:
			fmt.Fprintln(cmd.OutOrStdout(), "busy")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}

// findVMByMachineID scans cluster resources for a VM matching the machine ID.
func findVMByMachineID(ctx context.Context, opts options.Options) (int, error) {
	client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return 0, err
	}

	expectedName := "devpod-" + opts.MachineID
	for _, r := range resources {
		if r.Type == "qemu" && r.Name == expectedName && r.Node == opts.Node {
			return r.VMID, nil
		}
	}
	return 0, fmt.Errorf("VM not found: %s", expectedName)
}
