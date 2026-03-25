# Services

TraLa provides several ways to customize how services are displayed, including exclusion, overrides, and manual services.

## Service Exclusion

Hide specific services from appearing in the dashboard by specifying router names or entrypoints in your configuration.

### Excluding Routers

```yaml
services:
  exclude:
    routers:
      - "traefik-api"     # Hide the Traefik API itself
      - "admin-panel"     # Hide internal admin interface
      - "api*"            # Hide all routers starting with "api"
```

Wildcard patterns are supported:
- `*` matches any number of characters
- `?` matches a single character

### Excluding Entrypoints

Hide services based on their entrypoint:

```yaml
services:
  exclude:
    entrypoints:
      - "*lan*"           # Hide services using entrypoints containing "lan"
      - "internal"        # Hide services using the "internal" entrypoint
```

## Service Overrides

Customize display names and icons for your services.

### Override Display Name and Icon

```yaml
services:
  overrides:
    - service: "firefly-core"
      display_name: "Firefly III"
      icon: "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/firefly-iii.svg"
    
    - service: "home-assistant"
      display_name: "Home Assistant"
      icon: "home-assistant.svg"
```

### Override Icon Only

```yaml
services:
  overrides:
    - service: "plex"
      icon: "plex.webp"
    
    - service: "unknown-service"
      icon: "https://selfh.st/content/images/2023/09/favicon-1.png"
```

### Override with Group Assignment

```yaml
services:
  overrides:
    - service: "unifi-controller"
      display_name: "UniFi Network"
      icon: "ubiquiti-unifi.svg"
      group: "Network"
```

This assigns the service to the "Network" group regardless of automatic tag-based grouping.

### Icon File Extensions

When using filenames from the selfh.st icon repository, specify the extension:

- `.png` (default if no extension specified)
- `.svg`
- `.webp`

The application automatically constructs the appropriate URL based on the file extension.

## Manual Services

Add custom services that aren't managed by Traefik. For complete documentation, see [Manual Services](/docs/manual_services).
