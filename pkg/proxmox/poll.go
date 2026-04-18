package proxmox

import (
	"context"
	"fmt"
	"time"
)

// WaitForIP polls the QEMU guest agent until a non-loopback IPv4 address is found.
// Returns the first non-loopback IPv4 address, or an error after timeout.
func WaitForIP(ctx context.Context, client Client, node string, vmid int, timeout time.Duration) (string, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", fmt.Errorf("timed out waiting for VM IP after %v", timeout)
		case <-ticker.C:
			ifaces, err := client.GetNetworkInterfaces(ctx, node, vmid)
			if err != nil {
				// Agent may not be ready yet — keep trying
				continue
			}
			if ip := findNonLoopbackIPv4(ifaces); ip != "" {
				return ip, nil
			}
		}
	}
}

func findNonLoopbackIPv4(interfaces []NetworkInterface) string {
	for _, iface := range interfaces {
		if iface.Name == "lo" {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.Type == "ipv4" && addr.IPAddress != "127.0.0.1" {
				return addr.IPAddress
			}
		}
	}
	return ""
}
