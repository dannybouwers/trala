# Contributing to TraLa

Thank you for your interest in contributing to TraLa! Whether you've found a bug, have a feature idea, or want to help improve the documentation—your contributions are valued and welcome.

## Quick Start

The contribution process is straightforward:

1. **Fork** the repository
2. **Create** a branch for your changes
3. **Open** a pull request
4. **Wait** for review—kilo-code-bot will provide automated feedback, and a human reviewer will make the final decision

That's it! Every pull request gets personal attention from a maintainer.

## What We're Looking For

Contributions come in many forms:

- 🐛 **Bug fixes** — Found something broken? Let us know or fix it yourself
- ✨ **New features** — Have an idea for improving TraLa?
- 📖 **Documentation** — Help make the docs clearer or more complete
- 🎨 **UI/UX improvements** — Make the dashboard more usable
- 🔧 **Tests** — Verify things work as expected
- 🌍 **Translations** — Add or improve language support

Don't worry if you're new to open source—this project is a great place to start. We appreciate the effort, and even small contributions make a difference.

## AI-Assisted Coding

TraLa itself was built with AI assistance, and we're open to contributions created with AI tools. If you're using AI to help write code, that's perfectly fine—just make sure you understand what the code does before submitting.

**A note on PR size**: To help reviewers (both automated and human) understand and validate changes effectively, we encourage breaking large changes into smaller, focused pull requests. A PR that addresses a single concern (one feature, one bug fix, one type of improvement) is easier to review and test than one that mixes many unrelated changes. This applies whether you wrote the code yourself or with AI assistance.

## Pull Request Guidelines

When submitting a pull request, consider:

- **Keep it focused** — One PR per concern (e.g., security fixes, frontend improvements, documentation updates)
- **Make it reviewable** — Aim for changes that can be reviewed in a single session
- **Test your changes** — Verify the application works as expected (see Testing section below)
- **Describe what you changed** — A clear summary helps reviewers understand your intent

If your PR touches many files or addresses unrelated concerns, consider splitting it into smaller, incremental PRs. This makes it easier for reviewers to provide meaningful feedback and for you to get your contributions merged faster.

## Code Review Process

All pull requests go through two levels of review:

1. **kilo-code-bot** — An automated review that checks for best practices, consistency, functional comments, project alignment, automation opportunities, and appropriate PR scope. This helps catch issues early.

2. **Human review** — A maintainer will personally review your PR, provide feedback, and make the final decision on merging.

The automated review is never the final word—every PR is accepted or declined by a human. We're happy to work with contributors to get their changes merged.

### Review Criteria

kilo-code-bot evaluates PRs against these priorities:

- **Best Practices** — Code follows industry standards for the language/framework
- **Consistency** — Style, naming, and structure match the codebase
- **Functional Comments** — Clear explanations for complex logic
- **Project Alignment** — Features match TraLa's goals
- **Automation First** — Prefer automated solutions over manual processes
- **PR Size & Scope** — Focused on a single concern, reviewable in one session

## Testing

TraLa has two independent components—changes to one don't require testing the other.

### Application Testing

The application is tested using the demo stack with Docker Compose:

```bash
# Build the application
cd demo && docker compose build

# Start the testing stack
cd demo && docker compose up -d

# Verify the application runs correctly
# Access the dashboard at https://trala.localhost

# Stop when finished
cd demo && docker compose down
```

For custom test configurations, create a `./local/` directory and add your own `docker-compose.yml` and `configuration.yml` files there.

### Website Testing

The documentation website (in `website/`) is an Astro project:

```bash
# Install dependencies
cd website && npm install

# Build the website
cd website && npm run build

# Start development server
cd website && npm run dev

# Preview the built site
cd website && npm run preview
```

For complete building and testing instructions, see the [Development Guide](docs/development.md).

## Development Setup

To set up your local development environment:

1. Install prerequisites: Go 1.21+, Node.js 25+, Docker, Docker Compose
2. Clone your fork
3. Follow the [Development Guide](docs/development.md) for build instructions

The demo stack provides a working example with mock services to test against.

## Reporting Issues

Found a bug or have a feature request? Open an issue! Use the provided templates when available and include:

- A clear description of the issue or feature
- Steps to reproduce (for bugs)
- Expected vs. actual behavior
- Your environment details (OS, browser, TraLa version)

The more information you provide, the faster we can understand and address your input.

## Getting Help

- **Discussions** — Ask questions or share ideas in GitHub Discussions
- **Issues** — Report bugs or request features
- **Docs** — Check [trala.fyi](https://www.trala.fyi) for full documentation

We're here to help and appreciate your interest in making TraLa better. Looking forward to your contributions!