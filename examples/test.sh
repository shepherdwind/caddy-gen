#!/bin/bash

# Color definitions
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default mode
DEV_MODE=false

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --dev) DEV_MODE=true; shift ;;
    *) echo "Unknown parameter: $1"; exit 1 ;;
  esac
done

echo -e "${YELLOW}Caddy-Gen Functionality Test${NC}"
echo "=============================="
if [ "$DEV_MODE" = true ]; then
  echo -e "${YELLOW}Running in development mode${NC}"
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
  echo -e "${RED}Error: Docker is not running. Please start Docker first.${NC}"
  exit 1
fi

# Create output directory
mkdir -p output

# Start test environment
echo -e "\n${YELLOW}1. Starting test environment${NC}"

if [ "$DEV_MODE" = true ]; then
  # In dev mode, only start the test containers
  docker-compose down > /dev/null 2>&1
  docker-compose up -d web1 web2 web3
  if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Unable to start test containers.${NC}"
    exit 1
  fi
  echo -e "${GREEN}Test containers started.${NC}"
  
  # Run caddy-gen locally
  echo -e "\n${YELLOW}Running caddy-gen locally...${NC}"
  echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
  cd ..
  CADDY_GEN_NETWORK=gateway CADDY_GEN_OUTFILE=examples/output/docker-sites.caddy go run main.go
else
  # In normal mode, start all services including caddy-gen
  docker-compose down > /dev/null 2>&1
  docker-compose up -d
  if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Unable to start test environment.${NC}"
    exit 1
  fi
  echo -e "${GREEN}Test environment started.${NC}"
  
  # Wait for services to start
  echo -e "\n${YELLOW}2. Waiting for services to start${NC}"
  echo "Waiting 10 seconds..."
  sleep 10
  
  # Check if caddy-gen generated the config file
  echo -e "\n${YELLOW}3. Checking configuration file${NC}"
  if [ -f "output/docker-sites.caddy" ]; then
    echo -e "${GREEN}Configuration file generated: output/docker-sites.caddy${NC}"
    echo "Configuration file contents:"
    echo "-----------------------------"
    cat output/docker-sites.caddy
    echo "-----------------------------"
  else
    echo -e "${RED}Error: Configuration file not generated.${NC}"
    docker-compose logs caddy-gen
    exit 1
  fi
  
  # Test dynamic updates
  echo -e "\n${YELLOW}4. Testing dynamic updates${NC}"
  echo "Stopping web1 container..."
  docker-compose stop web1
  echo "Waiting 5 seconds..."
  sleep 5
  
  echo "Checking if configuration file was updated..."
  grep -q "web1.local" output/docker-sites.caddy
  if [ $? -eq 0 ]; then
    echo -e "${RED}Error: web1.local still exists in the configuration file.${NC}"
  else
    echo -e "${GREEN}Configuration file correctly updated, web1.local removed.${NC}"
  fi
  
  echo "Restarting web1 container..."
  docker-compose start web1
  echo "Waiting 5 seconds..."
  sleep 5
  
  echo "Checking if configuration file was updated..."
  grep -q "web1.local" output/docker-sites.caddy
  if [ $? -eq 0 ]; then
    echo -e "${GREEN}Configuration file correctly updated, web1.local added back.${NC}"
  else
    echo -e "${RED}Error: web1.local not added back to the configuration file.${NC}"
  fi
  
  # Verify configuration content
  echo -e "\n${YELLOW}5. Verifying configuration content${NC}"
  echo "Checking for web1.local configuration..."
  grep -q "web1.local" output/docker-sites.caddy && echo -e "${GREEN}web1.local configuration found.${NC}" || echo -e "${RED}web1.local configuration not found.${NC}"
  
  echo "Checking for web2.local path configuration..."
  grep -q "web2.local" output/docker-sites.caddy && grep -q "reverse_proxy /api" output/docker-sites.caddy && echo -e "${GREEN}web2.local path configuration found.${NC}" || echo -e "${RED}web2.local path configuration not found.${NC}"
  
  echo "Checking for custom header in web2 configuration..."
  grep -q "header Server" output/docker-sites.caddy && echo -e "${GREEN}Custom header configuration found.${NC}" || echo -e "${RED}Custom header configuration not found.${NC}"
  
  echo "Checking for multiple hostnames in web3 configuration..."
  grep -q "web3.local www.web3.local" output/docker-sites.caddy && echo -e "${GREEN}Multiple hostnames configuration found.${NC}" || echo -e "${RED}Multiple hostnames configuration not found.${NC}"
  
  echo "Checking for host directive in web3 configuration..."
  grep -q "tls internal" output/docker-sites.caddy && echo -e "${GREEN}Host directive configuration found.${NC}" || echo -e "${RED}Host directive configuration not found.${NC}"
  
  # Test complete
  echo -e "\n${YELLOW}6. Test complete${NC}"
  echo -e "${GREEN}All tests completed.${NC}"
  
  echo -e "\n${YELLOW}Tip:${NC} Use 'docker-compose down' to clean up the test environment."
fi 