# Description

A simple, modern, and dynamic dashboard for your Traefik services. This application automatically discovers services via the Traefik API and displays them in a clean, responsive grid. It's designed to be run as a lightweight, multi-arch Docker container.

## Features

### Automation first
- **Auto-Discovery:** Automatically fetches and displays all HTTP routers from your Traefik instance.
- **Icon Auto-Detection:** Intelligently finds the best icon for each service using selfh.st/icons as the main source.
- **Smart Grouping:** Automatically group services based on tags from selfh.st/apps.
- **Light/Dark Mode:** Automatic Light/Dark mode based on your OS settings.

### Configuration Overrides
Everything automatic can be overwritten with a single YAML configuration file, providing ultimate customization control.

### Additional Features
- **Manual Services:** Add custom services to your dashboard that aren't managed by Traefik.
- **Service Exclusion:** Hide specific services from the dashboard using router and entry point name exclusions.
- **Live Search & Sort:** Instantly filter and sort your services by name, URL, or priority.
- **External Search:** Use the search bar to quickly search the web with your configured search engine.
- **Lightweight & Multi-Arch:** Built with Go and a minimal Alpine base, the Docker image is small and compatible with `amd64` and `arm64` architectures.
- **Multi-Language Support:** Available in English, German, and Dutch.

## Quick start

Get TraLa up and running in minutes. Add TraLa to your existing `docker-compose.yml`:

```yaml
services:
  traefik:
    image: "traefik"
    # ... your existing traefik configuration ...
    command:
      # ...
      - "--api.insecure=true" # Required for the dashboard to access the API

  trala:
    image: ghcr.io/dannybouwers/trala:latest
    environment:
      - TRAEFIK_API_HOST=http://traefik:8080
    labels:
      # Traefik Labels to expose TraLa itself
      - "traefik.enable=true"
      - "traefik.http.routers.trala.rule=Host(`trala.your-domain.com`)"
      - "traefik.http.services.trala.loadbalancer.server.port=8080"
      - "traefik.http.services.trala.loadbalancer.server.scheme=http"
```

## Next Steps

- **[Configuration](configuration.md)** — Learn how to customize TraLa with configuration files and environment variables
- **[Services](services.md)** — Exclude services, add overrides, or add manual services
- **[Grouping](grouping.md)** — Enable smart grouping to organize your services
- **[Icons](icons.md)** — Configure icon detection and custom icons
- **[Search](search.md)** — Set up external search engine
- **[Security](security.md)** — Secure Traefik API access with authentication
