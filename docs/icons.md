# Icons

TraLa automatically detects icons for your services using multiple methods, with support for custom icons.

## Auto-Detection via selfh.st

By default, TraLa uses [selfh.st/icons](https://selfh.st/icons) to automatically find the best icon for each service. This is the primary icon source and works for most popular services.

The icon detection uses fuzzy matching against the service name to find the best match in the selfh.st database.

### Configuration

```yaml
# configuration.yml
environment:
  selfhst_icon_url: https://cdn.jsdelivr.net/gh/selfhst/icons/
```

Set via environment variable: `SELFHST_ICON_URL=https://cdn.jsdelivr.net/gh/selfhst/icons/`

## Custom Icon Directory

For ultimate customization, mount a directory containing your own icons:

```yaml
# docker-compose.yml
services:
  trala:
    volumes:
      - ./icons:/icons:ro
```

### How It Works

1. Mount a directory with icon files to `/icons` in the container
2. TraLa performs fuzzy matching against icon filenames
3. Supported formats: `.png`, `.jpg`, `.jpeg`, `.svg`, `.webp`, `.gif`
4. Icon names are derived from filenames (without extension), case-insensitive

### Example

If your icons directory contains:
- `MyApp.png` → matches services named "myapp", "my-app", etc.
- `HomeAssistant.svg` → matches "home-assistant", "homeassistant"

## Icon Override Priority

The icon system follows this priority order (highest to lowest):

1. **Service override icon** — Explicitly set in configuration
2. **Custom icon directory** — Files mounted in `/icons`
3. **selfh.st icon database** — Auto-detection
4. **Default icon** — Fallback when no match found

## Service Icon Overrides

Override icons for specific services in your configuration:

```yaml
# configuration.yml
services:
  overrides:
    # Override with selfh.st icon
    - service: "plex"
      icon: "plex.webp"
    
    # Override with full URL
    - service: "unknown-service"
      icon: "https://example.com/icon.png"
    
    # Search engine icon override
    - service: "duckduckgo"
      icon: "https://example.com/ddg-icon.png"
```

## Search Engine Icon

The search bar displays a greyscale icon of your configured search engine. The icon is determined using the same logic as service icons.

The search engine is treated as a service using the second-level domain:
- `duckduckgo` from `https://duckduckgo.com/?q=`
- `google` from `https://www.google.com/search?q=`

Override the search engine icon using the service name (e.g., `google`, `duckduckgo`).
