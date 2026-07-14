package providers

import "context"

// Service represents a discovered service from a Traefik provider.
type Service struct {
	Name     string
	URL      string
	Priority int
	Icon     string
	Tags     []string
}

// Provider defines the interface for fetching services from a Traefik instance.
type Provider interface {
	FetchServices(ctx context.Context) ([]Service, error)
}
