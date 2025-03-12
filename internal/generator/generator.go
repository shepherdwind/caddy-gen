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
	siteConfigs := g.processSiteConfigs(containers)
	
	// Group by hostnames
	groups := g.groupSiteConfigs(siteConfigs)
	
	// Generate config
	return g.generateCaddyConfig(groups), nil
}

// processSiteConfigs processes containers and returns site configurations
func (g *Generator) processSiteConfigs(containers []types.Container) []SiteConfig {
	var siteConfigs []SiteConfig
	for _, container := range containers {
		configs := g.processContainer(container)
		siteConfigs = append(siteConfigs, configs...)
	}
	return siteConfigs
}

// groupSiteConfigs groups site configurations by hostnames
func (g *Generator) groupSiteConfigs(siteConfigs []SiteConfig) map[string][]SiteConfig {
	groups := make(map[string][]SiteConfig)
	for _, item := range siteConfigs {
		key := strings.Join(item.Hostnames, " ")
		groups[key] = append(groups[key], item)
	}
	return groups
}

// generateCaddyConfig generates Caddy configuration from grouped site configurations
func (g *Generator) generateCaddyConfig(groups map[string][]SiteConfig) string {
	var configParts []string
	i := 0
	
	for hostnames, group := range groups {
		configParts = append(configParts, g.generateHostConfig(hostnames, group, i))
		i++
	}
	
	return strings.Join(configParts, "\n\n")
}

// generateHostConfig generates configuration for a host group
func (g *Generator) generateHostConfig(hostnames string, group []SiteConfig, index int) string {
	hostMatcher := fmt.Sprintf("@caddy-gen-%d", index)
	
	var sectionLines []string
	sectionLines = append(sectionLines, fmt.Sprintf("%s host %s", hostMatcher, hostnames))
	sectionLines = append(sectionLines, fmt.Sprintf("handle %s {", hostMatcher))
	
	// Add host directives
	sectionLines = append(sectionLines, g.generateHostDirectives(group)...)
	
	// Add proxy directives
	sectionLines = append(sectionLines, g.generateProxyDirectives(group)...)
	
	sectionLines = append(sectionLines, "}")
	return strings.Join(sectionLines, "\n")
}

// generateHostDirectives generates host directives for a group
func (g *Generator) generateHostDirectives(group []SiteConfig) []string {
	var lines []string
	for _, item := range group {
		for _, directive := range item.HostDirectives {
			lines = append(lines, fmt.Sprintf("  %s", directive))
		}
	}
	return lines
}

// generateProxyDirectives generates proxy directives for a group
func (g *Generator) generateProxyDirectives(group []SiteConfig) []string {
	var lines []string
	for _, item := range group {
		lines = append(lines, fmt.Sprintf("  # %s", item.Name))
		lines = append(lines, fmt.Sprintf("  reverse_proxy %s {", item.PathMatcher))
		
		for _, directive := range item.ProxyDirectives {
			lines = append(lines, fmt.Sprintf("    %s", directive))
		}
		
		lines = append(lines, fmt.Sprintf("    to %s:%d", item.ProxyIP, item.Port))
		lines = append(lines, "  }")
	}
	return lines
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
	hostDirectives, proxyDirectives := g.processDirectives(directives)

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

// processDirectives processes directives and separates them into host and proxy directives
func (g *Generator) processDirectives(directives []string) ([]string, []string) {
	var hostDirectives, proxyDirectives []string
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "host:") {
			hostDirectives = append(hostDirectives, strings.TrimSpace(directive[5:]))
		} else {
			proxyDirectives = append(proxyDirectives, directive)
		}
	}
	return hostDirectives, proxyDirectives
} 