# Customization

TraLa ships with a fixed look, but you can restyle it and hide individual interface elements by mounting your own stylesheet. This page explains how to load custom CSS and gives ready-to-use snippets, including how to hide the search bar, sort controls, greeting, and footer (see [hide interface elements](#hiding-interface-elements)).

## Custom CSS

TraLa loads an optional stylesheet from a fixed path in the container:

- **Container path:** `/config/custom.css`
- **Served URL:** `/custom.css`

The `/config` directory is the same one used for `configuration.yml`, so you can mount a single file or the whole directory.

### Mounting the stylesheet

```yaml
# docker-compose.yml
services:
  trala:
    volumes:
      # Option A: mount a single file
      - ./custom.css:/config/custom.css:ro

      # Option B: mount the whole config directory (recommended)
      # - ./config:/config:ro
```

Option B keeps `configuration.yml` and `custom.css` in one folder, so you have a single volume to mount and back up. It also avoids a common gotcha with single-file bind mounts: editors that save atomically (write a temp file, then rename) replace the file's inode, and the container keeps seeing the old file until it is recreated.

### How it works

1. If `/config/custom.css` exists at request time, TraLa adds `<link rel="stylesheet" href="/custom.css">` to the page **after** the built-in `tailwind.css` and `trala.css`, so your rules win where they overlap.
2. If the file is absent, no link is emitted — there is no 404 in the browser console.
3. The file is revalidated on every page load, so **edits take effect on reload without restarting the container**. Hard-refresh your browser if a change does not appear.
4. The file must be readable by the container's non-root user. A host file that is only readable by root (for example `0600` owned by root) will be ignored.

> [!NOTE]
> Only this exact file is served. The `/config` directory itself is never exposed over HTTP because it can contain secrets such as `configuration.yml` and basic-auth password files.

### Security headers (Content-Security-Policy)

TraLa sends a Content-Security-Policy that allows stylesheets from the same origin (`'self'`) plus inline styles. This means:

- A stylesheet served from `/custom.css` works out of the box.
- `@import` of external stylesheets and `@font-face` fonts from other domains are blocked. Fonts are only allowed from `fonts.gstatic.com`; images referenced with `url()` are allowed from `https:` and `data:`.
- Custom CSS is cosmetic only. Hidden elements are still present in the HTML, so this is not a security or access-control feature.

## Hiding interface elements

Every visible block has a stable `id` or an easy selector, so you can hide exactly the elements you do not need. This is useful for kiosk displays, small screens, or a minimalist dashboard.

### Element reference

| Element | Selector | Notes |
|---------|----------|-------|
| Greeting and clock | `header` | The heading with the time-based greeting |
| Logo (top-left) | `body > div.absolute.top-0.left-0` | The TraLa logo and wordmark |
| Search bar | `div:has(> #search-form)` | Hides both the input and its spacing |
| Sort controls | `#sort-controls` | Name / URL / Priority buttons |
| Group controls | `#group-controls` | Grouping / expand-collapse buttons (needs `!important`) |
| Footer | `footer` | Version number and GitHub link |

> [!NOTE]
> Some elements — notably `#group-controls` — get their `display` set inline by JavaScript, which beats a plain ID selector. Use `!important` for those.

### Example: kiosk / minimal mode

Hide the search bar, sort controls, greeting, and footer:

```css
/* /config/custom.css */

/* Search bar (and the spacing around it) */
div:has(> #search-form) {
  display: none;
}

/* Sort buttons: Name / URL / Priority */
#sort-controls {
  display: none;
}

/* Grouping / expand-collapse buttons (set inline by JS, so !important) */
#group-controls {
  display: none !important;
}

/* Greeting and clock */
header {
  display: none;
}

/* Version number and GitHub link */
footer {
  display: none;
}
```

### Example: hide only the greeting

```css
header {
  display: none;
}
```

### Example: hide the logo

```css
body > div.absolute.top-0.left-0 {
  display: none;
}
```

## Theming

Because your stylesheet loads last, you can override the built-in Tailwind utilities with your own selectors. For example, to change the accent colour of the logo:

```css
.logo-font {
  color: #8b5cf6;
}
```

Use `!important` only when overriding inline styles or utility classes that you cannot beat with selector specificity.
