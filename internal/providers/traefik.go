package providers

import (
	"context"
	"net/http"

	"server/internal/config"
	"server/internal/models"
	"server/internal/services"
	"server/internal/traefik"
)

// TraefikProvider fetches services from a single Traefik instance.
type TraefikProvider struct {
	Instance   config.TraefikInstanceConfig
	HTTPClient *http.Client
}

// NewTraefikProvider creates a new TraefikProvider for the given instance.
func NewTraefikProvider(instance config.TraefikInstanceConfig) *TraefikProvider {
	return &TraefikProvider{
		Instance:   instance,
		HTTPClient: traefik.CreateHTTPClientForInstance(instance.InsecureSkipVerify),
	}
}

// FetchServices retrieves all services from the Traefik instance.
func (p *TraefikProvider) FetchServices(ctx context.Context) ([]Service, error) {
	entryPoints, err := traefik.FetchAllPagesWithInstanceAuth[models.TraefikEntryPoint](ctx, p.HTTPClient, p.Instance.APIHost+"/api/entrypoints", p.Instance)
	if err != nil {
		return nil, err
	}

	routers, err := traefik.FetchAllPagesWithInstanceAuth[models.TraefikRouter](ctx, p.HTTPClient, p.Instance.APIHost+"/api/http/routers", p.Instance)
	if err != nil {
		return nil, err
	}

	entryPointsMap := make(map[string]models.TraefikEntryPoint, len(entryPoints))
	for _, ep := range entryPoints {
		entryPointsMap[ep.Name] = ep
	}

	var result []Service
	for _, router := range routers {
		svc, ok := services.ProcessRouter(router, entryPointsMap, p.Instance.Name)
		if ok {
			result = append(result, Service{
				Name:     svc.Name,
				URL:      svc.URL,
				Priority: svc.Priority,
				Icon:     svc.Icon,
				Tags:     svc.Tags,
			})
		}
	}

	return result, nil
}
