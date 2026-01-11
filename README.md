# TraLa - Traefik Landing Page

A simple, modern, and dynamic dashboard for your Traefik services. This application automatically discovers services via the Traefik API and displays them in a clean, responsive grid. It's designed to be run as a lightweight, multi-arch Docker container.

![Example](.assets/trala-dashboard.png)

## ✨ Features

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

---

## 🚀 Getting Started

The easiest way to get started is by using the pre-built Docker image from the GitHub Container Registry (primary) or Docker Hub (secondary).

### `docker-compose.yml` (Recommended)

This is the recommended approach. Add this service to your existing `docker-compose.yml` file.

```yaml
services:
  traefik:
    image: "traefik:v3.0"
    # ... your existing traefik configuration ...
    command:
      # ...
      - "--api.insecure=true" # Required for the dashboard to access the API
    networks:
      - traefik-net # A shared network

  trala:
    # Use either GitHub Container Registry (primary) or Docker Hub (secondary)
    image: ghcr.io/dannybouwers/trala:latest  # Primary source
    # image: dannybouwers/trala:latest       # Secondary source (Docker Hub)
    container_name: trala
    restart: unless-stopped
    networks:
      - traefik-net # Must be on the same network as Traefik
    volumes:
      # Optional: Mount a configuration file. See "Configuration" section below.
      - ./configuration.yml:/config/configuration.yml:ro
      # Optional: Mount a directory with custom icons. See "Configuration" section below.
      - ./icons:/icons:ro
    environment:
      # Required: The internal Docker network address for the Traefik API
      - TRAEFIK_API_HOST=http://traefik:8080
      # Optional: Change refresh interval
      - REFRESH_INTERVAL_SECONDS=30
      # Optional: Change the search engine
      - SEARCH_ENGINE_URL=https://duckduckgo.com/?q=
      # Optional: Set to "debug" for verbose icon-finding logs
      - LOG_LEVEL=info
    labels:
      # --- Traefik Labels to expose TraLa itself ---
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

## 🔧 Configuration

The application can be configured with a configuration file and with environment variables. Environment overwrite settings from the configuration file. To view the effective configuration after startup, enable debug logging.

A sample configuration file is shown below:

```yaml
# TraLa Configuration File
# Version 3.3

version: 3.3

# Environment settings (optional, environment variables take precedence)
environment:
  selfhst_icon_url: https://cdn.jsdelivr.net/gh/selfhst/icons/
  search_engine_url: https://duckduckgo.com/?q=
  refresh_interval_seconds: 30
  log_level: info
  language: de  # change Language (Default "en").
  grouping:
    enabled: true
    columns: 3  # Number of columns in grouped view (1-6), default: 3
    tag_frequency_threshold: 0.9  # Threshold for excluding tags present in more than 90% of services
    min_services_per_group: 2  # Minimum number of services required for a tag to form a group (default: 2)
  traefik:
    api_host: http://traefik:8080
    enable_basic_auth: true
    insecure_skip_verify: false # Skip SSL certificate verification for Traefik API (default: false)
    basic_auth:
      username: username
      password: password # mutually exclusive with password_file
      password_file: /run/secrets/basic_auth_password # mutually exclusive with password

# Service configuration
services:
  # Services to exclude from the dashboard
  exclude:
    routers:
      - "traefik-api"  # Hide the Traefik API itself
      - "admin-panel"  # Hide internal admin interface
      - "api*" # Hide all routers starting with "api"
    entrypoints:
      - "*lan*"        # Hide services using entrypoints containing "lan"
      - "internal"     # Hide services using the "internal" entrypoint

  # Service overrides for display names and icons
  overrides:
    # Override both display name and icon
    - service: "firefly-core"
      display_name: "Firefly III"
      icon: "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/firefly-iii.svg"
    - service: "unifi-controller"
      display_name: "UniFi Network"
      icon: "ubiquiti-unifi.svg"
      group: "Network"
    - service: "home-assistant"
      display_name: "Home Assistant"
      icon: "home-assistant.svg"
    - service: "nextcloud"
      display_name: "NextCloud"
      icon: "nextcloud.svg"
    - service: "portainer"
      display_name: "Portainer"
      icon: "portainer.svg"
    
    # Override only icon
    - service: "plex"
      icon: "plex.webp"
    - service: "unknown-service"
      icon: "https://selfh.st/content/images/2023/09/favicon-1.png"
    
    # Override for search engine icon
    - service: "searxng-domain"
      icon: "searxng.svg"
    - service: "duckduckgo"
      icon: "https://example.com/ddg-icon.png"
  
  # Manually added services (not from Traefik)
  manual:
    # Basic manual service with just name and URL
    - name: "Reddit"
      url: "https://www.reddit.com"
    
    # Manual service with custom icon
    - name: "GitHub"
      url: "https://github.com"
      icon: "github.svg"
    
    # Manual service with icon and priority
    - name: "The Verge"
      url: "https://www.theverge.com"
      icon: "https://www.theverge.com/favicon.ico"
      priority: 100
    
    # Manual service with just name, URL, and priority (icon will be auto-detected)
    - name: "Hacker News"
      url: "https://news.ycombinator.com"
      priority: 90
```

Supported environment variables are shown below.

| Variable                   | Description                                                                                             | Default                                | Required |
| -------------------------- | ------------------------------------------------------------------------------------------------------- | -------------------------------------- | -------- |
| `TRAEFIK_API_HOST`         | The full base URL of your Traefik API. From within Docker, this is typically `http://traefik:8080`.        | `(none)`                               | **Yes** |
| `SELFHST_ICON_URL`         | Base URL of the Selfhst icon endpoint. Customize if you are hosting your own local instance. | `https://cdn.jsdelivr.net/gh/selfhst/icons/`                               | No |
| `REFRESH_INTERVAL_SECONDS` | The interval in seconds at which the service list automatically refreshes.                                | `30`                                   | No       |
| `SEARCH_ENGINE_URL`        | The URL for the external search engine. The search query will be appended to this URL.                    | `https://www.google.com/search?q=`     | No       |
| `LOG_LEVEL`                | Set to `debug` for verbose logging of the icon-finding process. Any other value is silent.              | `info`                                 | No       |
| `GROUPING_ENABLED`         | Enable or disable the smart grouping feature.                                                            | `true`                                 | No       |
| `GROUPED_COLUMNS`          | Number of columns in grouped view for xl screen size (1-6).                                              | `3`                                    | No       |
| `GROUPING_TAG_FREQUENCY_THRESHOLD` | Threshold for excluding tags present in more than this percentage of services (0.0-1.0). | `0.9`                                 | No       |
| `GROUPING_MIN_SERVICES_PER_GROUP` | Minimum number of services required for a tag to form a group (must be >= 1).              | `2`                                    | No       |
| `TRAEFIK_BASIC_AUTH_USERNAME`      | Sets the username for the Traefik basic auth scheme if enabled.                                                                                   | `(none)`                                     | No       |
| `TRAEFIK_BASIC_AUTH_PASSWORD`      | Sets the password for the Traefik basic auth scheme if enabled.                                                                                   | `(none)`                                     | No       |
| `TRAEFIK_BASIC_AUTH_PASSWORD_FILE` | Sets the file path from where to load the password for the Traefik basic auth scheme if enabled. Takes precedence over setting password directly. | `(none)`                                     | No       |
| `TRAEFIK_INSECURE_SKIP_VERIFY` | Skip SSL certificate verification for Traefik API connections (similar to `providers.http.tls.insecureSkipVerify` in Traefik). | `false`                                     | No       |
| `LANGUAGE`                 | The language of the application.                 | `en`                                   | No       |

### Switching Languages

The application's language can be configured either through the `LANGUAGE` environment variable or the `configuration.yml` file.

#### Available Languages
- `en` (English)
- `de` (German)
- `nl` (Dutch)

If no language is specified, the default language is English (`en`).

### Service Exclusion

You can hide specific services from appearing in the dashboard by specifying their router names in the `configuration.yml` file under the `exclude` key (`exclude.routers`). Wildcard patterns (*, ?) are supported, allowing flexible matching of multiple services. This is useful for hiding administrative interfaces or services you don't want to be easily accessible through the dashboard.

Additionally, you can exclude services based on their entrypoint names using the `exclude.entrypoints` key. This allows you to hide entire categories of services (e.g. internal-only or admin interfaces) by filtering on the Traefik entrypoint they use.

#### How It Works

The application uses the **router name** from your Traefik configuration (the part before the `@`) to identify services. By adding router names to the `exclude.routers`, those services will not be processed or displayed in the dashboard.

Similarly, if a router uses an entrypoint that matches a pattern in the `exclude.entrypoints` list, it will also be excluded. Wildcard patterns are supported here as well.

### Service Overrides

TraLa provides unified service overrides that allow you to customize both the display name and icon for your services. This is the most powerful feature for customizing your dashboard.

#### How It Works

The application uses the **router name** from your Traefik configuration (the part before the `@`) as the primary identifier for a service. You can map this router name to:

1. A custom display name (for better readability)
2. A specific icon (full URL or filename from selfh.st)
3. Both display name and icon

**This override has the highest priority.** If a router name is found in this file, TraLa will use the specified display name and/or icon and skip all other detection methods.

#### Configuration Options

Each service override can include:
- `service`: The router name to match (required)
- `display_name`: Custom display name (optional)
- `icon`: Icon override (optional)

When using a filename from the selfh.st icon repository, you can specify files with the following extensions:

- `.png` (default if no extension specified)
- `.svg`
- `.webp`

The application will automatically construct the appropriate URL based on the file extension

### Smart Grouping

Smart Grouping allows you to organize services into collapsible groups for better dashboard organization. This feature provides both automatic grouping based on service tags and manual control over group assignments.

#### Configuration Options

- `grouping.enabled`: Enable or disable the smart grouping feature (default: `true`). When enabled, services are automatically grouped based on tags from selfh.st icon metadata, and groups can be collapsed or expanded individually via the frontend toggle.

- `grouping.columns`: Control the number of columns displayed in grouped view for extra-large screens (1-6 columns, default: 3). The grouped view always shows 1 column on mobile devices and 2 columns on medium screens (tablets), with the configured number applying only to xl screen sizes.

- `grouping.tag_frequency_threshold`: Control which tags are used for automatic grouping by setting a threshold (0.0-1.0, default: 0.9). Tags present in more than this percentage of services are excluded to avoid overly broad groups. For example, a threshold of 0.9 excludes tags found in more than 90% of services, preventing common tags from creating unhelpful groupings.

- `grouping.min_services_per_group`: Set the minimum number of services required for a tag to form a group (must be >= 1, default: 2). Tags with fewer services than this threshold are excluded from automatic grouping. This helps ensure that groups contain a meaningful number of services.

- `group`: In service overrides, use the `group` field to manually assign services to specific groups, overriding automatic tag-based grouping.

These settings can be configured via the YAML configuration file or environment variables (`GROUPING_ENABLED`, `GROUPED_COLUMNS`, `GROUPING_TAG_FREQUENCY_THRESHOLD`, `GROUPING_MIN_SERVICES_PER_GROUP`).

### Custom Icon Directory

For ultimate customization, you can mount a directory containing your own icons at `/icons`. TraLa will scan this directory and use fuzzy matching to find the best icon for each service. This feature has priority over the Selfhst icon endpoint.

#### How It Works

1. Mount a directory containing your icon files to the `/icons` volume in the container
2. TraLa will perform a fuzzy search against the icon names to find the best match
3. Supported icon formats are: `.png`, `.jpg`, `.jpeg`, `.svg`, `.webp`, and `.gif`
4. The icon name is derived from the filename (without extension) and case-insensitive

For example, if you have a file named `MyApp.png` in your icons directory, it will match services with names like "myapp", "my-app", etc.

### Search Engine Icon

The search bar displays a greyscale icon of your configured search engine. The icon is determined using the exact same logic as Traefik services, including support for icon overrides and custom icons.

**How it works:**
- The search engine is treated as a service using the second-level domain of the search URL. For example: `duckduckgo` from `https://duckduckgo.com/?q=` or `google` from `https://www.google.com/search?q=`
- Icon overrides work the same way - you can override the search engine icon using the service name (e.g., `google`, `duckduckgo`)

### Manual Services

Sometimes you want to add services to your dashboard that aren't managed by Traefik - like external websites, cloud services, or resources hosted elsewhere. TraLa allows you to manually add these services through the configuration file.

#### How It Works

Manual services are defined in the `manual` section of your `configuration.yml` file. These services:

1. Are merged with Traefik-discovered services and displayed together
2. Use the same icon detection logic as Traefik services
3. Support all icon options (auto-detection, custom icons, etc.)

#### Configuration Options

Each manual service can include:
- `name`: The display name (required)
- `url`: The URL of the service (required)
- `icon`: Custom icon (optional - full URL or filename from selfh.st)
- `priority`: Priority for sorting (optional - higher numbers appear first, default: 50)

---

## 🔒 Traefik API Access and security (Advanced)

Instead of using `--api.insecure=true` in your Traefik configuration, you can create a dedicated router for the API. This approach allows fine-grained control over API access and security.

 - Define a router rule for accessing the Traefik API from other services. This will not add security.
 - Implement security features using middlewares such as allowlisting or basicAuth as described below. This doesn't require `--api.insecure=false`.

### Router rule

If TraLa is deployed in the same Docker network as Traefik, the router should also work within the network. This can be accomplished by adding the internal Traefik hostname as a host in the router of Traefik. TraLa will automatically ignore the service created for connecting to Traefik's API.

#### Example Configuration

```yaml
version: '3.8'
services:
  traefik:
    image: "traefik:v3.0"
    # ... your existing Traefik configuration ...
    command:
      # ...
      - --api # Secure API (instead of --api.insecure=true)
      - --entrypoints.web.address=:80
      # - ...
    labels:
      # ...
      # Dashboard & API
      - traefik.http.routers.traefik-api.entrypoints=web
      - traefik.http.routers.traefik-api.rule=Host(`traefik`) && PathPrefix(`/api`) # Use container hostname for internal access
      - traefik.http.routers.traefik-api.service=api@internal

  trala:
    # ... your existing TraLa configuration ...
    environment:
      - TRAEFIK_API_HOST=http://traefik # Use service name of traefik container with entrypoint port (80 in this example)
```

### Allowlisting Method

To add allowlisting to the Traefik API, the TraLa service must have a static IP. This is required because the IPAllowList middleware needs to know exactly which IP addresses are allowed to access the API. Define a network with a specific IP subnet and specify the internal IP address for TraLa:

```yaml
networks:
  traefik-proxy-network:
    name: traefik-proxy-network
    ipam:
      config:
        - subnet: 172.20.0.0/16 # IP subnet for your Docker network

services:
  trala:
    # ... your existing TraLa configuration ...
    networks:
      traefik-proxy-network:
        ipv4_address: 172.20.30.40 # Static IP within the defined subnet
```

Insert an IPAllowList middleware to the router that exposes the API:

```yaml
services:
  traefik:
    # ... your existing Traefik configuration ...
    networks:
      - traefik-proxy-network
    labels:
      # Limit access to the dashboard and API to the IP of the TraLa container
      - traefik.http.middlewares.traefik-api-allowlist.ipallowlist.sourcerange=172.20.30.40/32
      # API
      # ...
      - traefik.http.routers.traefik-api.middlewares=traefik-api-allowlist
```

### Basic Auth Method

To add basic auth to the Traefik API, insert a basic auth middleware into the router that exposes the API. Create the hashed credentials for the middleware:

``` bash
echo $(htpasswd -nbB <USERNAME> <PASSWORD>) | sed -e s/\\$/\\$\\$/g
```

Replace the `<REPLACE_ME>` tag with the resulting string:

```yaml
services:
  traefik:
    # ... your existing Traefik configuration ...
    labels:
      # Define the basic auth username and hashed password
      - "traefik.http.middlewares.traefik-api-auth.basicauth.users=<REPLACE_ME>"
      # API
      # ...
      - "traefik.http.routers.traefik-api.middlewares=traefik-api-auth"
```

> [!NOTE]
> The Traefik API will be reachable on all routes that use the `api@internal` service. If you have a router that exposes the Traefik dashboard, the API will be reachable there as well. Ensure there is authentication in place on all routers!

Enable basic auth in the configuration file with the `environment.traefik.enable_basic_auth` setting:

```yaml
environment:
  traefik:
    enable_basic_auth: true
```

There are three ways to specify the credentials for the basic auth scheme in TraLa, where the lower number takes precedence over higher numbers:

1. Docker Secret (Recommended)
2. Environment Variable
3. Configuration File

> [!IMPORTANT]
> Note that while we use the password hash for Traefik middleware, the plain password must be specified for TraLa using any of the methods below.

#### Docker Secret (Recommended)

To use a Docker secret, create a credentials file:

```bash
echo "<PASSWORD>" > basic_auth_password.txt
```

Add the file as Docker secret in the Docker compose:

```yaml
services:
  trala:
    # ... your existing TraLa configuration ...
    secrets:
      - basic_auth_password
    
secrets:
  basic_auth_password:
    file: ./basic_auth_password.txt
```

To point TraLa to the secret, either specify the path in the configuration file:

```yaml
environment:
  traefik:
    basic_auth:
      username: <USERNAME>
      password_file: /run/secrets/basic_auth_password
```

Or specify the path as environment variable:

```yaml
services:
  trala:
    # ... your existing TraLa configuration ...
    environment:
      - TRAEFIK_BASIC_AUTH_USERNAME=<USERNAME>
      - TRAEFIK_BASIC_AUTH_PASSWORD_FILE=/run/secrets/basic_auth_password
```

#### Environment Variable

To specify the credentials with environment variables, add the `TRAEFIK_BASIC_AUTH_USERNAME` and `TRAEFIK_BASIC_AUTH_PASSWORD` variables to your Docker compose file:

```yaml
services:
  trala:
    # ... your existing TraLa configuration ...
    environment:
      - TRAEFIK_BASIC_AUTH_USERNAME=<USERNAME>
      - TRAEFIK_BASIC_AUTH_PASSWORD=<PASSWORD>
```

#### Configuration File

To store the credentials as part of the configuration file, specify the following settings:

```yaml
environment:
  traefik:
    basic_auth:
      username: <USERNAME>
      password: <PASSWORD>
```

---

## 🛠️ Building Locally

If you want to build the image yourself:

1. **Clone the repository:**

    ```bash
    git clone https://github.com/dannybouwers/trala.git
    cd trala
    ```

2. **Build the Docker image:**

    ```bash
    docker build -t trala .
    ```

3. **Run the locally built image:**

    ```bash
    docker run -d -p 8080:8080 -e TRAEFIK_API_HOST="http://<your-traefik-ip>:8080" trala
    ```

---

## 📜 License

This project is licensed under the MIT License. See the `LICENSE` file for details.

---

## 🙏 Acknowledgements

This project was initially developed in close collaboration with Google's Gemini. I provided the architectural direction, feature requirements, and debugging, while Gemini handled the bulk of the code generation. This transparent, AI-assisted approach allowed for rapid development and iteration.

Special thanks to:

- **[Maria Letta](https://github.com/MariaLetta/free-gophers-pack)** for the wonderful Gopher logo used in the application.
- **[selfh.st/icons](https://selfh.st/icons/)** for providing the extensive, high-quality icon database that powers the service icon discovery.
