package service

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/gera2ld/caddy-gen/internal/config"
	"github.com/gera2ld/caddy-gen/internal/docker"
	"github.com/gera2ld/caddy-gen/internal/generator"
)

// Service is the main service
type Service struct {
	docker    *docker.Client
	generator *generator.Generator
	config    *config.Config
}

// NewService creates a new Service
func NewService() (*Service, error) {
	// Create config
	cfg := config.NewConfig()

	// Create Docker client
	dockerClient, err := docker.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	// Create generator
	gen := generator.NewGenerator(dockerClient, cfg)

	return &Service{
		docker:    dockerClient,
		generator: gen,
		config:    cfg,
	}, nil
}

// Close closes the service
func (s *Service) Close() error {
	return s.docker.Close()
}

// Run runs the service
func (s *Service) Run() error {
	// Initial config check
	s.CheckConfig()

	// Watch for Docker events
	log.Println("Waiting for Docker events...")
	s.docker.WatchEvents(s.CheckConfig)

	return nil
}

// CheckConfig checks and updates the configuration
func (s *Service) CheckConfig() {
	// Read current config
	currentConfig := s.readCurrentConfig()

	// Generate new config
	newConfig, err := s.generator.GenerateConfig()
	if err != nil {
		log.Printf("Failed to generate config: %v", err)
		return
	}

	// Write new config if changed
	if currentConfig != newConfig {
		s.writeNewConfig(newConfig)
	} else {
		log.Println("No change, skip notifying")
	}
}

// readCurrentConfig reads the current configuration from the file
func (s *Service) readCurrentConfig() string {
	data, err := ioutil.ReadFile(s.config.OutFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to read config file: %v", err)
		}
		return ""
	}
	return string(data)
}

// writeNewConfig writes the new configuration to the file and notifies
func (s *Service) writeNewConfig(newConfig string) {
	err := ioutil.WriteFile(s.config.OutFile, []byte(newConfig), 0644)
	if err != nil {
		log.Printf("Failed to write config: %v", err)
		return
	}
	
	log.Printf("Caddy config written: %s", s.config.OutFile)
	s.notifyConfigChange()
}

// notifyConfigChange notifies that the configuration has changed
func (s *Service) notifyConfigChange() {
	s.docker.Notify()
} 