# Multi-Host Support

TraLa can aggregate services from **multiple Traefik instances** into a single dashboard. This is useful when you run more than one Traefik proxy — for example one for public services, one for an "arr" stack, or separate proxies per network segment.

When multiple instances are configured, TraLa fetches routers from each Traefik API, labels every discovered service with the host it came from, and presents them in a **per-host view**. You can optionally blend all services into one combined grid.

## Configuring multiple Traefik instances

Add a `traefik` block that contains a list of `instances`. Each instance supports the same options as the legacy single-instance configuration.

```yaml
# configuration.yml
version: 4.0

environment:
  traefik:
    instances:
      - api_host: http://traefik:8080
        insecure_skip_verify: false
      - name: arr
        api_host: http://traefik-arr:8080
        enable_basic_auth: true
        basic_auth:
          username: proxy
          password_file: /run/secrets/basic_auth_password
```

### Instance options

| Option | Required | Description |
|--------|----------|-------------|
| `name` | No | Display name for the host. Defaults to the API host's hostname (e.g. `traefik`). Duplicate names get a numeric suffix. |
| `api_host` | Yes | Full base URL of the Traefik API (e.g. `http://traefik:8080`). |
| `enable_basic_auth` | No | Enable HTTP basic auth when talking to this instance's API. |
| `basic_auth.username` | No | Basic auth username (required when auth is enabled). |
| `basic_auth.password` | No | Basic auth password (plain text). Mutually exclusive with `password_file`. |
| `basic_auth.password_file` | No | Path to a file containing the basic auth password. |
| `insecure_skip_verify` | No | Skip TLS certificate verification for this instance's API. Default `false`. |

> [!NOTE]
> The list can contain a single entry. A `traefik` block with an `instances` list with one item is treated as **single-host mode**, which hides the host view and controls in the UI.

## Supported configuration formats

TraLa accepts three equivalent formats for the `traefik` block. All of them are normalized internally to the instance list above.

### 1. Explicit `instances` list (recommended)

```yaml
traefik:
  instances:
    - api_host: http://traefik:8080
    - api_host: http://traefik-arr:8080
```

### 2. Bare list (no `instances` key)

```yaml
traefik:
  - api_host: http://traefik:8080
  - api_host: http://traefik-arr:8080
```

### 3. Legacy single-instance format

```yaml
traefik:
  api_host: http://traefik:8080
  insecure_skip_verify: false
```

The legacy format is still fully supported. It is automatically converted into a single-instance list, so existing configuration files keep working without changes.

> [!NOTE]
> Environment variables (`TRAEFIK_API_HOST`, `TRAEFIK_BASIC_AUTH_*`, `TRAEFIK_INSECURE_SKIP_VERIFY`) apply **only to single-instance mode**. When TraLa detects a multi-instance configuration it logs a warning and ignores those variables — configure each instance in the configuration file instead.

## The dashboard in multi-host mode

When more than one instance is configured, the dashboard shows a **per-host view**:

- Services are grouped under a collapsible header for each host (using the instance `name`).
- Each host header can be clicked to expand or collapse its services.
- The **Expand/Collapse All** control toggles every host section at once.

### Mixing hosts

The **Mix Hosts** button (available only in multi-host mode) combines all services from every instance into a single flat grid — while still keeping smart grouping available. Toggle it again to return to the per-host layout.

### Manual services across hosts

Manual services are assigned to a host too. If you don't specify one, a manual service is attached to the first configured instance. Use the `host` option to pin it to a specific instance by name:

```yaml
services:
  manual:
    - name: "GitHub"
      url: "https://github.com"
      host: arr
```

See [Manual Services](/docs/manual_services) for all available options.

## Example: public and arr proxies

```yaml
environment:
  traefik:
    instances:
      - name: public
        api_host: http://traefik:8080
      - name: arr
        api_host: http://traefik-arr:8080
        enable_basic_auth: true
        basic_auth:
          username: proxy
          password_file: /run/secrets/basic_auth_password

services:
  exclude:
    routers:
      - "traefik-api"
```

In this setup the dashboard shows two host sections — **public** and **arr** — each containing only the routers discovered from its own Traefik API.
