# Configuration

TraLa can be configured using a YAML configuration file and environment variables. Environment variables take precedence over settings in the configuration file.

## Configuration File

A sample configuration file:

```yaml
# TraLa Configuration File
version: 3.3

# Environment settings
environment:
  # Icon settings
  selfhst_icon_url: https://cdn.jsdelivr.net/gh/selfhst/icons/

  # Search engine URL
  search_engine_url: https://duckduckgo.com/?q=

  # Refresh interval in seconds
  refresh_interval_seconds: 30

  # Log level: info, debug
  log_level: info

  # Language: en, de, nl
  language: nl

  # Smart grouping configuration
  grouping:
    enabled: true
    columns: 3
    tag_frequency_threshold: 0.9
    min_services_per_group: 2

  # Traefik API configuration
  traefik:
    api_host: http://traefik:8080
    enable_basic_auth: false
    insecure_skip_verify: false
    basic_auth:
      username: username
      password: password
      password_file: /run/secrets/basic_auth_password
```

### Mounting the Configuration File

To use a configuration file with Docker, mount it into the container:

```yaml
# docker-compose.yml
services:
  trala:
    image: trala
    volumes:
      - ./configuration.yml:/config/configuration.yml:ro
```

## Environment Variables

Environment variables override settings from the configuration file. Variable names are derived by converting the YAML key path to uppercase and replacing dots with underscores.

### Docker Compose Example

Here's how to configure TraLa using environment variables in Docker Compose:

```yaml
# docker-compose.yml
services:
  trala:
    image: trala
    environment:
      - TRAEFIK_API_HOST=http://traefik:8080
      - REFRESH_INTERVAL_SECONDS=30
      - LOG_LEVEL=info
      - LANGUAGE=en
      - GROUPING_ENABLED=true
      - GROUPED_COLUMNS=3
```

### Common Variables

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `TRAEFIK_API_HOST` | The full base URL of your Traefik API | (required) |
| `REFRESH_INTERVAL_SECONDS` | Auto-refresh interval | `30` |
| `SEARCH_ENGINE_URL` | Search engine URL | `https://www.google.com/search?q=` |
| `LOG_LEVEL` | Log level: `info` or `debug` | `info` |
| `LANGUAGE` | Language: `en`, `de`, or `nl` | `en` |
| `SELFHST_ICON_URL` | Base URL for icon endpoint | `https://cdn.jsdelivr.net/gh/selfhst/icons/` |

### Grouping Variables

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `GROUPING_ENABLED` | Enable smart grouping | `true` |
| `GROUPED_COLUMNS` | Number of columns (1-6) | `3` |
| `GROUPING_TAG_FREQUENCY_THRESHOLD` | Tag frequency threshold (0.0-1.0) | `0.9` |
| `GROUPING_MIN_SERVICES_PER_GROUP` | Min services per group | `2` |

### Traefik API Variables

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `TRAEFIK_ENABLE_BASIC_AUTH` | Enable basic auth | `false` |
| `TRAEFIK_INSECURE_SKIP_VERIFY` | Skip SSL verification | `false` |
| `TRAEFIK_BASIC_AUTH_USERNAME` | Basic auth username | - |
| `TRAEFIK_BASIC_AUTH_PASSWORD` | Basic auth password | - |
| `TRAEFIK_BASIC_AUTH_PASSWORD_FILE` | Path to password file | - |

## Language Settings

TraLa supports three languages:

- `en` — English (default)
- `de` — German
- `nl` — Dutch

Set the language using the `LANGUAGE` environment variable or the `language` key in the configuration file.

## Logging

Set the log level using the `LOG_LEVEL` environment variable:

- `info` — Default, shows informational messages
- `debug` — Verbose logging, useful for troubleshooting icon-finding issues

To view the effective configuration at startup, enable debug logging.
