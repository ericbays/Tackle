// Package cloud defines the cloud provider interface for endpoint provisioning
// and implements it for AWS EC2, Azure VM, and Proxmox VM.
package cloud

import "context"

// ProvisionConfig holds the parameters needed to provision a cloud VM for a phishing endpoint.
type ProvisionConfig struct {
	// Region or node name for the cloud provider.
	Region string
	// InstanceSize is the VM size/type (e.g., "t3.micro", "Standard_B1s", "2c-2g").
	InstanceSize string
	// OSImage is the AMI ID, Azure image reference, or Proxmox template VMID.
	OSImage string
	// SSHPublicKey is the public key to inject for initial SSH access.
	SSHPublicKey string
	// Tags are key-value pairs for VM identification and audit traceability.
	Tags map[string]string
	// UserData is the cloud-init or startup script content.
	UserData string
	// SecurityGroups (AWS) or NSG (Azure) identifiers.
	SecurityGroups []string
	// SubnetID (AWS/Azure) for VPC placement.
	SubnetID string
	// ResourceGroup (Azure) for resource placement.
	ResourceGroup string
}

// InstanceStatus represents the current status of a cloud VM.
type InstanceStatus struct {
	// State is a normalized state string: "running", "stopped", "terminated", "pending", "unknown".
	State string
	// PublicIP is the public IP address if available.
	PublicIP string
	// InstanceID is the cloud provider's instance identifier.
	InstanceID string
}

// StaticIPResult holds the result of a static IP allocation.
type StaticIPResult struct {
	// IP is the allocated static IP address.
	IP string
	// AllocationID is the provider-specific identifier for the allocation (Elastic IP allocation ID, etc.).
	AllocationID string
}

// Provider defines the interface for cloud infrastructure operations needed for endpoint provisioning.
type Provider interface {
	// ProvisionInstance creates a new VM and returns the cloud provider's instance ID.
	ProvisionInstance(ctx context.Context, config ProvisionConfig) (instanceID string, err error)
	// GetInstanceStatus returns the current status of a cloud VM.
	GetInstanceStatus(ctx context.Context, instanceID string) (InstanceStatus, error)
	// StopInstance stops a running VM (preserves disk/state).
	StopInstance(ctx context.Context, instanceID string) error
	// StartInstance starts a stopped VM.
	StartInstance(ctx context.Context, instanceID string) error
	// TerminateInstance permanently destroys a VM.
	TerminateInstance(ctx context.Context, instanceID string) error
	// AllocateStaticIP allocates a new static/elastic IP.
	AllocateStaticIP(ctx context.Context) (StaticIPResult, error)
	// AssociateStaticIP associates a static IP with an instance.
	AssociateStaticIP(ctx context.Context, instanceID, allocationID string) error
	// ReleaseStaticIP releases a static IP back to the provider.
	ReleaseStaticIP(ctx context.Context, allocationID string) error
	// ProviderName returns the cloud provider name ("aws", "azure", "proxmox").
	ProviderName() string
}
