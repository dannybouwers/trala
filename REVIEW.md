# REVIEW.md

Guidance for Kilo's automated Code Reviews on the TraLa repository. TraLa is a
Go backend with an embedded web dashboard and an independent Astro documentation
site (`website/`). Reviews should be **positive, helpful, and focused on the
big picture** — encourage contributors rather than overwhelming them with
nitpicks.

## Review priorities

Focus on what matters most, in order:

1. **Correctness and safety** — Does it work, and is it secure? Watch for
   secrets, unvalidated input, and unsafe Traefik API handling. See the
   [Security checklist](#security-checklist) for the specific concerns this
   project cares about (Traefik API auth, TLS, secret handling, URL
   reconstruction).
2. **Best Practices** — Does the code follow industry best practices for the
   language and framework in use (Go, Astro, etc.)? Flag shortcuts that trade
   clarity, safety, or maintainability for speed.
3. **Functional Comments** — Are complex or non-obvious decisions explained with
   clear, concise comments? Flag logic that is hard to follow without context.
4. **Testability** — Can the change be verified with the `go test ./...` suite,
   the demo stack, or the website build? Encourage testing, don't demand it for
   docs-only changes.
5. **Project alignment** — Does the change fit TraLa's goals (auto-discovery,
   icon detection, smart grouping, light/dark, multi-language, multi-arch)?
6. **Automation First** — Prefer solutions that automate tasks by default, with
   manual overrides only where absolutely necessary. Highlight opportunities to
   replace manual processes with automation.
7. **Focused scope** — One PR should address one concern. Flag PRs that mix
   unrelated changes and suggest splitting them. Large PRs (more than ~10 files
   or touching unrelated concerns) should be split into focused, incremental
   PRs that are reviewable in a single session and allow for incremental testing.
8. **Consistency** — Style, naming, and structure should match the existing
   Go and web code.

## Security checklist

Apply these checks, in order, to every PR that touches the backend or
configuration:

- **Traefik API authentication** — Basic auth credentials must never be
  hardcoded. They come from the config file, a password file, or environment
  variables. Flag any path that bakes a username or password into source.
- **TLS and certificate verification** — `InsecureSkipVerify` is a deliberate,
  opt-in escape hatch. If a PR widens its use beyond what the config already
  allows, flag it. Default to verified TLS.
- **Secret handling and redaction** — Basic auth passwords are sensitive. They
  must be redacted in logs and debug output (the config package already does
  this for the effective-config dump). Flag any new code path that could print
  credentials, password-file contents, or env-var secrets.
- **URL and rule reconstruction** — Service URLs are rebuilt from Traefik
  router rules via regex. Flag any assumption that a rule always contains a
  `Host(...)` clause, and any place where a reconstructed URL is used without
  validation.
- **Input validation** — Values that leave the trusted boundary (config values,
  language codes, API host) should be validated before use. The i18n package
  already guards against path traversal on language codes; follow that pattern.
- **Config parsing** — Configuration is YAML plus env overrides. Flag unvalidated
  env values parsed into booleans, numbers, or URLs without fallbacks, and any
  place that could leak one instance's credentials into another.

## Review output format

Every review ends with a short verdict and, where applicable, a list of
findings. Use this structure:

```
## Review verdict

<One or two sentences: the final recommendation — approve, request changes, or
comment — and the single most important reason.>

## How this PR relates to the review criteria

<Short overview mapping the PR to the priorities above. Mention which criteria
were checked and which are not applicable (e.g. docs-only).>

## Findings

### <Severity — Critical / Important / Minor>

- **<file>:<line>** — <concise description of the issue>
  <Why it matters, in one or two sentences. If it's a suggestion, say what the
  better approach is.>
```

Rules for findings:

- One finding per bullet; lead with `path:line`.
- Keep the explanation short and concrete. No nitpicks — if it's cosmetic, it
  belongs in the skip list.
- Every finding maps to a priority above; say which one if it isn't obvious.
- End with the verdict. No open questions, no "what do you think?".

## AI-assisted contributions

TraLa welcomes AI-assisted PRs. The PR template asks contributors to self-report
how AI was used; honour that and calibrate feedback accordingly:

- **🧑‍💻 Fully manual** — Review like any other PR.
- **💡 AI as a helper** — Slightly deeper checks on generated snippets; assume
  the contributor understands the code.
- **🤝 Collaborative** — Focus on correctness and alignment; trust the
  contributor's understanding.
- **🤖 Mostly AI-written** — Verify the code actually compiles and runs; treat
  explanations as teaching moments.
- **❓ Entirely AI-generated** — Check for hallucinated APIs, fabricated config
  keys, or Traefik endpoints that don't exist. Be extra gentle: explain findings
  as guidance, not criticism.

Never assume a contributor is careless because AI wrote the code. The goal is to
help them learn the codebase.

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

Size the review team to the PR. Sub-agents stay read-only, never post comments,
and return findings with path, line, severity, and rationale. The main reviewer
verifies every finding before posting.

- Use **0 sub-agents** for docs-only, translation, formatting, or single-file
  typo/config changes.
- Use **1 sub-agent** for focused changes under ~300 lines touching one risky
  area (Traefik client, config parsing, icon detection, handlers, i18n).
- Use **2 sub-agents** when a PR spans two areas, e.g. backend plus frontend.
- Use **up to 6 sub-agents** for large cross-cutting or security-sensitive
  changes above ~800 lines. Assign one reviewer per domain:

  1. **Backend reviewer** — Traefik client, config, services, handlers, security.
  2. **Frontend reviewer** — dashboard UI, accessibility, empty/error states, theme.
  3. **Test/docs reviewer** — demo-stack verification, docs accuracy.
  4. **Security reviewer** — Traefik API auth, TLS, secret handling, URL
     reconstruction, input validation (see the security checklist above).
  5. **i18n reviewer** — translation files, language fallback, localized strings,
     Weblate sync.
  6. **Build/CI reviewer** — `go test ./...`, Docker build, website build,
     workflow changes, dependency updates.
