# TraLa - Traefik Landing Page

A simple, modern, and dynamic dashboard for your Traefik services. This application automatically discovers services via the Traefik API and displays them in a clean, responsive grid. It's designed to be run as a lightweight, multi-arch Docker container.

![Example](.assets/trala-dashboard.png)

## ✨ Features

- **Auto-Discovery:** Automatically fetches and displays all HTTP routers from your Traefik instance.
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
      - traefik_api_host=http://traefik:8080
      # Optional: Change refresh interval
      - refresh_interval_seconds=30
      # Optional: Change the search engine
      - search_engine_url=https://duckduckgo.com/?q=
      # Optional: Set to "debug" for verbose icon-finding logs
      - log_level=info
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
# Version 1.0

version: 1.0

# Environment settings (optional, environment variables take precedence)
environment:
  selfhst_icon_url: https://cdn.jsdelivr.net/gh/selfhst/icons/
  search_engine_url: https://duckduckgo.com/?q=
  refresh_interval_seconds: 30
  log_level: info
  traefik:
    api_host: http://traefik:8080

# Icon customization
icons:
  # Path to directory with custom icons (optional)
  # Override icons for specific services
  overrides:
    # Using full URL
    - service: "firefly-core"
      icon: "https://selfh.st/content/images/2023/09/favicon-1.png"
    
    # Using selfh.st filename with extension
    - service: "unifi-controller"
      icon: "ubiquiti-unifi.png"
    - service: "home-assistant"
      icon: "home-assistant.svg"
    - service: "plex"
      icon: "plex.webp"
    
    # Using simple name (resolves to selfh.st)
    - service: "nextcloud"
      icon: "nextcloud"
    - service: "portainer"
      icon: "portainer"

# Service configuration
services:
  # Services to exclude from the dashboard
  exclude:
    - "traefik-api"  # Hide the Traefik API itself
    - "admin-panel"  # Hide internal admin interface
```

Supported environment variables are shown below.

| Variable                   | Description                                                                                             | Default                                | Required |
| -------------------------- | ------------------------------------------------------------------------------------------------------- | -------------------------------------- | -------- |
| `traefik_api_host`         | The full base URL of your Traefik API. From within Docker, this is typically `http://traefik:8080`.        | `(none)`                               | **Yes** |
| `selfhst_icon_url`         | Base URL of the Selfhst icon endpoint. Customize if you are hosting your own local instance. | `https://cdn.jsdelivr.net/gh/selfhst/icons/`                               | No |
| `refresh_interval_seconds` | The interval in seconds at which the service list automatically refreshes.                                | `30`                                   | No       |
| `search_engine_url`        | The URL for the external search engine. The search query will be appended to this URL.                    | `https://www.google.com/search?q=`     | No       |
| `log_level`                | Set to `debug` for verbose logging of the icon-finding process. Any other value is silent.              | `info`                                 | No       |

### Icon Overrides

While TraLa does its best to find the right icon, fuzzy search isn't perfect. For ultimate control, you can provide ovverides in the `configuration.yml` file with the `overrides` key. This is the most powerful feature for customizing your dashboard.

#### How It Works

The application uses the **router name** from your Traefik configuration (the part before the `@`) as the primary identifier for a service. You can map this router name to either:

1. A full URL to an icon (e.g., `https://selfh.st/content/images/2023/09/favicon-1.png`)
2. A specific icon filename from the [selfh.st icon repository](https://selfh.st/icons/)

**This override has the highest priority.** If a router name is found in this file, TraLa will use the specified icon and skip all other detection methods.

When using a filename from the selfh.st icon repository, you can specify files with the following extensions:

- `.png` (default if no extension specified)
- `.svg`
- `.webp`

The application will automatically construct the appropriate URL based on the file extension

### Custom Icon Directory

For ultimate customization, you can mount a directory containing your own icons at `/icons`. TraLa will scan this directory and use fuzzy matching to find the best icon for each service. This feature has the highest priority, even above icon overrides.

#### How It Works

1. Mount a directory containing your icon files to the `/icons` volume in the container
2. TraLa will scan this directory at startup and create a map of icon names to file paths
3. For each service, TraLa will perform a fuzzy search against the icon names to find the best match
4. Supported icon formats are: `.png`, `.jpg`, `.jpeg`, `.svg`, `.webp`, and `.gif`
5. The icon name is derived from the filename (without extension) and converted to lowercase
6. Icons are served directly by the application at the `/icons/filename.ext` URL

For example, if you have a file named `MyApp.png` in your icons directory, it will match services with names like "myapp", "my-app", etc. The icon will be served at `http://trala-hostname/icons/MyApp.png`.

### Service Exclusion

You can hide specific services from appearing in the dashboard by specifying their router names in the `configuration.yml` file with the `exclusions` key. This is useful for hiding administrative interfaces or services you don't want to be easily accessible through the dashboard.

#### How It Works

The application uses the **router name** from your Traefik configuration (the part before the `@`) to identify services. By adding router names to the exclusion list, those services will not be processed or displayed in the dashboard.

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
