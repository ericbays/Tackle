package endpoint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// RealSSHClientFactory creates SSH connections using the golang.org/x/crypto/ssh package.
type RealSSHClientFactory struct{}

// Dial establishes an SSH connection to the given address with retry logic.
// Retries for up to 2 minutes to allow time for the VM to finish booting.
func (f *RealSSHClientFactory) Dial(ctx context.Context, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	var client *ssh.Client
	var err error

	maxRetries := 24
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		client, err = ssh.Dial("tcp", addr, config)
		if err == nil {
			return &realSSHClient{client: client}, nil
		}

		if !isRetryableSSHError(err) {
			return nil, fmt.Errorf("ssh dial: %w", err)
		}

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("ssh dial: gave up after %d attempts: %w", maxRetries, err)
}

// isRetryableSSHError returns true for errors that indicate the VM is not yet ready.
func isRetryableSSHError(err error) bool {
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return false
}

// realSSHClient wraps an ssh.Client and implements the SSHClient interface.
type realSSHClient struct {
	client *ssh.Client
}

// Upload transfers a file to the remote host.
func (c *realSSHClient) Upload(ctx context.Context, remotePath string, data []byte, mode uint32) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh upload: new session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("ssh upload: stdin pipe: %w", err)
	}

	cmd := fmt.Sprintf("cat > %s && chmod %o %s", remotePath, mode, remotePath)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("ssh upload: start: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		_, writeErr := io.Copy(stdin, bytes.NewReader(data))
		stdin.Close()
		if writeErr != nil {
			done <- writeErr
			return
		}
		done <- session.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run executes a command on the remote host and returns combined output.
func (c *realSSHClient) Run(ctx context.Context, cmd string) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh run: new session: %w", err)
	}
	defer session.Close()

	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)

	go func() {
		output, runErr := session.CombinedOutput(cmd)
		done <- result{output, runErr}
	}()

	select {
	case r := <-done:
		return r.output, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close closes the SSH connection.
func (c *realSSHClient) Close() error {
	return c.client.Close()
}
