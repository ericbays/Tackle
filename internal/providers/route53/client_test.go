package route53

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// mockR53 is a mock implementation of r53API for testing.
type mockR53 struct {
	listErr error
}

func (m *mockR53) ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return &route53.ListHostedZonesOutput{}, nil
}

func TestTestConnection_Success(t *testing.T) {
	c := newClientWithAPI(&mockR53{}, 0)
	if err := c.TestConnection(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTestConnection_APIError(t *testing.T) {
	c := newClientWithAPI(&mockR53{listErr: errors.New("some AWS error")}, 0)
	err := c.TestConnection()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "route53") {
		t.Errorf("error should mention route53: %v", err)
	}
}

func TestTranslateAWSError_Nil(t *testing.T) {
	if err := translateAWSError(nil); err != nil {
		t.Errorf("nil input should return nil, got %v", err)
	}
}

func TestTranslateAWSError_GenericError(t *testing.T) {
	err := translateAWSError(errors.New("connection timeout"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "route53") {
		t.Errorf("error should mention route53: %v", err)
	}
}
