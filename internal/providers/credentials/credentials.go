// Package credentials defines per-provider credential structs used by domain provider connections.
// All credential types implement json.Marshaler to return a masked representation,
// preventing accidental exposure in logs, error messages, or API responses.
package credentials

import "encoding/json"

// masked is the string used to replace sensitive field values in safe output.
const masked = "***"

// NamecheapCredentials holds API credentials for the Namecheap XML API.
type NamecheapCredentials struct {
	APIUser  string `json:"api_user"`
	APIKey   string `json:"api_key"`
	Username string `json:"username"`
	ClientIP string `json:"client_ip"`
}

// MarshalJSON returns a representation with all sensitive fields masked.
func (c NamecheapCredentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		APIUser  string `json:"api_user"`
		APIKey   string `json:"api_key"`
		Username string `json:"username"`
		ClientIP string `json:"client_ip"`
	}{
		APIUser:  masked,
		APIKey:   masked,
		Username: masked,
		ClientIP: c.ClientIP, // IP is not secret; useful for debugging whitelist issues
	})
}

// String returns a masked representation to prevent leaking credentials via fmt.
func (c NamecheapCredentials) String() string { return "<NamecheapCredentials masked>" }

// GoDaddyEnvironment indicates which GoDaddy API environment to use.
type GoDaddyEnvironment string

const (
	// GoDaddyProduction is the live GoDaddy API.
	GoDaddyProduction GoDaddyEnvironment = "production"
	// GoDaddyOTE is the GoDaddy test/OTE environment.
	GoDaddyOTE GoDaddyEnvironment = "ote"
)

// GoDaddyCredentials holds API credentials for the GoDaddy REST API v1.
type GoDaddyCredentials struct {
	APIKey      string             `json:"api_key"`
	APISecret   string             `json:"api_secret"`
	Environment GoDaddyEnvironment `json:"environment"`
}

// MarshalJSON returns a representation with all sensitive fields masked.
func (c GoDaddyCredentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		APIKey      string             `json:"api_key"`
		APISecret   string             `json:"api_secret"`
		Environment GoDaddyEnvironment `json:"environment"`
	}{
		APIKey:      masked,
		APISecret:   masked,
		Environment: c.Environment,
	})
}

// String returns a masked representation.
func (c GoDaddyCredentials) String() string { return "<GoDaddyCredentials masked>" }

// Route53Credentials holds access credentials for the AWS Route 53 API.
// IAMRoleARN is optional; when set, the client assumes the role.
type Route53Credentials struct {
	AccessKeyID     string `json:"aws_access_key_id"`
	SecretAccessKey string `json:"aws_secret_access_key"`
	Region          string `json:"region"`
	IAMRoleARN      string `json:"iam_role_arn,omitempty"`
}

// MarshalJSON returns a representation with all sensitive fields masked.
func (c Route53Credentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		AccessKeyID     string `json:"aws_access_key_id"`
		SecretAccessKey string `json:"aws_secret_access_key"`
		Region          string `json:"region"`
		IAMRoleARN      string `json:"iam_role_arn,omitempty"`
	}{
		AccessKeyID:     masked,
		SecretAccessKey: masked,
		Region:          c.Region,
		IAMRoleARN:      c.IAMRoleARN,
	})
}

// String returns a masked representation.
func (c Route53Credentials) String() string { return "<Route53Credentials masked>" }

// AzureDNSCredentials holds service principal credentials for the Azure DNS API.
type AzureDNSCredentials struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
	ResourceGroup  string `json:"resource_group"`
}

// MarshalJSON returns a representation with all sensitive fields masked.
func (c AzureDNSCredentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		TenantID       string `json:"tenant_id"`
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		SubscriptionID string `json:"subscription_id"`
		ResourceGroup  string `json:"resource_group"`
	}{
		TenantID:       masked,
		ClientID:       masked,
		ClientSecret:   masked,
		SubscriptionID: c.SubscriptionID,
		ResourceGroup:  c.ResourceGroup,
	})
}

// String returns a masked representation.
func (c AzureDNSCredentials) String() string { return "<AzureDNSCredentials masked>" }

// AWSCredentials holds access credentials for AWS cloud infrastructure operations.
// IAMRoleARN is optional; when set the client assumes the specified role.
type AWSCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	IAMRoleARN      string `json:"iam_role_arn,omitempty"`
}

// MarshalJSON returns a masked representation.
func (c AWSCredentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		AccessKeyID     string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_access_key"`
		IAMRoleARN      string `json:"iam_role_arn,omitempty"`
	}{
		AccessKeyID:     masked,
		SecretAccessKey: masked,
		IAMRoleARN:      c.IAMRoleARN,
	})
}

// String returns a masked representation.
func (c AWSCredentials) String() string { return "<AWSCredentials masked>" }

// ProxmoxCredentials holds API token credentials for a Proxmox VE hypervisor.
type ProxmoxCredentials struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	TokenID      string `json:"token_id"`
	TokenSecret  string `json:"token_secret"`
	Node         string `json:"node"`
	TemplateVMID int    `json:"template_vmid"`
	Bridge       string `json:"bridge"`
	IPPoolStart  string `json:"ip_pool_start"`
	IPPoolEnd    string `json:"ip_pool_end"`
	Gateway      string `json:"gateway"`
	SubnetMask   string `json:"subnet_mask"`
}

// MarshalJSON returns a masked representation.
func (c ProxmoxCredentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Host         string `json:"host"`
		Port         int    `json:"port"`
		TokenID      string `json:"token_id"`
		TokenSecret  string `json:"token_secret"`
		Node         string `json:"node"`
		TemplateVMID int    `json:"template_vmid"`
		Bridge       string `json:"bridge"`
		IPPoolStart  string `json:"ip_pool_start"`
		IPPoolEnd    string `json:"ip_pool_end"`
		Gateway      string `json:"gateway"`
		SubnetMask   string `json:"subnet_mask"`
	}{
		Host:         c.Host,
		Port:         c.Port,
		TokenID:      masked,
		TokenSecret:  masked,
		Node:         c.Node,
		TemplateVMID: c.TemplateVMID,
		Bridge:       c.Bridge,
		IPPoolStart:  c.IPPoolStart,
		IPPoolEnd:    c.IPPoolEnd,
		Gateway:      c.Gateway,
		SubnetMask:   c.SubnetMask,
	})
}

// String returns a masked representation.
func (c ProxmoxCredentials) String() string { return "<ProxmoxCredentials masked>" }

// AzureCredentials holds service principal credentials for Azure cloud infrastructure operations.
type AzureCredentials struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
}

// MarshalJSON returns a masked representation.
func (c AzureCredentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		TenantID       string `json:"tenant_id"`
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		SubscriptionID string `json:"subscription_id"`
	}{
		TenantID:       masked,
		ClientID:       masked,
		ClientSecret:   masked,
		SubscriptionID: c.SubscriptionID,
	})
}

// String returns a masked representation.
func (c AzureCredentials) String() string { return "<AzureCredentials masked>" }
