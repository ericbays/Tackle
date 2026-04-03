# Tackle — Minimum Cloud IAM Permissions

This document defines the minimum IAM permissions required for each cloud provider when
configuring Tackle cloud credential sets. Follow the principle of least privilege: create
a dedicated IAM identity for Tackle and grant only the permissions listed here.

---

## AWS IAM Policy

Create a dedicated IAM user or role for Tackle. Attach the following inline or managed policy.

### Minimum IAM Policy (JSON)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "TackleEC2Operations",
      "Effect": "Allow",
      "Action": [
        "ec2:RunInstances",
        "ec2:TerminateInstances",
        "ec2:StopInstances",
        "ec2:StartInstances",
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeImages",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeKeyPairs",
        "ec2:CreateKeyPair",
        "ec2:DeleteKeyPair",
        "ec2:DescribeVpcs",
        "ec2:DescribeSubnets"
      ],
      "Resource": "*"
    },
    {
      "Sid": "TackleEC2Networking",
      "Effect": "Allow",
      "Action": [
        "ec2:AllocateAddress",
        "ec2:AssociateAddress",
        "ec2:DisassociateAddress",
        "ec2:ReleaseAddress",
        "ec2:DescribeAddresses"
      ],
      "Resource": "*"
    },
    {
      "Sid": "TackleRoute53",
      "Effect": "Allow",
      "Action": [
        "route53:ListHostedZones",
        "route53:GetHostedZone",
        "route53:ChangeResourceRecordSets",
        "route53:ListResourceRecordSets",
        "route53:GetChange"
      ],
      "Resource": "*"
    },
    {
      "Sid": "TackleSTS",
      "Effect": "Allow",
      "Action": [
        "sts:GetCallerIdentity"
      ],
      "Resource": "*"
    }
  ]
}
```

### IAM Role ARN (Optional)

If using an IAM Role ARN instead of static keys, also grant the base identity permission to assume the role:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "TackleAssumeRole",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "arn:aws:iam::<ACCOUNT_ID>:role/<TACKLE_ROLE_NAME>"
    }
  ]
}
```

The target role's trust policy must allow assumption from the Tackle user/role:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::<ACCOUNT_ID>:user/<TACKLE_USER>"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### Connection Test

Tackle uses `sts:GetCallerIdentity` to test AWS credentials. This call requires no specific
resource permissions and is always available to any valid AWS identity.

---

## Azure RBAC

Create a dedicated **App Registration** (service principal) in Azure Active Directory.
Assign the following roles to the service principal at the **subscription** scope (or narrower
if desired).

### Required Roles

| Scope | Role | Purpose |
|-------|------|---------|
| Subscription | `Virtual Machine Contributor` | Create, start, stop, delete phishing endpoint VMs |
| Subscription | `Network Contributor` | Allocate public IPs and manage NSG rules |
| Subscription | `DNS Zone Contributor` | Create and manage DNS record sets |
| Subscription | `Reader` | List resources for validation and status checks |

> **Minimum alternative:** Instead of `Virtual Machine Contributor`, you can create a custom
> role with only: `Microsoft.Compute/virtualMachines/*`, `Microsoft.Compute/disks/*`,
> `Microsoft.Network/publicIPAddresses/*`, `Microsoft.Network/networkInterfaces/*`.

### Azure CLI — Assign Roles

```bash
# Variables
SUBSCRIPTION_ID="<your-subscription-id>"
APP_OBJECT_ID="<service-principal-object-id>"

# Assign roles
az role assignment create \
  --assignee "$APP_OBJECT_ID" \
  --role "Virtual Machine Contributor" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"

az role assignment create \
  --assignee "$APP_OBJECT_ID" \
  --role "Network Contributor" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"

az role assignment create \
  --assignee "$APP_OBJECT_ID" \
  --role "DNS Zone Contributor" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"

az role assignment create \
  --assignee "$APP_OBJECT_ID" \
  --role "Reader" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"
```

### Create App Registration and Credentials

```bash
# Create app registration
az ad app create --display-name "Tackle-Infrastructure"

# Create service principal
az ad sp create --id "<app-id>"

# Create client secret (note the value — it won't be shown again)
az ad app credential reset --id "<app-id>" --append

# Get tenant ID
az account show --query tenantId -o tsv
```

### Connection Test

Tackle uses `armdns.NewZonesClient.ListPager` to test Azure credentials. This requires
the `DNS Zone Contributor` or `Reader` role on the subscription.

---

## Security Notes

- **Rotate credentials regularly.** Tackle supports credential rotation via the Update
  Credential API without downtime (new credentials are re-encrypted; status resets to `untested`).
- **Never log credentials.** Tackle masks all credential values in API responses and never
  writes them to application logs.
- **Encrypt at rest.** All credential values are encrypted with AES-256-GCM using a purpose-
  derived subkey from the platform master key before database storage.
- **Scope to minimum regions.** Add IAM condition keys (`aws:RequestedRegion`) or Azure
  management locks to restrict Tackle to the regions actually in use.
