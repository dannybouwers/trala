# Security

This guide covers how to secure access to the Traefik API when using TraLa.

## Traefik API Access Methods

Instead of using `--api.insecure=true` in your Traefik configuration, you can create a dedicated router for the API. This approach allows fine-grained control over API access and security.

 - [Define a router rule](#router-rule) for accessing the Traefik API from other services. This will not add security.
 - Implement security features using middlewares such as [allowlisting](#allowlisting-method) or [basicAuth](#basic-auth-method). This doesn't require `--api.insecure=false`.

## Router Rule

If TraLa is deployed in the same Docker network as Traefik, the router should also work within the network. This can be accomplished by adding the internal Traefik hostname as a host in the router of Traefik. TraLa will automatically ignore the service created for connecting to Traefik's API.

```yaml
# docker-compose.yml
services:
  traefik:
    image: "traefik:v3.0"
    command:
      - --api
      - --entrypoints.web.address=:80
    labels:
      # Dashboard & API
      - traefik.http.routers.traefik-api.entrypoints=web
      - traefik.http.routers.traefik-api.rule=Host(`traefik`) && PathPrefix(`/api`)
      - traefik.http.routers.traefik-api.service=api@internal

  trala:
    environment:
      - TRAEFIK_API_HOST=http://traefik
```

## Allowlisting Method

To add allowlisting to the Traefik API, the TraLa service must have a static IP. This is required because the IPAllowList middleware needs to know exactly which IP addresses are allowed to access the API. Define a network with a specific IP subnet and specify the internal IP address for TraLa:

### 1. Configure Static IP for TraLa

```yaml
# docker-compose.yml
networks:
  traefik-proxy-network:
    name: traefik-proxy-network
    ipam:
      config:
        - subnet: 172.20.0.0/16

services:
  trala:
    networks:
      traefik-proxy-network:
        ipv4_address: 172.20.30.40
```

### 2. Add IPAllowList Middleware

```yaml
# docker-compose.yml
services:
  traefik:
    labels:
      # Limit access to TraLa's IP
      - traefik.http.middlewares.traefik-api-allowlist.ipallowlist.sourcerange=172.20.30.40/32
      # Apply to API router
      - traefik.http.routers.traefik-api.middlewares=traefik-api-allowlist
```

## Basic Auth Method

Add basic authentication to the Traefik API.

### 1. Generate Credentials

```bash
echo $(htpasswd -nbB <USERNAME> <PASSWORD>) | sed -e s/\\$/\\$\\$/g
```

### 2. Configure Traefik Middleware

```yaml
# docker-compose.yml
services:
  traefik:
    labels:
      # Replace <REPLACE_ME> with the output from htpasswd
      - traefik.http.middlewares.traefik-api-auth.basicauth.users=<REPLACE_ME>
      - traefik.http.routers.traefik-api.middlewares=traefik-api-auth
```

### 3. Enable Basic Auth in TraLa

```yaml
# configuration.yml
environment:
  traefik:
    enable_basic_auth: true
```

## Credential Methods

TraLa supports three ways to specify basic auth credentials (in order of priority):

### Docker Secret (Recommended)

Create a credentials file:
```bash
echo "<PASSWORD>" > basic_auth_password.txt
```

Add to Docker compose:
```yaml
# docker-compose.yml
services:
  trala:
    secrets:
      - basic_auth_password

secrets:
  basic_auth_password:
    file: ./basic_auth_password.txt
```

Configure TraLa to use the secret:
```yaml
# configuration.yml
environment:
  traefik:
    basic_auth:
      username: <USERNAME>
      password_file: /run/secrets/basic_auth_password
```

Or via environment variable:
```yaml
# docker-compose.yml
environment:
  - TRAEFIK_BASIC_AUTH_USERNAME=<USERNAME>
  - TRAEFIK_BASIC_AUTH_PASSWORD_FILE=/run/secrets/basic_auth_password
```

### Environment Variable

```yaml
# docker-compose.yml
environment:
  - TRAEFIK_BASIC_AUTH_USERNAME=<USERNAME>
  - TRAEFIK_BASIC_AUTH_PASSWORD=<PASSWORD>
```

### Configuration File

```yaml
# configuration.yml
environment:
  traefik:
    basic_auth:
      username: <USERNAME>
      password: <PASSWORD>
```

?> While Traefik uses password hashes, TraLa requires the plain password.

!> The Traefik API will be reachable on all routes using `api@internal`. Ensure authentication is enabled on all routers that expose the dashboard!
