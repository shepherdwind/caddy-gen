package generator

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/gera2ld/caddy-gen/internal/config"
	"github.com/gera2ld/caddy-gen/internal/docker"
)

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

// Generator generates Caddy configuration
type Generator struct {
	docker *docker.Client
	config *config.Config
}

// NewGenerator creates a new Generator
func NewGenerator(dockerClient *docker.Client, cfg *config.Config) *Generator {
	return &Generator{
		docker: dockerClient,
		config: cfg,
	}
}

// GenerateConfig generates Caddy configuration
func (g *Generator) GenerateConfig() (string, error) {
	// List containers
	containers, err := g.docker.ListContainers()
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %v", err)
	}

	// Process containers
	var items []SiteConfig
	for _, container := range containers {
		configs := g.processContainer(container)
		items = append(items, configs...)
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

// processContainer processes a container and returns site configurations
func (g *Generator) processContainer(container types.Container) []SiteConfig {
	var configs []SiteConfig

	rawBind, exists := container.Labels["virtual.bind"]
	if !exists || strings.TrimSpace(rawBind) == "" {
		return configs
	}

	// Process each binding
	for _, bindInfo := range strings.Split(rawBind, ";") {
		bindInfo = strings.TrimSpace(bindInfo)
		if bindInfo == "" {
			continue
		}

		config, err := g.parseBindInfo(bindInfo, container)
		if err != nil {
			log.Printf("Error parsing bind info for container %s: %v", container.Names[0], err)
			continue
		}

		configs = append(configs, config)
	}

	return configs
}

// parseBindInfo parses a bind info string and returns a site configuration
func (g *Generator) parseBindInfo(bindInfo string, container types.Container) (SiteConfig, error) {
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

	if len(bindElements) < 2 {
		return SiteConfig{}, fmt.Errorf("invalid bind format: %s", bind)
	}

	port, err := strconv.Atoi(bindElements[0])
	if err != nil {
		return SiteConfig{}, fmt.Errorf("invalid port in binding %s: %v", bind, err)
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
	if networkSettings, exists := container.NetworkSettings.Networks[g.config.Network]; exists {
		proxyIP = networkSettings.IPAddress
	}

	return SiteConfig{
		Hostnames:       hostnames,
		Port:            port,
		PathMatcher:     path,
		Name:            strings.TrimPrefix(container.Names[0], "/"),
		HostDirectives:  hostDirectives,
		ProxyDirectives: proxyDirectives,
		ProxyIP:         proxyIP,
	}, nil
} 