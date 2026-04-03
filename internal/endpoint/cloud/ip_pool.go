package cloud

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"tackle/internal/repositories"
)

// IPPoolAllocator manages Proxmox static IP allocation from per-credential pools.
type IPPoolAllocator struct {
	repo *repositories.PhishingEndpointRepository
}

// NewIPPoolAllocator creates a new IPPoolAllocator.
func NewIPPoolAllocator(repo *repositories.PhishingEndpointRepository) *IPPoolAllocator {
	return &IPPoolAllocator{repo: repo}
}

// EnsurePoolPopulated checks if the pool for a credential has IPs, and populates it from the
// configured IP range if empty.
func (a *IPPoolAllocator) EnsurePoolPopulated(ctx context.Context, credentialID, ipStart, ipEnd string) error {
	total, _, err := a.repo.GetProxmoxPoolUtilization(ctx, credentialID)
	if err != nil {
		return fmt.Errorf("ip pool: check utilization: %w", err)
	}
	if total > 0 {
		return nil // Pool already has IPs.
	}

	ips, err := expandIPRange(ipStart, ipEnd)
	if err != nil {
		return fmt.Errorf("ip pool: expand range: %w", err)
	}

	return a.repo.PopulateProxmoxIPPool(ctx, credentialID, ips)
}

// Allocate assigns the next available IP from a credential's pool to an endpoint.
func (a *IPPoolAllocator) Allocate(ctx context.Context, credentialID, endpointID string) (string, error) {
	alloc, err := a.repo.AllocateProxmoxIP(ctx, credentialID, endpointID)
	if err != nil {
		return "", err
	}
	return alloc.IPAddress, nil
}

// Release returns an endpoint's allocated IP back to the pool.
func (a *IPPoolAllocator) Release(ctx context.Context, endpointID string) error {
	return a.repo.ReleaseProxmoxIP(ctx, endpointID)
}

// Utilization returns the total and allocated count for a credential's pool.
func (a *IPPoolAllocator) Utilization(ctx context.Context, credentialID string) (total, allocated int, err error) {
	return a.repo.GetProxmoxPoolUtilization(ctx, credentialID)
}

// expandIPRange generates all IPs between start and end (inclusive).
func expandIPRange(startStr, endStr string) ([]string, error) {
	start := net.ParseIP(startStr)
	end := net.ParseIP(endStr)
	if start == nil || end == nil {
		return nil, fmt.Errorf("invalid IP range: %s - %s", startStr, endStr)
	}

	start = start.To4()
	end = end.To4()
	if start == nil || end == nil {
		return nil, fmt.Errorf("only IPv4 ranges supported")
	}

	startInt := ipToUint32(start)
	endInt := ipToUint32(end)
	if startInt > endInt {
		return nil, fmt.Errorf("start IP %s is greater than end IP %s", startStr, endStr)
	}

	count := endInt - startInt + 1
	if count > 1024 {
		return nil, fmt.Errorf("IP range too large: %d addresses (max 1024)", count)
	}

	ips := make([]string, 0, count)
	for i := startInt; i <= endInt; i++ {
		ips = append(ips, uint32ToIP(i))
	}
	return ips, nil
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(n uint32) string {
	parts := []string{
		strconv.Itoa(int(n >> 24 & 0xFF)),
		strconv.Itoa(int(n >> 16 & 0xFF)),
		strconv.Itoa(int(n >> 8 & 0xFF)),
		strconv.Itoa(int(n & 0xFF)),
	}
	return strings.Join(parts, ".")
}
