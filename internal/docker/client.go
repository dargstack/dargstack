package docker

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
)

// Client wraps the Docker SDK client for query operations.
type Client struct {
	api *client.Client
}

// NewClient creates a new Docker SDK client.
func NewClient() (*Client, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}
	return &Client{api: cli}, nil
}

// Close releases the Docker client resources.
func (c *Client) Close() error {
	return c.api.Close()
}

// Ping checks if Docker is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.api.Ping(ctx, client.PingOptions{})
	if err != nil {
		return fmt.Errorf("docker is not reachable: %w", err)
	}
	return nil
}

// SwarmStatus returns the current node's swarm state.
func (c *Client) SwarmStatus(ctx context.Context) (swarm.LocalNodeState, error) {
	result, err := c.api.Info(ctx, client.InfoOptions{})
	if err != nil {
		return "", fmt.Errorf("get Docker info: %w", err)
	}
	return result.Info.Swarm.LocalNodeState, nil
}

// ListStackServices lists services belonging to a stack, using server-side
// label filtering to avoid fetching all services on large swarms.
func (c *Client) ListStackServices(ctx context.Context, stackName string) ([]swarm.Service, error) {
	f := make(client.Filters).Add("label", "com.docker.stack.namespace="+stackName)
	result, err := c.api.ServiceList(ctx, client.ServiceListOptions{Filters: f})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	return result.Items, nil
}

// IsStackRunning checks if any services for the stack are deployed.
func (c *Client) IsStackRunning(ctx context.Context, stackName string) (bool, error) {
	services, err := c.ListStackServices(ctx, stackName)
	if err != nil {
		return false, err
	}
	return len(services) > 0, nil
}
