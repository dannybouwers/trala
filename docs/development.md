# Development

This guide covers how to develop and contribute to TraLa.

## Testing and Development Options

Choose one of the following approaches to develop and test TraLa.

---

### Option 1: Docker Compose with Demo Stack

The demo stack provides a complete testing environment with mock services routed by Traefik.

#### Prerequisites

- Docker
- Docker Compose

#### Demo Files

- `demo/docker-compose.yml` — Stack definition
- `demo/configuration.yml` — Test configuration

#### Running the Demo

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

#### Demo Services

The demo stack includes:
- Traefik with the whoami service
- TraLa dashboard accessible at `https://trala.localhost`
- Various test routers (subscription-manager, firefly-iii, jellyfin, plex, portainer, etc.)

---

### Option 2: Build and Run with Dockerfile

Build the Docker image yourself and run it locally.

#### Prerequisites

- Docker

#### Build the Image

```bash
docker build -t trala .
```

This builds Tailwind CSS and compiles the Go application in a multi-stage build.

#### Run the Container

```bash
docker run -d -p 8080:8080 -e TRAEFIK_API_HOST="http://<your-traefik-ip>:8080" trala
```

Replace `<your-traefik-ip>` with your Traefik API host IP address.

#### Mount Custom Configuration (Optional)

To use a custom configuration file:

```bash
docker run -d -p 8080:8080 -v /path/to/your/configuration.yml:/config/configuration.yml trala
```

---

### Option 3: Manual Build (No Docker)

Build and run TraLa directly on your local machine without Docker. This requires Go and Node.js.

#### Prerequisites

- Go 1.21+
- Node.js 25+

#### Step 1: Clone the Repository

```bash
git clone https://github.com/dannybouwers/trala.git
cd trala
```

#### Step 2: Install Node.js Dependencies and Build Tailwind CSS

1. Navigate to the web directory:
   ```bash
   cd web/html
   ```

2. Install Tailwind CSS:
   ```bash
   npm install tailwindcss @tailwindcss/cli
   ```

3. Build the Tailwind CSS file:
   ```bash
   npx @tailwindcss/cli -i tailwind.src.css -o ../css/tailwind.css
   ```

The compiled Tailwind CSS will be output to `web/css/tailwind.css`.

#### Step 3: Build the Go Application

1. Return to the project root:
   ```bash
   cd ../..
   ```

2. Build the Go server:
   ```bash
   go build -o trala ./cmd/server/
   ```

#### Step 4: Run the Application

Create the required directory structure and run:

```bash
mkdir -p static template translations

# Copy frontend files to the correct locations
cp web/css/tailwind.css static/css/
cp web/css/trala.css static/css/
cp -r web/js/* static/js/
cp -r web/img/* static/img/
cp web/html/index.html template/index.html
cp translations/* translations/
```

Run the server:

```bash
TRAEFIK_API_HOST="http://<your-traefik-ip>:8080" ./trala
```

Replace `<your-traefik-ip>` with your Traefik API host IP address.

The application will start on `http://localhost:8080`.

---

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
