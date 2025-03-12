package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Config represents the notification configuration
type NotifyConfig struct {
	ContainerID string   `json:"containerId"`
	WorkingDir  string   `json:"workingDir"`
	Command     []string `json:"command"`
}

// SiteConfig represents a site configuration
type SiteConfig struct {
	Hostnames       []string
	Port            int
	PathMatcher     string
	Name            string
	HostDirectives  []string
	ProxyDirectives []string
	ProxyIP         string
}

var (
	caddyGenNetwork = getEnv("CADDY_GEN_NETWORK", "gateway")
	caddyGenOutfile = getEnv("CADDY_GEN_OUTFILE", "docker-sites.caddy")
	caddyGenNotify  = parseNotifyConfig(getEnv("CADDY_GEN_NOTIFY", ""))
)

func main() {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	// Initial config check
	checkConfig(cli)

	// Listen for Docker events
	log.Println("Waiting for Docker events...")
	bindEvents(cli)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func parseNotifyConfig(raw string) *NotifyConfig {
	if raw == "" {
		return nil
	}

	var config NotifyConfig
	err := json.Unmarshal([]byte(raw), &config)
	if err != nil {
		log.Printf("Failed to parse CADDY_GEN_NOTIFY: %v", err)
		return nil
	}
	return &config
}

func generateConfig(cli *client.Client) (string, error) {
	ctx := context.Background()
	
	// List containers in the specified network
	args := filters.NewArgs()
	args.Add("network", caddyGenNetwork)
	args.Add("status", "created")
	args.Add("status", "restarting")
	args.Add("status", "running")
	
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: args,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %v", err)
	}

	var items []SiteConfig
	for _, container := range containers {
		rawBind, exists := container.Labels["virtual.bind"]
		if !exists || strings.TrimSpace(rawBind) == "" {
			continue
		}

		// Process each binding
		for _, bindInfo := range strings.Split(rawBind, ";") {
			bindInfo = strings.TrimSpace(bindInfo)
			if bindInfo == "" {
				continue
			}

			bindParts := strings.Split(bindInfo, "|")
			bind := strings.TrimSpace(bindParts[0])
			directives := bindParts[1:]

			// Process bind part
			bindElements := strings.Fields(bind)
			var path string
			if strings.HasPrefix(bind, "/") {
				path = bindElements[0]
				bindElements = bindElements[1:]
			}

			port, err := strconv.Atoi(bindElements[0])
			if err != nil {
				log.Printf("Invalid port in binding %s: %v", bind, err)
				continue
			}
			hostnames := bindElements[1:]

			// Process directives
			var hostDirectives, proxyDirectives []string
			for _, directive := range directives {
				directive = strings.TrimSpace(directive)
				if strings.HasPrefix(directive, "host:") {
					hostDirectives = append(hostDirectives, strings.TrimSpace(directive[5:]))
				} else {
					proxyDirectives = append(proxyDirectives, directive)
				}
			}

			// Get container IP in the network
			var proxyIP string
			if networkSettings, exists := container.NetworkSettings.Networks[caddyGenNetwork]; exists {
				proxyIP = networkSettings.IPAddress
			}

			items = append(items, SiteConfig{
				Hostnames:       hostnames,
				Port:            port,
				PathMatcher:     path,
				Name:            strings.TrimPrefix(container.Names[0], "/"),
				HostDirectives:  hostDirectives,
				ProxyDirectives: proxyDirectives,
				ProxyIP:         proxyIP,
			})
		}
	}

	// Group by hostnames
	groups := make(map[string][]SiteConfig)
	for _, item := range items {
		key := strings.Join(item.Hostnames, " ")
		groups[key] = append(groups[key], item)
	}

	// Generate config
	var configParts []string
	i := 0
	for hostnames, group := range groups {
		hostMatcher := fmt.Sprintf("@caddy-gen-%d", i)
		i++

		var sectionLines []string
		sectionLines = append(sectionLines, fmt.Sprintf("%s host %s", hostMatcher, hostnames))
		sectionLines = append(sectionLines, fmt.Sprintf("handle %s {", hostMatcher))

		// Add host directives
		for _, item := range group {
			for _, directive := range item.HostDirectives {
				sectionLines = append(sectionLines, fmt.Sprintf("  %s", directive))
			}
		}

		// Add proxy directives
		for _, item := range group {
			sectionLines = append(sectionLines, fmt.Sprintf("  # %s", item.Name))
			sectionLines = append(sectionLines, fmt.Sprintf("  reverse_proxy %s {", item.PathMatcher))
			
			for _, directive := range item.ProxyDirectives {
				sectionLines = append(sectionLines, fmt.Sprintf("    %s", directive))
			}
			
			sectionLines = append(sectionLines, fmt.Sprintf("    to %s:%d", item.ProxyIP, item.Port))
			sectionLines = append(sectionLines, "  }")
		}

		sectionLines = append(sectionLines, "}")
		configParts = append(configParts, strings.Join(sectionLines, "\n"))
	}

	return strings.Join(configParts, "\n\n"), nil
}

func notify(cli *client.Client) {
	if caddyGenNotify == nil {
		return
	}

	log.Printf("Notify: %+v", caddyGenNotify)
	
	ctx := context.Background()
	
	// Create exec configuration
	execConfig := types.ExecConfig{
		Cmd:          caddyGenNotify.Command,
		WorkingDir:   caddyGenNotify.WorkingDir,
		AttachStdout: false,
		AttachStderr: false,
		Detach:       true,
	}

	// Create exec instance
	resp, err := cli.ContainerExecCreate(ctx, caddyGenNotify.ContainerID, execConfig)
	if err != nil {
		log.Printf("Failed to create exec: %v", err)
		return
	}

	// Start exec instance
	err = cli.ContainerExecStart(ctx, resp.ID, types.ExecStartCheck{})
	if err != nil {
		log.Printf("Failed to start exec: %v", err)
	}
}

func checkConfig(cli *client.Client) {
	// Read current config
	var currentConfig string
	data, err := ioutil.ReadFile(caddyGenOutfile)
	if err == nil {
		currentConfig = string(data)
	}

	// Generate new config
	newConfig, err := generateConfig(cli)
	if err != nil {
		log.Printf("Failed to generate config: %v", err)
		return
	}

	// Write new config if changed
	if currentConfig != newConfig {
		err = ioutil.WriteFile(caddyGenOutfile, []byte(newConfig), 0644)
		if err != nil {
			log.Printf("Failed to write config: %v", err)
			return
		}
		log.Printf("Caddy config written: %s", caddyGenOutfile)
		notify(cli)
	} else {
		log.Println("No change, skip notifying")
	}
}

// Debounce function to avoid multiple config checks
func debounce(f func(*client.Client), delay time.Duration) func(*client.Client) {
	var timer *time.Timer
	return func(cli *client.Client) {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, func() {
			f(cli)
		})
	}
}

func bindEvents(cli *client.Client) {
	ctx := context.Background()
	
	// Create filter for container events
	args := filters.NewArgs()
	args.Add("type", "container")
	args.Add("event", "start")
	args.Add("event", "stop")
	
	// Create message channel
	messages, errs := cli.Events(ctx, types.EventsOptions{
		Filters: args,
	})

	// Create debounced check function
	debouncedCheckConfig := debounce(checkConfig, 1*time.Second)

	// Process events
	for {
		select {
		case <-messages:
			debouncedCheckConfig(cli)
		case err := <-errs:
			if err != nil {
				log.Printf("Error receiving events: %v", err)
				time.Sleep(5 * time.Second) // Wait before reconnecting
				
				// Recreate event stream
				messages, errs = cli.Events(ctx, types.EventsOptions{
					Filters: args,
				})
			}
		}
	}
} 