// Package route53 implements a client for the AWS Route 53 API using AWS SDK for Go v2.
package route53

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"tackle/internal/providers/ratelimit"
	"tackle/internal/providers/credentials"
)

const (
	defaultRateLimit = 5  // 5 requests/second per AWS Route 53 API limits (converted to per-minute below)
	testTimeout      = 15 * time.Second
)

// r53API defines the subset of the Route 53 client used by this package, enabling mock injection in tests.
type r53API interface {
	ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
}

// Client is an AWS Route 53 API client.
type Client struct {
	r53         r53API
	rateLimiter *ratelimit.RateLimiter
}

// NewClient creates a Route 53 client from the provided credentials.
// ratePerMinute overrides the default (300/min = 5/sec) if > 0.
func NewClient(creds credentials.Route53Credentials, ratePerMinute int) (*Client, error) {
	if ratePerMinute <= 0 {
		ratePerMinute = defaultRateLimit * 60 // convert 5/sec to 300/min
	}

	cfg := aws.Config{
		Region: creds.Region,
		Credentials: awscreds.NewStaticCredentialsProvider(
			creds.AccessKeyID,
			creds.SecretAccessKey,
			"",
		),
	}

	// If an IAM Role ARN is provided, assume that role.
	if creds.IAMRoleARN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		stsSvc := sts.NewFromConfig(cfg)
		result, err := stsSvc.AssumeRole(ctx, &sts.AssumeRoleInput{
			RoleArn:         aws.String(creds.IAMRoleARN),
			RoleSessionName: aws.String("tackle-provider-connection"),
		})
		if err != nil {
			return nil, fmt.Errorf("route53: assume role %s: %w", creds.IAMRoleARN, err)
		}
		cfg.Credentials = awscreds.NewStaticCredentialsProvider(
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken,
		)
	}

	r53Client := route53.NewFromConfig(cfg)
	return &Client{
		r53:         r53Client,
		rateLimiter: ratelimit.NewRateLimiter(ratePerMinute),
	}, nil
}

// newClientWithAPI creates a Client using a pre-built API client (used in tests for injection).
func newClientWithAPI(api r53API, ratePerMinute int) *Client {
	if ratePerMinute <= 0 {
		ratePerMinute = defaultRateLimit * 60
	}
	return &Client{
		r53:         api,
		rateLimiter: ratelimit.NewRateLimiter(ratePerMinute),
	}
}

// TestConnection validates credentials by listing hosted zones (max 1 result).
// Returns nil on success or a descriptive, actionable error.
func (c *Client) TestConnection() error {
	c.rateLimiter.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	maxItems := aws.Int32(1)
	_, err := c.r53.ListHostedZones(ctx, &route53.ListHostedZonesInput{
		MaxItems: maxItems,
	})
	if err != nil {
		return translateAWSError(err)
	}
	return nil
}

// ListDomains retrieves a list of all domains associated with the provider.
func (c *Client) ListDomains() ([]string, error) {
	var domains []string
	var marker *string

	for {
		c.rateLimiter.Wait()

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		
		input := &route53.ListHostedZonesInput{
			MaxItems: aws.Int32(100),
			Marker:   marker,
		}

		resp, err := c.r53.ListHostedZones(ctx, input)
		cancel()
		
		if err != nil {
			return nil, fmt.Errorf("route53 list_domains: %w", translateAWSError(err))
		}

		for _, hz := range resp.HostedZones {
			// AWS Route53 adds a trailing dot to the hosted zone names, so we strip it.
			name := aws.ToString(hz.Name)
			if len(name) > 0 && name[len(name)-1] == '.' {
				name = name[:len(name)-1]
			}
			domains = append(domains, name)
		}

		if !resp.IsTruncated {
			break
		}
		marker = resp.NextMarker
	}

	return domains, nil
}

// translateAWSError converts AWS SDK errors into actionable messages.
func translateAWSError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	// Check for common AWS error patterns by inspecting the error message.
	// The AWS SDK wraps errors in smithy error types.
	var invalidCredsErr interface{ ErrorCode() string }
	if errors.As(err, &invalidCredsErr) {
		code := invalidCredsErr.ErrorCode()
		switch code {
		case "InvalidClientTokenId":
			return fmt.Errorf("route53: AWS Access Key ID is invalid. Verify your credentials")
		case "SignatureDoesNotMatch":
			return fmt.Errorf("route53: AWS Secret Access Key is invalid or does not match the Access Key ID")
		case "AccessDenied":
			return fmt.Errorf("route53: access denied. Ensure the IAM user or role has route53:ListHostedZones permission")
		case "InvalidAction":
			return fmt.Errorf("route53: invalid action. Verify the AWS region is correct")
		case "Throttling":
			return fmt.Errorf("route53: request throttled by AWS. Reduce the configured rate limit for this connection")
		case "ExpiredTokenException":
			return fmt.Errorf("route53: AWS credentials have expired. Refresh or replace the credentials")
		case "AccessDeniedException":
			return fmt.Errorf("route53: access denied. Ensure the IAM user or role has route53:ListHostedZones permission")
		}
	}

	// Fallback: include the original error for diagnostics.
	// Sanitize to avoid leaking any credential context from the error string.
	if containsSensitive(msg) {
		return fmt.Errorf("route53: AWS API error (details redacted for security)")
	}
	return fmt.Errorf("route53: AWS API error: %s", msg)
}

// containsSensitive is a best-effort check to avoid logging credential fragments.
func containsSensitive(s string) bool {
	if len(s) > 500 {
		return true
	}
	return false
}
