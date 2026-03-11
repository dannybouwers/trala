# Development

This guide covers how to develop and contribute to TraLa.

## Building from Source

### Prerequisites

- Go 1.21+
- Node.js 25+ (for Tailwind CSS)
- Docker
- Docker Compose

### Build Steps

1. Clone the repository:
   ```bash
   git clone https://github.com/dannybouwers/trala.git
   cd trala
   ```

2. Build the Docker image (this also builds Tailwind CSS):
   ```bash
   docker build -t trala .
   ```

3. Run the locally built image:
   ```bash
   docker run -d -p 8080:8080 -e TRAEFIK_API_HOST="http://<your-traefik-ip>:8080" trala
   ```

### Building Tailwind CSS Manually

If you need to rebuild Tailwind CSS separately:

```bash
cd web/html
npm install tailwindcss @tailwindcss/cli
npx @tailwindcss/cli -i tailwind.src.css -o ../css/tailwind.css
```

## Local Development with Demo Stack

TraLa includes a demo stack for testing. The demo uses Docker Compose and includes mock services routed by Traefik.

### Demo Files

- `demo/docker-compose.yml` — Stack definition
- `demo/configuration.yml` — Test configuration

### Running the Demo

1. Navigate to the demo directory:
   ```bash
   cd demo
   ```

2. Build the application:
   ```bash
   docker compose build
   ```

3. Start the testing stack:
   ```bash
   docker compose up -d
   ```

4. Access the dashboard at `https://trala.localhost`

5. When finished, stop the stack:
   ```bash
   docker compose down
   ```

> [!IMPORTANT]
> The demo uses HTTPS with self-signed certificates. You may need to accept the security warning in your browser or add the certificate to your trust store.

### Demo Services

The demo stack includes:
- Traefik with the whoami service
- TraLa dashboard accessible at `https://trala.localhost`
- Various test routers (subscription-manager, firefly-iii, jellyfin, plex, portainer, etc.)

## Project Structure

```
trala/
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/          # Configuration handling
│   ├── handlers/        # HTTP handlers
│   ├── i18n/            # Internationalization
│   ├── icons/           # Icon detection and caching
│   ├── models/          # Data models
│   ├── services/        # Service processing and grouping
│   └── traefik/         # Traefik API client
├── web/
│   ├── css/             # Stylesheets
│   ├── html/            # HTML templates
│   ├── img/             # Images and icons
│   └── js/              # JavaScript
├── translations/        # Language files
├── docs/                # Documentation
└── demo/                # Demo stack
```

### Key Packages

| Package | Description |
|---------|-------------|
| `internal/config` | Configuration file and environment variable parsing |
| `internal/traefik` | Traefik API client |
| `internal/services` | Service discovery, processing, and grouping |
| `internal/icons` | Icon detection and caching |
| `internal/handlers` | HTTP request handlers |
| `internal/i18n` | Internationalization |

## Testing Approach

Testing is performed using the demo stack:

1. Changes are tested by running the application
2. Verify behavior by accessing the running API
3. Configuration changes are tested by observing application output

The demo stack provides:
- Mock services via Traefik's whoami
- Test configuration in `demo/configuration.yml`
- Realistic Traefik routing setup
