package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// Client wraps the Docker SDK client for query operations.
type Client struct {
	api *client.Client
}

// NewClient creates a new Docker SDK client.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
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
	_, err := c.api.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker is not reachable: %w", err)
	}
	return nil
}

// SwarmStatus returns the current node's swarm state.
func (c *Client) SwarmStatus(ctx context.Context) (swarm.LocalNodeState, error) {
	info, err := c.api.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("get Docker info: %w", err)
	}
	return info.Swarm.LocalNodeState, nil
}

// ListStackServices lists services belonging to a stack.
func (c *Client) ListStackServices(ctx context.Context, stackName string) ([]swarm.Service, error) {
	services, err := c.api.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	var stackServices []swarm.Service
	for i := range services {
		if services[i].Spec.Labels["com.docker.stack.namespace"] == stackName {
			stackServices = append(stackServices, services[i])
		}
	}
	return stackServices, nil
}

// IsStackRunning checks if any services for the stack are deployed.
func (c *Client) IsStackRunning(ctx context.Context, stackName string) (bool, error) {
	services, err := c.ListStackServices(ctx, stackName)
	if err != nil {
		return false, err
	}
	return len(services) > 0, nil
}
