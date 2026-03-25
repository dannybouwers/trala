# Manual Services

Add custom services that aren't managed by Traefik. This is useful for adding external links, internal tools, or services that aren't exposed through Traefik.

## Examples

### Automatic Icon and Group

The simplest manual service configuration. TraLa automatically detects the icon based on the service name and assigns it to a group based on auto-detection.

```yaml
services:
  manual:
    - name: "Reddit"
      url: "https://www.reddit.com"
```

### Custom Icon and Group

Use a specific icon from the selfh.st icon repository and assign to a custom group.

```yaml
services:
  manual:
    - name: "GitHub"
      url: "https://github.com"
      icon: "github.svg"
      group: "Development"
```

### External Icon URL

Use an external icon URL for services not in the selfh.st repository.

```yaml
services:
  manual:
    - name: "The Verge"
      url: "https://www.theverge.com"
      icon: "https://www.theverge.com/favicon.ico"
      priority: 100
```

## Configuration Options

| Option | Required | Description | Default |
|--------|----------|-------------|---------|
| `name` | Yes | Display name | - |
| `url` | Yes | Service URL | - |
| `icon` | No | Custom icon (URL or filename). See [Icons](/docs/icons) for details. | Auto-detected |
| `priority` | No | Sort priority (higher = first) | 50 |
| `group` | No | Assign to a specific group | Auto-grouped |

> [!NOTE]
> Manual services are merged with Traefik-discovered services and use the same icon detection logic.