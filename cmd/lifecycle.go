package cmd

import (
	"fmt"
	"time"

	"github.com/iamveen/devpod-proxmox-provider/pkg/options"
	"github.com/iamveen/devpod-proxmox-provider/pkg/proxmox"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a workspace VM",
	RunE:  runDelete,
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	opts := options.FromEnv()
	if err := opts.Validate(); err != nil {
		return err
	}

	client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)

	vmid, err := findVMByMachineID(ctx, opts)
	if err != nil {
		// VM not found — already deleted
		return nil
	}

	// Check current status
	s, err := client.GetVMStatus(ctx, opts.Node, vmid)
	if err != nil {
		// VM already gone
		return nil
	}

	// If running, shut it down first
	if s.Status == "running" {
		fmt.Fprintf(cmd.OutOrStdout(), "Shutting down VM %d...\n", vmid)
		_, _ = client.ShutdownVM(ctx, opts.Node, vmid)
		// Wait briefly for shutdown, but don't fail
		time.Sleep(2 * time.Second)
	}

	// Also try stop if still running
	_, _ = client.StopVM(ctx, opts.Node, vmid)

	// Destroy the VM
	upid, err := client.DeleteVM(ctx, opts.Node, vmid, true)
	if err != nil {
		return fmt.Errorf("deleting VM: %w", err)
	}
	if err := client.WaitForTask(ctx, opts.Node, upid, 5*time.Minute); err != nil {
		return fmt.Errorf("waiting for delete: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "VM %d deleted\n", vmid)
	return nil
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a stopped workspace VM",
	RunE:  runStart,
}

func init() {
	RootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	opts := options.FromEnv()
	if err := opts.Validate(); err != nil {
		return err
	}

	client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)

	vmid, err := findVMByMachineID(ctx, opts)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	upid, err := client.StartVM(ctx, opts.Node, vmid)
	if err != nil {
		return fmt.Errorf("starting VM: %w", err)
	}
	if err := client.WaitForTask(ctx, opts.Node, upid, 5*time.Minute); err != nil {
		return fmt.Errorf("waiting for VM start: %w", err)
	}
	return nil
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running workspace VM",
	RunE:  runStop,
}

func init() {
	RootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	opts := options.FromEnv()
	if err := opts.Validate(); err != nil {
		return err
	}

	client := proxmox.NewHTTPClient(opts.Host, opts.Port, opts.Token)

	vmid, err := findVMByMachineID(ctx, opts)
	if err != nil {
		return fmt.Errorf("VM not found: %w", err)
	}

	upid, err := client.ShutdownVM(ctx, opts.Node, vmid)
	if err != nil {
		return fmt.Errorf("shutting down VM: %w", err)
	}
	if err := client.WaitForTask(ctx, opts.Node, upid, 5*time.Minute); err != nil {
		return fmt.Errorf("waiting for VM shutdown: %w", err)
	}
	return nil
}

// command outputs the SSH command template for DevPod to execute on the VM.
var commandCmd = &cobra.Command{
	Use:   "command",
	Short: "Output SSH command template for DevPod helper commands",
	RunE:  runCommand,
}

func init() {
	RootCmd.AddCommand(commandCmd)
}

func runCommand(cmd *cobra.Command, args []string) error {
	// The command template uses DevPod's SSH machinery — we just output
	// what DevPod needs to run helper commands on the machine.
	fmt.Fprintln(cmd.OutOrStdout(), `"${DEVPOD}" helper sh -c "${COMMAND}"`)
	return nil
}
