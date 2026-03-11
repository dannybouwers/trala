# Setup Guide

This guide covers everything you need to get TraLa up and running.

## Docker Images

TraLa is available on both GitHub Container Registry (primary) and Docker Hub (secondary):

| Registry | Image Name | Example |
|----------|------------|---------|
| GitHub Container Registry (Primary) | `ghcr.io/dannybouwers/trala` | `ghcr.io/dannybouwers/trala:latest` |
| Docker Hub (Secondary) | `dannybouwers/trala` | `dannybouwers/trala:latest` |

### Version Tags

| Tag | Description | Recommended |
|-----|-------------|-------------|
| `latest` | Latest stable release | ✅ Yes |
| `major.minor.patch` | Specific version (e.g., `3.3.0`) | For reproducibility |
| `major.minor` | Latest patch for a minor version | For flexibility |

---

## Quick Start

Add TraLa to your existing `docker-compose.yml`:

```yaml
services:
  traefik:
    image: "traefik:v3.0"
    # ... your existing traefik configuration ...
    command:
      # ... your existing commands ...
      - "--api.insecure=true"  # Required for TraLa

  trala:
    image: ghcr.io/dannybouwers/trala:latest
    container_name: trala
    restart: unless-stopped
    environment:
      - TRAEFIK_API_HOST=http://traefik:8080
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.trala.rule=Host(`trala.your-domain.com`)"
      - "traefik.http.services.trala.loadbalancer.server.port=8080"
```

That's it! TraLa will automatically discover and display all your Traefik services.

---

## Full-Featured Example

Here's a complete example with all available configuration options:

```yaml
# docker-compose.yml
services:
  traefik:
    image: "traefik:v3.0"
    container_name: traefik
    restart: unless-stopped
    networks:
      - traefik-net
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"  # API port (only required for troubleshooting)
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    command:
      - "--api.insecure=true"
      - "--api.dashboard=true"
      # Your other Traefik configuration...

  trala:
    # Primary: GitHub Container Registry
    image: ghcr.io/dannybouwers/trala:latest
    # Secondary: Docker Hub
    # image: dannybouwers/trala:latest
    container_name: trala
    restart: unless-stopped
    networks:
      - traefik-net
    volumes:
      # Configuration file (optional but recommended)
      - ./configuration.yml:/config/configuration.yml:ro
      # Custom icons directory (optional)
      - ./icons:/icons:ro
    environment:
      # Required: Traefik API endpoint
      - TRAEFIK_API_HOST=http://traefik:8080
      
      # Optional: Refresh interval (default: 30 seconds)
      - REFRESH_INTERVAL_SECONDS=30
      
      # Optional: External search engine
      - SEARCH_ENGINE_URL=https://duckduckgo.com/?q=
      
      # Optional: Log level (default: info)
      - LOG_LEVEL=info
      
      # Optional: Language (en, de, nl)
      - LANGUAGE=en
    labels:
      # Traefik labels to expose TraLa
      - "traefik.enable=true"
      - "traefik.http.routers.trala.rule=Host(`trala.your-domain.com`)"
      - "traefik.http.routers.trala.entrypoints=websecure"
      - "traefik.http.routers.trala.tls=true"
      - "traefik.http.services.trala.loadbalancer.server.port=8080"
      - "traefik.http.services.trala.loadbalancer.server.scheme=http"

networks:
  traefik-net:
    driver: bridge
```

---

## Configuration File

A configuration file provides more customization options:

```yaml
# configuration.yml
version: 3.3

environment:
  selfhst_icon_url: https://cdn.jsdelivr.net/gh/selfhst/icons/
  search_engine_url: https://duckduckgo.com/?q=
  refresh_interval_seconds: 30
  log_level: info
  language: en
  grouping:
    enabled: true
    columns: 3
    tag_frequency_threshold: 0.9
    min_services_per_group: 2
  traefik:
    api_host: http://traefik:8080
    enable_basic_auth: false
    insecure_skip_verify: false

services:
  exclude:
    routers:
      - "traefik-api"
    entrypoints:
      - "*lan*"
  
  overrides:
    - service: "home-assistant"
      display_name: "Home Assistant"
      icon: "home-assistant.svg"
    - service: "unifi-controller"
      display_name: "UniFi Network"
      icon: "ubiquiti-unifi.svg"
      group: "Network"
  
  manual:
    - name: "GitHub"
      url: "https://github.com"
      icon: "github.svg"
      priority: 100
```

---

## What's Next?

Now that you have TraLa running, explore these features to customize your dashboard:

| Feature | Description | Documentation |
|---------|-------------|---------------|
| **Configuration** | Full configuration options with YAML and environment variables | [Configuration](/docs/configuration) |
| **Service Management** | Exclude, override, or manually add services | [Services](/docs/services) |
| **Smart Grouping** | Auto-group services by tags | [Grouping](/docs/grouping) |
| **Custom Icons** | Use selfh.st icons or your own | [Icons](/docs/icons) |
| **External Search** | Configure search engine | [Search](/docs/search) |
| **Security** | Secure Traefik API access | [Security](/docs/security) |

---

## Troubleshooting

### Services Not Appearing

1. Verify Traefik API is accessible (from within the Trala container):
   ```bash
   docker exec trala curl http://traefik:8080/api/http/routers
   ```
   
   Or from your Docker host (if port 8080 is exposed):
   ```bash
   curl http://localhost:8080/api/http/routers
   ```

2. Check TraLa logs:
   ```bash
   docker logs trala
   ```

3. Enable debug logging:
   ```yaml
   environment:
     - LOG_LEVEL=debug
   ```

### Icons Not Loading

1. Check internet connectivity to `cdn.jsdelivr.net`
2. Try custom icons in `/icons` directory
3. See [/docs/icons](/docs/icons) for debugging tips

### Network Issues

Ensure TraLa and Traefik are on the same Docker network:

```yaml
networks:
  traefik-net:
    driver: bridge

services:
  trala:
    networks:
      - traefik-net
```
