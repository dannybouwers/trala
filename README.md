# TraLa - Traefik Landing Page

A simple, modern, and dynamic dashboard for your Traefik services. This application automatically discovers services via the Traefik API and displays them in a clean, responsive grid. It's designed to be run as a lightweight, multi-arch Docker container.

![Example](.assets/trala-dashboard.png)

## ✨ Features

- **Auto-Discovery:** Automatically fetches and displays all HTTP routers from your Traefik instance.
- **Manual Services:** Add custom services to your dashboard that aren't managed by Traefik (e.g., Reddit, GitHub, external websites).
- **Advanced Icon Fetching:** Intelligently finds the best icon for each service using a robust, prioritized strategy.
- **Icon Overrides:** Manually map router names to specific icons for perfect results every time.
- **Custom Icon Directory:** Mount your own icon directory at `/icons` for ultimate customization with fuzzy matching.
- **Modern UI:** Clean, responsive interface with automatic Light/Dark mode based on your OS settings.
- **Live Search & Sort:** Instantly filter and sort your services by name, URL, or priority.
- **External Search:** Use the search bar to quickly search the web with your configured search engine.
- **Lightweight & Multi-Arch:** Built with Go and a minimal Alpine base, the Docker image is small and compatible with `amd64` and `arm64` architectures.
- **Service Exclusion:** Hide specific services from the dashboard using router name exclusions.

---

## 🚀 Getting Started

The easiest way to get started is by using the pre-built Docker image from the GitHub Container Registry.

### `docker-compose.yml` (Recommended)

This is the recommended approach. Add this service to your existing `docker-compose.yml` file.

```yaml
version: '3.8'

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
    image: ghcr.io/dannybouwers/trala:latest
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
# Version 2.0

version: 2.0

# Environment settings (optional, environment variables take precedence)
environment:
  selfhst_icon_url: https://cdn.jsdelivr.net/gh/selfhst/icons/
  search_engine_url: https://duckduckgo.com/?q=
  refresh_interval_seconds: 30
  log_level: info
  traefik:
    api_host: http://traefik:8080
    enable_basic_auth: true
    basic_auth:
      username: user
      password: pass  

# Service configuration
services:
  # Services to exclude from the dashboard
  exclude:
    - "traefik-api"  # Hide the Traefik API itself
    - "admin-panel"  # Hide internal admin interface
    - "api*" # Hide all routers starting with "api"
  
  # Service overrides for display names and icons
  overrides:
    # Override both display name and icon
    - service: "firefly-core"
      display_name: "Firefly III"
      icon: "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/firefly-iii.svg"
    - service: "unifi-controller"
      display_name: "UniFi Network"
      icon: "ubiquiti-unifi.svg"
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
| `TRAEFIK_BASIC_AUTH_USER`  | Sets the username for the Traefik basic auth scheme if enabled.              | `(none)`                                 | No       |
| `TRAEFIK_BASIC_AUTH_FILE`  | Sets the file path from where to load the password for the Traefik basic auth scheme if enabled.         | `(none)`                                 | No       |

### Service Exclusion

You can hide specific services from appearing in the dashboard by specifying their router names in the `configuration.yml` file with the `exclusions` key. Wildcard patterns (*, ?) are supported, allowing flexible matching of multiple services. This is useful for hiding administrative interfaces or services you don't want to be easily accessible through the dashboard.

#### How It Works

The application uses the **router name** from your Traefik configuration (the part before the `@`) to identify services. By adding router names to the exclusion list, those services will not be processed or displayed in the dashboard.

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

# 🔒 Secure Traefik API Access (Advanced)

Instead of using `--api.insecure=true` in your Traefik configuration, you can create a dedicated router for the API. This approach is more secure as it allows fine-grained control over API access.

### How It Works

If TraLa is deployed in the same Docker network as Traefik, the router should also work within the network. This can be accomplished by adding the internal Traefik hostname as a host in the router of Traefik.

### Example Configuration

```yaml
version: '3.8'
services:
  traefik:
    image: "traefik:v3.0"
    hostname: traefik # <-- specify the hostname for this container
    # ... your existing traefik configuration ...
    command:
      # ...
      - --api # Secure API
      - --entrypoints.web.address=:80
      # - ...
    labels:
      # ...
      # Dashboard & API
      - traefik.http.routers.traefik-api.entrypoints=web
      - traefik.http.routers.traefik-api.rule=Host(`traefik`) && PathPrefix(`/api`) # <-- use the container hostname in the router rule
      - traefik.http.routers.traefik-api.service=api@internal

  trala:
    # ... your existing traefik configuration ...
    environment:
      - TRAEFIK_API_HOST=http://traefik # <-- specify the hostname of the traefik container and the port of the entrypoint (if not protocol default)
```
With this configuration, you can remove the `--api.insecure=true` flag from your Traefik configuration, making your setup more secure. TraLa will automatically ignore the service created for connecting to Traefik's API.

# Traefik Basic Auth

To secure the Traefik API access with basic auth, create a credentials file:

```bash
echo "<PASSWORD>" > basic_auth_password.txt
```

Add the file as Docker secret in the Docker compose:

```yaml
services:
  trala:
    [...]
    secrets:
      - basic_auth_password
    
secrets:
  basic_auth_password:
    file: ./basic_auth_password.txt
```

To point Trala to the secret, either specify the path in the configuration file:

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
    [...]
    environment:
      - TRAEFIK_BASIC_AUTH_FILE=/run/secrets/basic_auth_password
```

To add basic auth to the Traefik API, insert a basic auth middleware into the router that exposes the API. To create the hashed credentials for the middleware, use `echo $(htpasswd -nB user) | sed -e s/\\$/\\$\\$/g`. Replace the resulting string with the `<REPLACE_ME>` tag:

```yaml
- "traefik.http.routers.internal-api.entrypoints=traefik-internal"
- "traefik.http.routers.internal-api.rule=PathPrefix(`/api`)"
- "traefik.http.routers.internal-api.service=api@internal"
- "traefik.http.routers.internal-api.middlewares=auth"
- "traefik.http.middlewares.auth.basicauth.users=<REPLACE_ME>"
```

Finally, enable basic auth in the configuration file with the `environment.traefik.enable_basic_auth` setting:

```yaml
environment:
  traefik:
    enable_basic_auth: true
```

---

# 🛠️ Building Locally

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
