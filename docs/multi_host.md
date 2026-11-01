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

## Configuring instances with environment variables

Each instance can also be configured with indexed environment variables of the form `TRAEFIK_INSTANCES_<n>_<OPTION>`, where `<n>` is the **1-based** instance number. The options mirror the instance options table above:

| Environment Variable | Description |
|---------------------|-------------|
| `TRAEFIK_INSTANCES_<n>_API_HOST` | Full base URL of the Traefik API for instance `<n>` (e.g. `http://traefik:8080`). |
| `TRAEFIK_INSTANCES_<n>_NAME` | Display name for instance `<n>`. |
| `TRAEFIK_INSTANCES_<n>_ENABLE_BASIC_AUTH` | Enable basic auth for instance `<n>`. |
| `TRAEFIK_INSTANCES_<n>_INSECURE_SKIP_VERIFY` | Skip TLS certificate verification for instance `<n>`. |
| `TRAEFIK_INSTANCES_<n>_BASIC_AUTH_USERNAME` | Basic auth username for instance `<n>`. |
| `TRAEFIK_INSTANCES_<n>_BASIC_AUTH_PASSWORD` | Basic auth password for instance `<n>`. |
| `TRAEFIK_INSTANCES_<n>_BASIC_AUTH_PASSWORD_FILE` | Path to a password file for instance `<n>`. |

### How YAML and environment variables are merged

1. The `instances` list from the configuration file (in any of the supported formats) forms the base.
2. Each indexed variable is applied to the instance at its position: `TRAEFIK_INSTANCES_2_NAME` sets the name of the second instance, `TRAEFIK_INSTANCES_3_API_HOST` sets the API host of the third, and so on.
3. Fields are merged individually: setting `TRAEFIK_INSTANCES_2_NAME` on a file-configured second instance changes only its name and keeps its `api_host` and all other file values.
4. When `n` is larger than the number of configured instances, new instances are appended. Options that are not provided default to empty — `api_host` is required, so an appended instance without it fails the load.
5. The presence of any indexed variable switches TraLa to multi-instance mode, even when only one instance ends up configured.
6. Invalid values (wrong type or out of range) log a warning and keep the previous value. Unknown option suffixes and unusable indexes (such as `TRAEFIK_INSTANCES_0_...`) are ignored with a warning; the load continues.
7. After merging, display names that are still unset are derived from each instance's API hostname, and duplicate names get a numeric suffix.

The legacy `TRAEFIK_*` variables from [Configuration](/docs/configuration#traefik-api-variables) apply **only to the first instance in single-instance mode**. When TraLa detects a multi-instance configuration — from the file or from indexed variables — it logs a warning and ignores those variables.

Example: two instances configured purely with environment variables:

```yaml
# docker-compose.yml
services:
  trala:
    image: trala
    environment:
      - TRAEFIK_INSTANCES_1_NAME=public
      - TRAEFIK_INSTANCES_1_API_HOST=http://traefik:8080
      - TRAEFIK_INSTANCES_2_NAME=arr
      - TRAEFIK_INSTANCES_2_API_HOST=http://traefik-arr:8080
```

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
