# Comprehensive Code Review — TraLa

**Date:** 2026-03-21
**Scope:** Full repository review covering backend logic, security, frontend, CI/CD, Docker, and configuration.

---

## Table of Contents

1. [Critical & High Severity Issues](#1-critical--high-severity-issues)
2. [Backend Logic Review](#2-backend-logic-review)
3. [Security Review](#3-security-review)
4. [Frontend Review](#4-frontend-review)
5. [CI/CD & Docker Review](#5-cicd--docker-review)
6. [Configuration & Translation Review](#6-configuration--translation-review)
7. [Summary Matrix](#7-summary-matrix)

---

## 1. Critical & High Severity Issues

These issues should be addressed as a priority.

### P0 — `text/template` Instead of `html/template` (XSS)

**File:** `internal/handlers/handlers.go`, line 16

The handlers package imports `"text/template"` instead of `"html/template"`. Go's `text/template` performs **no HTML escaping**. Any i18n translation string or data value containing `<`, `>`, `"`, or `&` will be injected raw into the HTML output. If an attacker can influence translation files (e.g., compromised volume mount, path traversal via `LANGUAGE` env var), they get full XSS.

**Fix:** Change the import from `"text/template"` to `"html/template"`.

### P0 — URL Construction Bug: Path Placed Before Port

**File:** `internal/traefik/traefik.go`, line 288

When a non-default port is used, the path is placed **before** the port:
```go
return fmt.Sprintf("%s://%s%s:%s", protocol, hostname, path, port)
```
This produces URLs like `https://example.com/app:8443` instead of `https://example.com:8443/app`. This is a functional bug that will break service links for any service with a non-default port and a path.

**Fix:** Swap the order: `fmt.Sprintf("%s://%s:%s%s", protocol, hostname, port, path)`

### P0 — `defer cancel()` in Loop Leaks Contexts

**File:** `internal/traefik/traefik.go`, line 154

Inside the `FetchAllPages` pagination loop, `defer cancel()` is called per iteration. Deferred functions only execute when the enclosing **function** returns, not at loop iteration end. This creates N leaked contexts during pagination.

**Fix:** Call `cancel()` explicitly at the end of each loop iteration instead of using `defer`.

### P0 — Ignored Error Can Cause Nil Pointer Panic

**File:** `internal/icons/cache.go`, lines 81, 132

```go
req, _ := http.NewRequestWithContext(context.Background(), "GET", selfhstAPIURL, nil)
```
The error from `NewRequestWithContext` is discarded. If the URL is invalid, `req` is `nil` and the next line (`req.Header.Set(...)`) will panic.

**Fix:** Check the error and return early on failure.

### P1 — No Authentication on API/Dashboard

**Files:** `cmd/server/main.go` lines 53-59, `internal/handlers/handlers.go`

There is **zero authentication** on any endpoint (`/`, `/api/services`, `/api/status`, `/static/`, `/icons/`). Anyone with network access to port 8080 can enumerate all internal services, their URLs, icons, tags, and groups. This exposes internal network topology.

**Recommendation:** Add authentication middleware or prominently document that TraLa **must** be placed behind an authenticating reverse proxy.

### P1 — SSRF via Favicon/HTML Icon Fetching

**File:** `internal/icons/icons.go`, lines 167-209

`FindFavicon()` and `FindHTMLIcon()` make outbound HTTP requests to service URLs derived from Traefik router rules. If an attacker registers a Traefik router with `Host(\`169.254.169.254\`)`, TraLa will request cloud metadata endpoints.

**Fix:** Add an IP allowlist/blocklist to block requests to private/link-local/loopback addresses.

### P1 — No HTTP Security Headers

**File:** `internal/handlers/handlers.go`

The following security headers are completely missing:
- `Content-Security-Policy` (CSP)
- `X-Frame-Options` / `frame-ancestors`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy`
- `Strict-Transport-Security` (HSTS)
- `Permissions-Policy`

**Fix:** Add a middleware that sets security headers on all responses.

### P1 — Credentials Leaked in Debug Logs

**File:** `internal/config/config.go`, lines 221-229

When `LOG_LEVEL=debug`, the entire configuration (including Traefik basic auth password) is marshaled to YAML and printed to stdout. This leaks credentials to container logs.

**Fix:** Redact sensitive fields before logging the configuration.

### P1 — Container Runs as Root

**File:** `dockerfile`

There is no `USER` directive. The container runs as root by default.

**Fix:** Add a non-root user:
```dockerfile
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
```

---

## 2. Backend Logic Review

### Error Handling Issues

| Issue | File | Line(s) |
|-------|------|---------|
| HTTP response status code not checked before JSON decoding | `cache.go` | 84-92, 135-143 |
| `json.NewEncoder(w).Encode()` error ignored | `handlers.go` | 184, 268 |
| Password file content not trimmed (trailing `\n` becomes part of password) | `config.go` | 195 |

### Code Quality

| Issue | File | Description |
|-------|------|-------------|
| Duplicated `debugf` function | `handlers.go:274`, `processor.go:221`, `traefik.go:294`, `cache.go:268` | Identical function in 4 packages; extract to shared utility |
| Duplicated icon URL construction | `icons.go:41-50`, `processor.go:118-124` | Same pattern in two places; violates DRY |
| Dead code | `processor.go:213-217` | Unreachable `return` after redundant length check |
| Redundant self-assignment | `grouping.go:29` | `services[i].Group = s.Group` assigns field to itself |
| Unused `htmlTemplate` bytes kept in memory | `handlers.go:58` | Raw bytes stored but never used after `parsedTemplate` is created |

### Concurrency Issues

| Issue | File | Description |
|-------|------|-------------|
| Race condition on `serviceOverrideMap` | `config.go:200` | Map written outside mutex lock, read with `RLock` |
| Config accessors return mutable references | `config.go:454,461,468,475` | `GetExcludeRouters()`, `GetServiceOverrideMap()` etc. return direct references to internal slices/maps; callers can corrupt shared state |
| `debugLogEnabled` not concurrency-safe | `cache.go:284-289` | Bool read/written without synchronization; should use `atomic.Bool` |
| Nested locks in `FindUserIcon` | `cache.go:238-249` | Acquires `userIconsMux.RLock()` then `sortedUserIconNamesMux.RLock()`; fragile if lock order changes |
| Pre-warm goroutine errors silently discarded | `main.go:48-50` | `go icons.GetSelfHstIconNames()` etc. return values never checked |

### Architecture Concerns

| Issue | File | Description |
|-------|------|-------------|
| `http.ResponseWriter` passed into non-handler code | `traefik.go` | Traefik client functions write HTTP errors directly; tight coupling to HTTP layer |
| Global mutable state throughout codebase | Multiple | Every package uses package-level globals; makes testing difficult |
| Hardcoded filesystem paths | `config.go:23`, `i18n.go:18`, `main.go:45-46,57-58` | Paths like `/config/configuration.yml`, `/app/template`, etc. prevent running outside Docker |

### Configuration Handling

| Issue | File | Description |
|-------|------|-------------|
| No validation of `LOG_LEVEL` | `config.go` | Invalid values like `"debg"` accepted silently |
| No validation of `LANGUAGE` | `config.go` | No check if translation file exists until `i18n.Init()` |
| No env var override for `enable_basic_auth` | `config.go` | Inconsistent with other basic auth settings that have env var overrides |
| Config parse failure is non-fatal | `config.go:82` | YAML syntax errors only log a warning; app runs with unintended defaults |
| Basic auth validation happens before env var overrides | `config.go:88` | Validation runs at Step 3, env overrides applied at Step 4 |

---

## 3. Security Review

### SSRF

- **Icon fetching** (`icons.go:167-209`): Outbound HTTP requests to URLs derived from Traefik router rules with no IP restrictions.
- **Recommendation:** Implement a custom `http.Transport` with a `DialContext` that blocks private/loopback/link-local IP ranges.

### Path Traversal

- **`LANGUAGE` env var** (`i18n.go:41`): Used directly in `filepath.Join(translationDir, lang+".yaml")`. While practical impact is limited (YAML parsed as i18n messages), input should be validated against a known set of languages.
- **`TRAEFIK_BASIC_AUTH_PASSWORD_FILE`** (`config.go:183-197`): Reads arbitrary files from filesystem with no path validation.

### TLS

- **`InsecureSkipVerify`** (`traefik.go:54`): When enabled, disables all TLS certificate verification on the Traefik API connection (which may carry basic auth credentials). Consider supporting custom CA certificates instead.

### Static File Serving

- **Directory listing enabled** (`main.go:57-58`): Go's `http.FileServer` serves directory listings by default on `/static/` and `/icons/`. Wrap the file server to disable directory listing.

### HTTP Server Configuration

- **No timeouts** (`main.go:63`): Bare `http.ListenAndServe` with no `ReadTimeout`, `WriteTimeout`, `ReadHeaderTimeout`, or `MaxHeaderBytes`. Vulnerable to slowloris-style DoS.
- **No request size limits**: Unbounded JSON decoding from Traefik API and selfh.st CDN (`traefik.go:179`, `cache.go:91,142`). Use `io.LimitReader`.

### External Resources

- **Google Fonts loaded without SRI** (`index.html:10-12`).
- **selfh.st icon CDN uses `http://`** in demo config — no integrity verification.
- **GitHub Actions not pinned by SHA** (`publish.yml`) — supply-chain risk via tag rewriting.

### Positive Findings

- No hardcoded credentials, API keys, or tokens found in source code.
- `rel="noopener noreferrer"` used on external links in HTML.
- `escapeHtml()` function used consistently in JavaScript for innerHTML.
- Docker socket mounted read-only (`:ro`) in demo docker-compose.

---

## 4. Frontend Review

### JavaScript (`web/js/trala.js`)

**XSS Protection:**
- `escapeHtml()` function (line 18) is defined and used consistently in `createServiceCard()` and search results. No active XSS vulnerabilities found in JavaScript.

**Memory & Performance:**
| Issue | Line(s) | Description |
|-------|---------|-------------|
| Full DOM rebuild every 30 seconds | 154-163, 166-203 | `innerHTML = ''` followed by element-by-element append; use `DocumentFragment` or diff |
| `img.onerror` closures on detached DOM | 141, 352 | Old callbacks may fire on already-removed elements during re-render |
| Clock updates every 6 seconds | 398-401 | Only shows hours:minutes; 60s interval would suffice |
| No `AbortController` for API requests | 236 | Overlapping requests possible if API is slow |
| `localStorage.setItem` not in try/catch | 293 | Throws `QuotaExceededError` in Safari private browsing |

**UX Issues:**
| Issue | Description |
|-------|-------------|
| Group collapse state lost on refresh | Every 30s re-render rebuilds DOM; user's collapsed groups re-expand |
| Error replaces good data | On refresh failure, `showErrorPage` hides the service grid entirely instead of showing a toast |
| No empty state message | When API returns zero services (not from search), the grid is empty with no explanation |
| No skeleton/placeholder UI | First load shows empty grid until API responds |
| Search web-launch not discoverable | Pressing Enter opens search engine, but there's no visual hint |
| No lazy loading on service icons | All icons load eagerly; `loading="lazy"` would help with 50+ services |

### Accessibility

**Critical accessibility issues:**
| Issue | Location | Description |
|-------|----------|-------------|
| Group headers not keyboard accessible | JS lines 184-191 | `<h2>` elements with click handlers but no `role="button"`, not focusable via Tab, no Enter/Space support |
| No ARIA on expand/collapse | JS lines 184-191, HTML line 63 | No `aria-expanded`, `aria-controls` attributes |
| Search input has no label | HTML line 41 | Only `placeholder`, no `<label>` or `aria-label` |
| GitHub footer link has no accessible text | HTML line 82-86 | SVG with `aria-hidden="true"`, no text alternative |
| Sort buttons lack `aria-pressed`/`aria-current` | HTML lines 57-59 | Only visual CSS class indicates active sort |
| Loading/error states not announced | HTML lines 23, 66 | No `aria-live` region or `role="alert"` |
| No skip-to-content link | HTML | No mechanism to skip header for keyboard users |
| Config warning has no accessible text | HTML lines 76-80 | Only `title` attribute on SVG parent |

### HTML & CSS

- Inconsistent display toggling: `style="display:none"` vs Tailwind `hidden` class used interchangeably.
- Hardcoded hex colors in `trala.css` (lines 18, 37, 44, 66) instead of Tailwind theme tokens.
- Fixed footer can overlap content on small screens with no bottom padding compensation.

---

## 5. CI/CD & Docker Review

### Dockerfile

| Severity | Issue | Description |
|----------|-------|-------------|
| Critical | No non-root user | Container runs as root; add `USER` directive |
| Critical | No `.dockerignore` | Entire repo (including `.git/`) sent as build context |
| High | Go module cache not optimized | `go.mod`/`go.sum` should be copied before source for layer caching |
| Medium | `build-base` unnecessary | `CGO_ENABLED=0` means C compiler is not needed |
| Medium | `curl` for healthcheck | Increases attack surface; use `wget` (already in Alpine) or built-in binary |
| Medium | `-a -installsuffix cgo` flags | Legacy pattern unnecessary with modern Go and `CGO_ENABLED=0` |
| Low | COPY flattens directory structure | Line 7 copies files from 3 dirs into one flat directory; fragile |
| Low | `VOLUME` declarations | Create anonymous volumes unexpectedly; better in docker-compose |
| Low | Filename should be `Dockerfile` | Convention is capital D; some tools may not find `dockerfile` |

### CI/CD Workflows

| Severity | Issue | File | Description |
|----------|-------|------|-------------|
| High | `latest` tag never applied | `publish.yml:52` | `is_default_branch` evaluates on tag-only workflow; always false |
| High | No build cache | `publish.yml` | Multi-platform builds rebuild from scratch every time |
| High | Actions not pinned by SHA | `publish.yml` | Supply-chain risk via tag rewriting |
| Medium | Hardcoded Docker Hub username | `publish.yml:15` | Breaks on fork or username change |
| Low | No concurrency control | `website.yml` | Overlapping Pages deployments possible |

### Docker Compose (Demo)

| Severity | Issue | Description |
|----------|-------|-------------|
| High | `--api.insecure=true` | Traefik API exposed without authentication |
| Medium | `traefik:latest` | Non-reproducible; pin to specific version |
| Medium | Port 8080 exposed to all interfaces | Should be `127.0.0.1:8080:8080` for Traefik dashboard |
| Low | No `depends_on` for trala | Startup order not guaranteed |

---

## 6. Configuration & Translation Review

### Go Module

- **`go.yaml.in/yaml/v4 v4.0.0-rc.4`** is a release candidate — breaking changes possible before stable release.
- **Module name `server`** (bare name) — convention is full import path like `github.com/dannybouwers/trala`.
- **`go.sum` has stale entries** — run `go mod tidy` to clean up.

### Translations

All 16 keys (`title`, `error`, `fetch_error`, `search`, `clear_search`, `search_placeholder`, `name`, `url`, `priority`, `greeting_night`, `greeting_morning`, `greeting_afternoon`, `greeting_evening`, `grouping`, `expand_collapse_all`, `uncategorized`) are present and consistent across all three languages (en, de, nl). No missing keys found.

### .gitignore

Missing entries for:
- Built Go binaries
- `node_modules/`

---

## 7. Summary Matrix

| # | Severity | Category | Issue | File(s) |
|---|----------|----------|-------|---------|
| 1 | **P0** | Security | `text/template` instead of `html/template` (XSS) | `handlers.go:16` |
| 2 | **P0** | Bug | URL construction: path before port | `traefik.go:288` |
| 3 | **P0** | Bug | `defer cancel()` in loop leaks contexts | `traefik.go:154` |
| 4 | **P0** | Bug | Ignored error causes nil pointer panic | `cache.go:81,132` |
| 5 | **P1** | Security | No authentication on any endpoint | `main.go:53-59` |
| 6 | **P1** | Security | SSRF via icon fetching | `icons.go:167-209` |
| 7 | **P1** | Security | No HTTP security headers | `handlers.go` |
| 8 | **P1** | Security | Credentials leaked in debug logs | `config.go:221-229` |
| 9 | **P1** | Security | Container runs as root | `dockerfile` |
| 10 | **P1** | CI/CD | `latest` tag never applied in publish workflow | `publish.yml:52` |
| 11 | **P1** | CI/CD | No build cache for multi-platform Docker builds | `publish.yml` |
| 12 | **P2** | Concurrency | Race condition on `serviceOverrideMap` | `config.go:200` |
| 13 | **P2** | Concurrency | Config accessors return mutable references | `config.go:454-475` |
| 14 | **P2** | Security | No HTTP server timeouts (slowloris) | `main.go:63` |
| 15 | **P2** | Security | Directory listing enabled on `/icons/` and `/static/` | `main.go:57-58` |
| 16 | **P2** | Bug | Password file not trimmed of trailing newline | `config.go:195` |
| 17 | **P2** | Accessibility | Group headers not keyboard accessible, no ARIA | JS `184-191` |
| 18 | **P2** | UX | Group collapse state lost every 30s refresh | JS `166-203` |
| 19 | **P2** | Docker | No `.dockerignore` file | Project root |
| 20 | **P2** | Docker | Go module cache not optimized | `dockerfile` |
| 21 | **P3** | Quality | Duplicated `debugf` in 4 packages | Multiple |
| 22 | **P3** | Quality | Dead code in `ExtractServiceNameFromURL` | `processor.go:213-217` |
| 23 | **P3** | Config | No validation of `LOG_LEVEL` or `LANGUAGE` | `config.go` |
| 24 | **P3** | Deps | YAML v4 dependency is a release candidate | `go.mod` |
| 25 | **P3** | Deps | Module name is bare `server` | `go.mod:1` |
| 26 | **P3** | Frontend | `localStorage` not in try/catch | `trala.js:293` |
| 27 | **P3** | Frontend | Clock updates every 6s but only shows HH:MM | `trala.js:398` |
| 28 | **P3** | CI/CD | Actions not pinned by SHA | `publish.yml` |
| 29 | **P3** | CSS | Hardcoded hex values instead of Tailwind tokens | `trala.css` |
| 30 | **P3** | Docker | `build-base` installed but unused with CGO_ENABLED=0 | `dockerfile` |

---

*Review conducted using automated multi-agent analysis covering backend logic, security, frontend, and infrastructure.*
