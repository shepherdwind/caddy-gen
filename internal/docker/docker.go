package docker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/api/types/events"
	"github.com/gera2ld/caddy-gen/internal/config"
)

// Client wraps the Docker client with additional functionality
type Client struct {
	client *client.Client
	config *config.Config
}

// NewClient creates a new Docker client
func NewClient(cfg *config.Config) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}
	
	return &Client{
		client: cli,
		config: cfg,
	}, nil
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.client.Close()
}

// ListContainers lists containers in the specified network
func (c *Client) ListContainers() ([]types.Container, error) {
	ctx := context.Background()
	
	// Create filter for containers in the specified network
	args := c.createNetworkFilter()
	
	// List containers
	return c.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: args,
	})
}

// createNetworkFilter creates a filter for containers in the specified network
func (c *Client) createNetworkFilter() filters.Args {
	args := filters.NewArgs()
	args.Add("network", c.config.Network)
	args.Add("status", "created")
	args.Add("status", "restarting")
	args.Add("status", "running")
	return args
}

// Notify notifies the Caddy container to reload
func (c *Client) Notify() {
	if c.config.Notify == nil {
		return
	}

	log.Printf("Notify: %+v", c.config.Notify)
	
	ctx := context.Background()
	
	// Create and execute the command
	c.executeCommand(ctx)
}

// executeCommand creates and executes a command in the container
func (c *Client) executeCommand(ctx context.Context) {
	// Create exec configuration
	execConfig := c.createExecConfig()

	// Create exec instance
	resp, err := c.client.ContainerExecCreate(ctx, c.config.Notify.ContainerID, execConfig)
	if err != nil {
		log.Printf("Failed to create exec: %v", err)
		return
	}

	// Start exec instance
	err = c.client.ContainerExecStart(ctx, resp.ID, types.ExecStartCheck{})
	if err != nil {
		log.Printf("Failed to start exec: %v", err)
	}
}

// createExecConfig creates an exec configuration for the container
func (c *Client) createExecConfig() types.ExecConfig {
	return types.ExecConfig{
		Cmd:          c.config.Notify.Command,
		WorkingDir:   c.config.Notify.WorkingDir,
		AttachStdout: false,
		AttachStderr: false,
		Detach:       true,
	}
}

// WatchEvents watches for Docker events and calls the callback function
func (c *Client) WatchEvents(callback func()) {
	ctx := context.Background()
	
	// Create filter for container events
	args := c.createEventFilter()
	
	// Create debounced callback function
	debouncedCallback := debounce(callback, 1*time.Second)
	
	// Start watching events
	c.watchEventLoop(ctx, args, debouncedCallback)
}

// createEventFilter creates a filter for container events
func (c *Client) createEventFilter() filters.Args {
	args := filters.NewArgs()
	args.Add("type", "container")
	args.Add("event", "start")
	args.Add("event", "stop")
	return args
}

// watchEventLoop watches for Docker events in a loop
func (c *Client) watchEventLoop(ctx context.Context, args filters.Args, callback func()) {
	for {
		// Create message channel
		messages, errs := c.client.Events(ctx, types.EventsOptions{
			Filters: args,
		})
		
		// Process events
		c.processEvents(messages, errs, callback)
	}
}

// processEvents processes Docker events
func (c *Client) processEvents(messages <-chan events.Message, errs <-chan error, callback func()) {
	for {
		select {
		case <-messages:
			callback()
		case err := <-errs:
			if err != nil {
				log.Printf("Error receiving events: %v", err)
				time.Sleep(5 * time.Second) // Wait before reconnecting
				return
			}
		}
	}
}

// Debounce function to avoid multiple callbacks
func debounce(f func(), delay time.Duration) func() {
	var timer *time.Timer
	return func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, f)
	}
} 