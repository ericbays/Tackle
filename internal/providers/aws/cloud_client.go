// Package aws provides cloud infrastructure operations for Amazon Web Services.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	tacklecreds "tackle/internal/providers/credentials"
)

// CloudClient performs non-mutating AWS operations for credential validation and template field validation.
type CloudClient struct {
	cfg    aws.Config
	region string
}

// NewCloudClient creates an AWS CloudClient from the provided credentials.
// When creds.IAMRoleARN is non-empty the client will assume that role via STS AssumeRole.
func NewCloudClient(ctx context.Context, creds tacklecreds.AWSCredentials, region string) (*CloudClient, error) {
	if region == "" {
		region = "us-east-1"
	}

	staticCreds := aws.NewCredentialsCache(
		credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, ""),
	)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(staticCreds),
	)
	if err != nil {
		return nil, fmt.Errorf("aws cloud client: load config: %w", err)
	}

	if creds.IAMRoleARN != "" {
		// Assume the role using the base credentials.
		stsClient := sts.NewFromConfig(cfg)
		out, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
			RoleArn:         aws.String(creds.IAMRoleARN),
			RoleSessionName: aws.String("tackle-session"),
		})
		if err != nil {
			return nil, fmt.Errorf("aws cloud client: assume role: %w", classifyAWSError(err))
		}
		roleCreds := credentials.NewStaticCredentialsProvider(
			aws.ToString(out.Credentials.AccessKeyId),
			aws.ToString(out.Credentials.SecretAccessKey),
			aws.ToString(out.Credentials.SessionToken),
		)
		cfg.Credentials = aws.NewCredentialsCache(roleCreds)
	}

	return &CloudClient{cfg: cfg, region: region}, nil
}

// TestConnection verifies credentials by calling STS GetCallerIdentity (non-mutating).
// Returns a descriptive error if credentials are invalid or permissions are missing.
func (c *CloudClient) TestConnection(ctx context.Context) error {
	client := sts.NewFromConfig(c.cfg)
	_, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return classifyAWSError(err)
	}
	return nil
}

// ValidateRegion returns true if region is a recognized AWS region identifier.
func (c *CloudClient) ValidateRegion(region string) bool {
	return isValidAWSRegion(region)
}

// ValidateInstanceSize returns true if the given instance type is recognized.
// Uses a built-in static list for fast validation without an extra API call.
func (c *CloudClient) ValidateInstanceSize(instanceType string) bool {
	_, ok := validInstanceTypes[instanceType]
	return ok
}

// classifyAWSError returns a human-readable error from an AWS SDK error.
func classifyAWSError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "InvalidClientTokenId") || strings.Contains(msg, "InvalidAccessKeyId"):
		return fmt.Errorf("invalid AWS access key ID")
	case strings.Contains(msg, "SignatureDoesNotMatch"):
		return fmt.Errorf("invalid AWS secret access key")
	case strings.Contains(msg, "AuthFailure"):
		return fmt.Errorf("AWS authentication failure: check access key and secret")
	case strings.Contains(msg, "UnauthorizedOperation") || strings.Contains(msg, "AccessDenied"):
		return fmt.Errorf("insufficient AWS permissions")
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dial tcp"):
		return fmt.Errorf("cannot reach AWS API: check network connectivity")
	default:
		return fmt.Errorf("AWS API error: %w", err)
	}
}

// isValidAWSRegion returns true if the string is a recognized AWS region.
func isValidAWSRegion(region string) bool {
	_, ok := validAWSRegions[region]
	return ok
}

// validAWSRegions is the set of AWS regions as of 2024.
var validAWSRegions = map[string]struct{}{
	"us-east-1": {}, "us-east-2": {}, "us-west-1": {}, "us-west-2": {},
	"af-south-1": {},
	"ap-east-1": {}, "ap-south-1": {}, "ap-south-2": {},
	"ap-southeast-1": {}, "ap-southeast-2": {}, "ap-southeast-3": {}, "ap-southeast-4": {},
	"ap-northeast-1": {}, "ap-northeast-2": {}, "ap-northeast-3": {},
	"ca-central-1": {}, "ca-west-1": {},
	"eu-central-1": {}, "eu-central-2": {},
	"eu-west-1": {}, "eu-west-2": {}, "eu-west-3": {},
	"eu-north-1": {}, "eu-south-1": {}, "eu-south-2": {},
	"il-central-1": {},
	"me-central-1": {}, "me-south-1": {},
	"sa-east-1": {},
	"us-gov-east-1": {}, "us-gov-west-1": {},
}

// validInstanceTypes is a representative static list of common AWS instance types.
// A full live check would call DescribeInstanceTypes; the static list covers the common cases.
var validInstanceTypes = map[string]struct{}{
	// General purpose
	"t2.nano": {}, "t2.micro": {}, "t2.small": {}, "t2.medium": {}, "t2.large": {}, "t2.xlarge": {}, "t2.2xlarge": {},
	"t3.nano": {}, "t3.micro": {}, "t3.small": {}, "t3.medium": {}, "t3.large": {}, "t3.xlarge": {}, "t3.2xlarge": {},
	"t3a.nano": {}, "t3a.micro": {}, "t3a.small": {}, "t3a.medium": {}, "t3a.large": {}, "t3a.xlarge": {}, "t3a.2xlarge": {},
	"t4g.nano": {}, "t4g.micro": {}, "t4g.small": {}, "t4g.medium": {}, "t4g.large": {}, "t4g.xlarge": {}, "t4g.2xlarge": {},
	"m5.large": {}, "m5.xlarge": {}, "m5.2xlarge": {}, "m5.4xlarge": {},
	"m6i.large": {}, "m6i.xlarge": {}, "m6i.2xlarge": {}, "m6i.4xlarge": {},
	// Compute optimized
	"c5.large": {}, "c5.xlarge": {}, "c5.2xlarge": {}, "c5.4xlarge": {},
	"c6i.large": {}, "c6i.xlarge": {}, "c6i.2xlarge": {}, "c6i.4xlarge": {},
	// Memory optimized
	"r5.large": {}, "r5.xlarge": {}, "r5.2xlarge": {}, "r5.4xlarge": {},
}
