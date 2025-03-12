# Caddy-Gen Test Examples

This directory contains a complete test environment to verify the functionality of caddy-gen.

## Test Environment

The test environment includes the following components:

1. **caddy-gen** - Monitors Docker containers and generates Caddy configuration
2. **Test websites** - Three simple Nginx websites for testing different configuration scenarios:
   - **web1** - Simple hostname configuration
   - **web2** - Configuration with path and custom headers
   - **web3** - Configuration with multiple hostnames and directives

## Usage

### 1. Start the test environment

There are two ways to run the tests:

#### Normal mode (using Docker)

This mode builds and runs caddy-gen in a Docker container:

```bash
cd examples
docker-compose up -d
```

#### Development mode (for debugging)

This mode runs only the test containers in Docker, but runs caddy-gen directly from your local source code:

```bash
cd examples
./test.sh --dev
```

This is useful for debugging as you can modify the code and see changes immediately without rebuilding the Docker image.

### 2. Check the generated configuration

```bash
cat output/docker-sites.caddy
```

### 3. Test dynamic updates

Try adding, modifying, or removing containers, then observe how caddy-gen automatically updates the configuration:

```bash
# Stop a website
docker-compose stop web1

# Start a website
docker-compose start web1

# Modify labels and recreate
docker-compose up -d --force-recreate web2
```

### 4. Verify the configuration

The generated configuration should include:

- Reverse proxy for web1.local to web1:80
- Path-based reverse proxy for web2.local/api to web2:80 with custom Server header
- Reverse proxy for web3.local and www.web3.local to web3:80 with TLS and HSTS directives

## Cleanup

```bash
docker-compose down
```
