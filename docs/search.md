# Search

TraLa includes an external search bar that allows you to quickly search the web using your configured search engine.

## Configuration

### Search Engine URL

```yaml
# configuration.yml
environment:
  search_engine_url: https://duckduckgo.com/?q=
```

Set via environment variable: `SEARCH_ENGINE_URL=https://duckduckgo.com/?q=`

### Example Search Engines

| Search Engine | URL |
|---------------|-----|
| DuckDuckGo | `https://duckduckgo.com/?q=` |
| Google | `https://www.google.com/search?q=` |
| Bing | `https://www.bing.com/search?q=` |
| Qwant | `https://www.qwant.com/?q=` |
| Ecosia | `https://www.ecosia.org/search?q=` |

## Search Engine Icon

The search bar displays a greyscale icon of your configured search engine.

### How It Works

The search engine icon is determined using the same logic as Traefik services:

1. Extract the second-level domain from the search URL
   - `https://duckduckgo.com/?q=` → `duckduckgo`
   - `https://www.google.com/search?q=` → `google`

2. Apply the same icon detection priority:
   - Service override icon
   - Custom icon directory
   - selfh.st icon database
   - Default icon

### Custom Search Engine Icon

Override the search engine icon using service overrides:

```yaml
# configuration.yml
services:
  overrides:
    - service: "duckduckgo"
      icon: "https://example.com/ddg-icon.png"
    
    - service: "google"
      icon: "google.svg"
```

## Live Search & Sort

Beyond external search, TraLa also provides:

- **Instant filtering** — Filter services by name, URL, or priority
- **Sorting** — Sort services by name or priority
- **Real-time updates** — Results update as you type
