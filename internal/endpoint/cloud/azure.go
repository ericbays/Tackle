package cloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"

	tacklecreds "tackle/internal/providers/credentials"
)

// AzureProvider implements the Provider interface for Microsoft Azure VMs.
type AzureProvider struct {
	cred           *azidentity.ClientSecretCredential
	subscriptionID string
	resourceGroup  string
	region         string
}

// NewAzureProvider creates an Azure provider from the given credentials.
func NewAzureProvider(creds tacklecreds.AzureCredentials, region, resourceGroup string) (*AzureProvider, error) {
	cred, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("azure provider: create credential: %w", err)
	}
	return &AzureProvider{
		cred:           cred,
		subscriptionID: creds.SubscriptionID,
		resourceGroup:  resourceGroup,
		region:         region,
	}, nil
}

// ProviderName returns "azure".
func (p *AzureProvider) ProviderName() string { return "azure" }

// ProvisionInstance creates an Azure VM.
func (p *AzureProvider) ProvisionInstance(ctx context.Context, config ProvisionConfig) (string, error) {
	client, err := armcompute.NewVirtualMachinesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return "", fmt.Errorf("azure provider: create vm client: %w", err)
	}

	vmName := "tackle-ep-" + generateShortID()
	rg := config.ResourceGroup
	if rg == "" {
		rg = p.resourceGroup
	}
	region := config.Region
	if region == "" {
		region = p.region
	}

	// Build tags map.
	tags := make(map[string]*string)
	for k, v := range config.Tags {
		tags[k] = to.Ptr(v)
	}
	tags["managed-by"] = to.Ptr("tackle")

	// Parse image reference (publisher:offer:sku:version).
	imgParts := strings.SplitN(config.OSImage, ":", 4)
	var imageRef *armcompute.ImageReference
	if len(imgParts) == 4 {
		imageRef = &armcompute.ImageReference{
			Publisher: to.Ptr(imgParts[0]),
			Offer:     to.Ptr(imgParts[1]),
			SKU:       to.Ptr(imgParts[2]),
			Version:   to.Ptr(imgParts[3]),
		}
	} else {
		imageRef = &armcompute.ImageReference{ID: to.Ptr(config.OSImage)}
	}

	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(region),
		Tags:     tags,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(config.InstanceSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: imageRef,
				OSDisk: &armcompute.OSDisk{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(vmName),
				AdminUsername: to.Ptr("tackleadmin"),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr("/home/tackleadmin/.ssh/authorized_keys"),
								KeyData: to.Ptr(config.SSHPublicKey),
							},
						},
					},
				},
			},
		},
	}

	// Add network interface if subnet specified.
	if config.SubnetID != "" {
		vmParams.Properties.NetworkProfile = &armcompute.NetworkProfile{
			NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
				{ID: to.Ptr(config.SubnetID)},
			},
		}
	}

	poller, err := client.BeginCreateOrUpdate(ctx, rg, vmName, vmParams, nil)
	if err != nil {
		return "", fmt.Errorf("azure provider: begin create vm: %w", classifyAzureError(err))
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("azure provider: create vm: %w", classifyAzureError(err))
	}

	if result.ID == nil {
		return "", fmt.Errorf("azure provider: vm created but no ID returned")
	}
	return *result.ID, nil
}

// GetInstanceStatus returns the current status of an Azure VM.
func (p *AzureProvider) GetInstanceStatus(ctx context.Context, instanceID string) (InstanceStatus, error) {
	client, err := armcompute.NewVirtualMachinesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return InstanceStatus{}, fmt.Errorf("azure provider: create vm client: %w", err)
	}

	// Extract resource group and VM name from the full resource ID.
	rg, vmName := parseAzureResourceID(instanceID)
	if rg == "" || vmName == "" {
		return InstanceStatus{}, fmt.Errorf("azure provider: cannot parse resource ID %q", instanceID)
	}

	result, err := client.InstanceView(ctx, rg, vmName, nil)
	if err != nil {
		return InstanceStatus{}, fmt.Errorf("azure provider: get instance view: %w", classifyAzureError(err))
	}

	status := InstanceStatus{InstanceID: instanceID, State: "unknown"}
	if result.Statuses != nil {
		for _, s := range result.Statuses {
			if s.Code != nil && strings.HasPrefix(*s.Code, "PowerState/") {
				status.State = normalizeAzureState(*s.Code)
			}
		}
	}

	return status, nil
}

// StopInstance deallocates an Azure VM.
func (p *AzureProvider) StopInstance(ctx context.Context, instanceID string) error {
	client, err := armcompute.NewVirtualMachinesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return fmt.Errorf("azure provider: create vm client: %w", err)
	}

	rg, vmName := parseAzureResourceID(instanceID)
	poller, err := client.BeginDeallocate(ctx, rg, vmName, nil)
	if err != nil {
		return fmt.Errorf("azure provider: begin deallocate: %w", classifyAzureError(err))
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure provider: deallocate: %w", classifyAzureError(err))
	}
	return nil
}

// StartInstance starts a deallocated Azure VM.
func (p *AzureProvider) StartInstance(ctx context.Context, instanceID string) error {
	client, err := armcompute.NewVirtualMachinesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return fmt.Errorf("azure provider: create vm client: %w", err)
	}

	rg, vmName := parseAzureResourceID(instanceID)
	poller, err := client.BeginStart(ctx, rg, vmName, nil)
	if err != nil {
		return fmt.Errorf("azure provider: begin start: %w", classifyAzureError(err))
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure provider: start: %w", classifyAzureError(err))
	}
	return nil
}

// TerminateInstance deletes an Azure VM permanently.
func (p *AzureProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	client, err := armcompute.NewVirtualMachinesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return fmt.Errorf("azure provider: create vm client: %w", err)
	}

	rg, vmName := parseAzureResourceID(instanceID)
	poller, err := client.BeginDelete(ctx, rg, vmName, nil)
	if err != nil {
		return fmt.Errorf("azure provider: begin delete: %w", classifyAzureError(err))
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure provider: delete: %w", classifyAzureError(err))
	}
	return nil
}

// AllocateStaticIP allocates a new Azure static public IP.
func (p *AzureProvider) AllocateStaticIP(ctx context.Context) (StaticIPResult, error) {
	client, err := armnetwork.NewPublicIPAddressesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return StaticIPResult{}, fmt.Errorf("azure provider: create ip client: %w", err)
	}

	ipName := "tackle-ip-" + generateShortID()
	poller, err := client.BeginCreateOrUpdate(ctx, p.resourceGroup, ipName, armnetwork.PublicIPAddress{
		Location: to.Ptr(p.region),
		Tags:     map[string]*string{"managed-by": to.Ptr("tackle")},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
	}, nil)
	if err != nil {
		return StaticIPResult{}, fmt.Errorf("azure provider: begin create ip: %w", classifyAzureError(err))
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return StaticIPResult{}, fmt.Errorf("azure provider: create ip: %w", classifyAzureError(err))
	}

	ip := ""
	if result.Properties != nil && result.Properties.IPAddress != nil {
		ip = *result.Properties.IPAddress
	}

	return StaticIPResult{
		IP:           ip,
		AllocationID: *result.ID,
	}, nil
}

// AssociateStaticIP associates a static IP with an Azure VM via its NIC.
func (p *AzureProvider) AssociateStaticIP(ctx context.Context, instanceID, allocationID string) error {
	// Azure IP association is done through NIC configuration, which is typically set
	// during provisioning. This is a placeholder for post-provisioning association.
	// In practice, the NIC's IP configuration is updated to reference the static IP.
	return nil
}

// ReleaseStaticIP releases an Azure static public IP.
func (p *AzureProvider) ReleaseStaticIP(ctx context.Context, allocationID string) error {
	client, err := armnetwork.NewPublicIPAddressesClient(p.subscriptionID, p.cred, nil)
	if err != nil {
		return fmt.Errorf("azure provider: create ip client: %w", err)
	}

	rg, ipName := parseAzureResourceID(allocationID)
	poller, err := client.BeginDelete(ctx, rg, ipName, nil)
	if err != nil {
		return fmt.Errorf("azure provider: begin delete ip: %w", classifyAzureError(err))
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure provider: delete ip: %w", classifyAzureError(err))
	}
	return nil
}

func normalizeAzureState(code string) string {
	switch code {
	case "PowerState/running":
		return "running"
	case "PowerState/deallocated", "PowerState/stopped":
		return "stopped"
	case "PowerState/deallocating", "PowerState/stopping":
		return "stopped"
	case "PowerState/starting":
		return "pending"
	default:
		return "unknown"
	}
}

// parseAzureResourceID extracts resource group and resource name from an Azure resource ID.
// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/{provider}/{type}/{name}
func parseAzureResourceID(resourceID string) (resourceGroup, name string) {
	parts := strings.Split(resourceID, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			resourceGroup = parts[i+1]
		}
	}
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}
	return resourceGroup, name
}

func classifyAzureError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "AADSTS70011") || strings.Contains(msg, "invalid_client"):
		return fmt.Errorf("invalid Azure client credentials")
	case strings.Contains(msg, "AuthorizationFailed"):
		return fmt.Errorf("insufficient Azure permissions")
	case strings.Contains(msg, "SubscriptionNotFound"):
		return fmt.Errorf("Azure subscription not found")
	case strings.Contains(msg, "ResourceGroupNotFound"):
		return fmt.Errorf("Azure resource group not found")
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dial"):
		return fmt.Errorf("cannot reach Azure API")
	default:
		return err
	}
}
