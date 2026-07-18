# REVIEW.md

Guidance for Kilo's automated Code Reviews on the TraLa repository. TraLa is a
Go backend with an embedded web dashboard and an independent Astro documentation
site (`website/`). Reviews should be **positive, helpful, and focused on the
big picture** — encourage contributors rather than overwhelming them with
nitpicks.

## Review priorities

Focus on what matters most, in order:

1. **Correctness and safety** — Does it work, and is it secure? Watch for
   secrets, unvalidated input, and unsafe Traefik API handling.
2. **Project alignment** — Does the change fit TraLa's goals (auto-discovery,
   icon detection, smart grouping, light/dark, multi-language, multi-arch)?
3. **Focused scope** — One PR should address one concern. Flag PRs that mix
   unrelated changes and suggest splitting them.
4. **Consistency** — Style, naming, and structure should match the existing
   Go and web code.
5. **Testability** — Can the change be verified with the demo stack or the
   website build? Encourage testing, don't demand it for docs-only changes.

## What to skip

- Formatting and cosmetic style nits (use `gofmt`/linters; don't flag manually).
- Translation files and dependency-lockfile-only changes (usually Renovate).
- Trivial doc typo fixes.

## Severity calibration

- **Critical** — Security issues, data loss, broken builds, or broken dashboard.
- **Important** — Bugs, missing tests for real behavior, scope creep.
- **Minor / suggestion** — Nice-to-haves, optional clarity improvements.

Keep the tone welcoming. This project welcomes AI-assisted contributions; when a
PR was largely AI-generated, explain findings as teaching moments rather than
criticism.

## Sub-agent usage

- Use **0 sub-agents** for docs-only, translation, formatting, or single-file
  typo/config changes.
- Use **1 sub-agent** for focused changes under ~300 lines touching one risky
  area (Traefik client, config parsing, icon detection, handlers, i18n).
- Use **up to 3 sub-agents** when a PR spans the backend API, the web dashboard,
  and the data model:
  1. **Backend reviewer** — Traefik client, config, services, handlers, security.
  2. **Frontend reviewer** — dashboard UI, accessibility, empty/error states, theme.
  3. **Test/docs reviewer** — demo-stack verification, docs accuracy.
- Use the **full 6 sub-agents** only for large cross-cutting or security-sensitive
  changes above ~800 lines.

All sub-agents stay read-only, never post comments, and return findings with
path, line, severity, and rationale. The main reviewer verifies every finding
before posting.
