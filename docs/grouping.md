# Grouping

TraLa's smart grouping feature automatically organizes services into collapsible groups based on tags from the selfh.st icon metadata.

## Overview

When enabled, TraLa analyzes service tags and groups services that share common tags. Groups can be collapsed or expanded individually via the dashboard toggle.

## Configuration Options

### Enable/Disable

```yaml
# configuration.yml
environment:
  grouping:
    enabled: true  # Default: true
```

Set via environment variable: `GROUPING_ENABLED=true`

### Column Count

Control the number of columns displayed on extra-large screens (xl, 1280px+):

```yaml
# configuration.yml
environment:
  grouping:
    columns: 3  # Range: 1-6, Default: 3
```

The grouped view always shows:
- 1 column on mobile
- 2 columns on tablets
- Configured number on xl screens

Set via environment variable: `GROUPED_COLUMNS=3`

### Tag Frequency Threshold

Exclude tags that appear in too many services (prevents overly broad groups):

```yaml
# configuration.yml
environment:
  grouping:
    tag_frequency_threshold: 0.9  # Default: 0.9
```

A threshold of 0.9 excludes tags found in more than 90% of services.

Set via environment variable: `GROUPING_TAG_FREQUENCY_THRESHOLD=0.9`

### Minimum Services per Group

Set the minimum number of services required to form a group:

```yaml
# configuration.yml
environment:
  grouping:
    min_services_per_group: 2  # Default: 2
```

Set via environment variable: `GROUPING_MIN_SERVICES_PER_GROUP=2`

## Manual Group Assignment

Override automatic grouping by manually assigning services to groups. See [Services](/docs/services) for details.

## Environment Variables Summary

| Variable | Description | Default |
|----------|-------------|---------|
| `GROUPING_ENABLED` | Enable/disable grouping | `true` |
| `GROUPED_COLUMNS` | Columns on xl screens (1-6) | `3` |
| `GROUPING_TAG_FREQUENCY_THRESHOLD` | Tag frequency threshold (0.0-1.0) | `0.9` |
| `GROUPING_MIN_SERVICES_PER_GROUP` | Minimum services per group | `2` |
