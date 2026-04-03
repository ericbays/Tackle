package cloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	tacklecreds "tackle/internal/providers/credentials"
)

// AWSProvider implements the Provider interface for Amazon Web Services EC2.
type AWSProvider struct {
	cfg    aws.Config
	region string
}

// NewAWSProvider creates an AWS provider from the given credentials and region.
func NewAWSProvider(ctx context.Context, creds tacklecreds.AWSCredentials, region string) (*AWSProvider, error) {
	if region == "" {
		region = "us-east-1"
	}

	staticCreds := aws.NewCredentialsCache(
		credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, ""),
	)

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(staticCreds),
	)
	if err != nil {
		return nil, fmt.Errorf("aws provider: load config: %w", err)
	}

	if creds.IAMRoleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		out, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
			RoleArn:         aws.String(creds.IAMRoleARN),
			RoleSessionName: aws.String("tackle-endpoint"),
		})
		if err != nil {
			return nil, fmt.Errorf("aws provider: assume role: %w", err)
		}
		roleCreds := credentials.NewStaticCredentialsProvider(
			aws.ToString(out.Credentials.AccessKeyId),
			aws.ToString(out.Credentials.SecretAccessKey),
			aws.ToString(out.Credentials.SessionToken),
		)
		cfg.Credentials = aws.NewCredentialsCache(roleCreds)
	}

	return &AWSProvider{cfg: cfg, region: region}, nil
}

// ProviderName returns "aws".
func (p *AWSProvider) ProviderName() string { return "aws" }

// ProvisionInstance creates an EC2 instance with the given configuration.
func (p *AWSProvider) ProvisionInstance(ctx context.Context, config ProvisionConfig) (string, error) {
	client := ec2.NewFromConfig(p.cfg)

	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(config.OSImage),
		InstanceType: types.InstanceType(config.InstanceSize),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	}

	if config.SubnetID != "" {
		input.SubnetId = aws.String(config.SubnetID)
	}

	if len(config.SecurityGroups) > 0 {
		input.SecurityGroupIds = config.SecurityGroups
	}

	if config.UserData != "" {
		input.UserData = aws.String(config.UserData)
	}

	if config.SSHPublicKey != "" {
		input.KeyName = aws.String(config.SSHPublicKey)
	}

	// Convert tags.
	if len(config.Tags) > 0 {
		var tags []types.Tag
		for k, v := range config.Tags {
			tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		input.TagSpecifications = []types.TagSpecification{
			{ResourceType: types.ResourceTypeInstance, Tags: tags},
		}
	}

	result, err := client.RunInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("aws provider: run instances: %w", classifyAWSError(err))
	}

	if len(result.Instances) == 0 {
		return "", fmt.Errorf("aws provider: no instances returned")
	}

	return aws.ToString(result.Instances[0].InstanceId), nil
}

// GetInstanceStatus returns the current status of an EC2 instance.
func (p *AWSProvider) GetInstanceStatus(ctx context.Context, instanceID string) (InstanceStatus, error) {
	client := ec2.NewFromConfig(p.cfg)

	result, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return InstanceStatus{}, fmt.Errorf("aws provider: describe instances: %w", classifyAWSError(err))
	}

	for _, res := range result.Reservations {
		for _, inst := range res.Instances {
			status := InstanceStatus{
				InstanceID: aws.ToString(inst.InstanceId),
				State:      normalizeAWSState(inst.State),
			}
			if inst.PublicIpAddress != nil {
				status.PublicIP = aws.ToString(inst.PublicIpAddress)
			}
			return status, nil
		}
	}

	return InstanceStatus{}, fmt.Errorf("aws provider: instance %s not found", instanceID)
}

// StopInstance stops an EC2 instance.
func (p *AWSProvider) StopInstance(ctx context.Context, instanceID string) error {
	client := ec2.NewFromConfig(p.cfg)
	_, err := client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("aws provider: stop instance: %w", classifyAWSError(err))
	}
	return nil
}

// StartInstance starts a stopped EC2 instance.
func (p *AWSProvider) StartInstance(ctx context.Context, instanceID string) error {
	client := ec2.NewFromConfig(p.cfg)
	_, err := client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("aws provider: start instance: %w", classifyAWSError(err))
	}
	return nil
}

// TerminateInstance terminates an EC2 instance permanently.
func (p *AWSProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	client := ec2.NewFromConfig(p.cfg)
	_, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("aws provider: terminate instance: %w", classifyAWSError(err))
	}
	return nil
}

// AllocateStaticIP allocates a new Elastic IP.
func (p *AWSProvider) AllocateStaticIP(ctx context.Context) (StaticIPResult, error) {
	client := ec2.NewFromConfig(p.cfg)
	result, err := client.AllocateAddress(ctx, &ec2.AllocateAddressInput{
		Domain: types.DomainTypeVpc,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeElasticIp,
				Tags: []types.Tag{
					{Key: aws.String("managed-by"), Value: aws.String("tackle")},
				},
			},
		},
	})
	if err != nil {
		return StaticIPResult{}, fmt.Errorf("aws provider: allocate address: %w", classifyAWSError(err))
	}
	return StaticIPResult{
		IP:           aws.ToString(result.PublicIp),
		AllocationID: aws.ToString(result.AllocationId),
	}, nil
}

// AssociateStaticIP associates an Elastic IP with an EC2 instance.
func (p *AWSProvider) AssociateStaticIP(ctx context.Context, instanceID, allocationID string) error {
	client := ec2.NewFromConfig(p.cfg)
	_, err := client.AssociateAddress(ctx, &ec2.AssociateAddressInput{
		InstanceId:   aws.String(instanceID),
		AllocationId: aws.String(allocationID),
	})
	if err != nil {
		return fmt.Errorf("aws provider: associate address: %w", classifyAWSError(err))
	}
	return nil
}

// ReleaseStaticIP releases an Elastic IP.
func (p *AWSProvider) ReleaseStaticIP(ctx context.Context, allocationID string) error {
	client := ec2.NewFromConfig(p.cfg)
	_, err := client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
		AllocationId: aws.String(allocationID),
	})
	if err != nil {
		return fmt.Errorf("aws provider: release address: %w", classifyAWSError(err))
	}
	return nil
}

func normalizeAWSState(state *types.InstanceState) string {
	if state == nil {
		return "unknown"
	}
	switch state.Name {
	case types.InstanceStateNameRunning:
		return "running"
	case types.InstanceStateNameStopped:
		return "stopped"
	case types.InstanceStateNameTerminated:
		return "terminated"
	case types.InstanceStateNamePending:
		return "pending"
	case types.InstanceStateNameStopping:
		return "stopped"
	case types.InstanceStateNameShuttingDown:
		return "terminated"
	default:
		return "unknown"
	}
}

func classifyAWSError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "InvalidClientTokenId") || strings.Contains(msg, "InvalidAccessKeyId"):
		return fmt.Errorf("invalid AWS access key ID")
	case strings.Contains(msg, "SignatureDoesNotMatch"):
		return fmt.Errorf("invalid AWS secret access key")
	case strings.Contains(msg, "AuthFailure"):
		return fmt.Errorf("AWS authentication failure")
	case strings.Contains(msg, "UnauthorizedOperation") || strings.Contains(msg, "AccessDenied"):
		return fmt.Errorf("insufficient AWS permissions")
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dial tcp"):
		return fmt.Errorf("cannot reach AWS API: check network connectivity")
	default:
		return err
	}
}
