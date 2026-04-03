package credentials

import (
	"encoding/json"
	"fmt"
)

// rawNamecheap is the unmasked storage representation of NamecheapCredentials.
type rawNamecheap struct {
	APIUser  string `json:"api_user"`
	APIKey   string `json:"api_key"`
	Username string `json:"username"`
	ClientIP string `json:"client_ip"`
}

// rawGoDaddy is the unmasked storage representation of GoDaddyCredentials.
type rawGoDaddy struct {
	APIKey      string             `json:"api_key"`
	APISecret   string             `json:"api_secret"`
	Environment GoDaddyEnvironment `json:"environment"`
}

// rawRoute53 is the unmasked storage representation of Route53Credentials.
type rawRoute53 struct {
	AccessKeyID     string `json:"aws_access_key_id"`
	SecretAccessKey string `json:"aws_secret_access_key"`
	Region          string `json:"region"`
	IAMRoleARN      string `json:"iam_role_arn,omitempty"`
}

// rawAzureDNS is the unmasked storage representation of AzureDNSCredentials.
type rawAzureDNS struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
	ResourceGroup  string `json:"resource_group"`
}

// rawAWS is the unmasked storage representation of AWSCredentials.
type rawAWS struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	IAMRoleARN      string `json:"iam_role_arn,omitempty"`
}

// rawAzure is the unmasked storage representation of AzureCredentials.
type rawAzure struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
}

// marshalRaw serializes a credential struct using its unmasked raw form.
// This is used internally only for encryption — never exposed via API.
func marshalRaw(creds any) ([]byte, error) {
	switch c := creds.(type) {
	case NamecheapCredentials:
		return json.Marshal(rawNamecheap{APIUser: c.APIUser, APIKey: c.APIKey, Username: c.Username, ClientIP: c.ClientIP})
	case *NamecheapCredentials:
		return json.Marshal(rawNamecheap{APIUser: c.APIUser, APIKey: c.APIKey, Username: c.Username, ClientIP: c.ClientIP})
	case GoDaddyCredentials:
		return json.Marshal(rawGoDaddy{APIKey: c.APIKey, APISecret: c.APISecret, Environment: c.Environment})
	case *GoDaddyCredentials:
		return json.Marshal(rawGoDaddy{APIKey: c.APIKey, APISecret: c.APISecret, Environment: c.Environment})
	case Route53Credentials:
		return json.Marshal(rawRoute53{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, Region: c.Region, IAMRoleARN: c.IAMRoleARN})
	case *Route53Credentials:
		return json.Marshal(rawRoute53{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, Region: c.Region, IAMRoleARN: c.IAMRoleARN})
	case AzureDNSCredentials:
		return json.Marshal(rawAzureDNS{TenantID: c.TenantID, ClientID: c.ClientID, ClientSecret: c.ClientSecret, SubscriptionID: c.SubscriptionID, ResourceGroup: c.ResourceGroup})
	case *AzureDNSCredentials:
		return json.Marshal(rawAzureDNS{TenantID: c.TenantID, ClientID: c.ClientID, ClientSecret: c.ClientSecret, SubscriptionID: c.SubscriptionID, ResourceGroup: c.ResourceGroup})
	case AWSCredentials:
		return json.Marshal(rawAWS{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, IAMRoleARN: c.IAMRoleARN})
	case *AWSCredentials:
		return json.Marshal(rawAWS{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, IAMRoleARN: c.IAMRoleARN})
	case AzureCredentials:
		return json.Marshal(rawAzure{TenantID: c.TenantID, ClientID: c.ClientID, ClientSecret: c.ClientSecret, SubscriptionID: c.SubscriptionID})
	case *AzureCredentials:
		return json.Marshal(rawAzure{TenantID: c.TenantID, ClientID: c.ClientID, ClientSecret: c.ClientSecret, SubscriptionID: c.SubscriptionID})
	default:
		return nil, fmt.Errorf("unsupported credentials type %T", creds)
	}
}

// unmarshalRaw deserializes JSON into a credential struct pointer using its unmasked raw form.
func unmarshalRaw(data []byte, target any) error {
	switch t := target.(type) {
	case *NamecheapCredentials:
		var r rawNamecheap
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.APIUser = r.APIUser
		t.APIKey = r.APIKey
		t.Username = r.Username
		t.ClientIP = r.ClientIP
		return nil
	case *GoDaddyCredentials:
		var r rawGoDaddy
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.APIKey = r.APIKey
		t.APISecret = r.APISecret
		t.Environment = r.Environment
		return nil
	case *Route53Credentials:
		var r rawRoute53
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.AccessKeyID = r.AccessKeyID
		t.SecretAccessKey = r.SecretAccessKey
		t.Region = r.Region
		t.IAMRoleARN = r.IAMRoleARN
		return nil
	case *AzureDNSCredentials:
		var r rawAzureDNS
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.TenantID = r.TenantID
		t.ClientID = r.ClientID
		t.ClientSecret = r.ClientSecret
		t.SubscriptionID = r.SubscriptionID
		t.ResourceGroup = r.ResourceGroup
		return nil
	case *AWSCredentials:
		var r rawAWS
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.AccessKeyID = r.AccessKeyID
		t.SecretAccessKey = r.SecretAccessKey
		t.IAMRoleARN = r.IAMRoleARN
		return nil
	case *AzureCredentials:
		var r rawAzure
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		t.TenantID = r.TenantID
		t.ClientID = r.ClientID
		t.ClientSecret = r.ClientSecret
		t.SubscriptionID = r.SubscriptionID
		return nil
	default:
		return fmt.Errorf("unsupported target type %T", target)
	}
}
