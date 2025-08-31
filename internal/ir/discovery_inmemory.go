package ir

import (
	"context"
	"fmt"
	"strings"
)

type InMemoryService struct {
	// dot-separated package path
	Package string
	Name    string
	Content string
}

// InMemoryDiscovery is a test implementation of Discovery that stores data in memory
type InMemoryDiscovery struct {
	services map[string]*ServiceMetadata
	contents map[ServiceID]string
}

// NewInMemoryDiscovery creates a new InMemoryDiscovery instance
func NewInMemoryDiscovery(svcs []InMemoryService) *InMemoryDiscovery {

	discovery := &InMemoryDiscovery{
		services: make(map[string]*ServiceMetadata),
		contents: make(map[ServiceID]string),
	}

	for _, svc := range svcs {
		pkgPath := strings.Split(svc.Package, ".")
		filePath := strings.Join(pkgPath, "/") + "/" + svc.Name + ".graphql"
		discovery.services[svc.Name] = &ServiceMetadata{
			ID:       ServiceID(svc.Name),
			Name:     svc.Name,
			PkgPath:  pkgPath,
			FilePath: filePath,
		}
		discovery.contents[ServiceID(svc.Name)] = svc.Content
	}
	return discovery
}

// ListMetadata implements Discovery interface
func (d *InMemoryDiscovery) ListMetadata(ctx context.Context) ([]*ServiceMetadata, error) {
	pkgs := make([]*ServiceMetadata, 0, len(d.services))
	for _, pkg := range d.services {
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// ReadServiceSDL implements Discovery interface
func (d *InMemoryDiscovery) ReadServiceSDL(ctx context.Context, serviceID ServiceID) (string, error) {
	content, exists := d.contents[serviceID]
	if !exists {
		return "", fmt.Errorf("service %q not found", serviceID)
	}
	return content, nil
}
