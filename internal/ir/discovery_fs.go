package ir

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemDiscovery implements Discovery for filesystem-based GraphQL schemas
type FileSystemDiscovery struct {
	svcFilePaths map[string]string
	svcMetas     map[ServiceID]*ServiceMetadata
}

// NewFileSystemDiscovery creates a new FileSystemDiscovery for the given root directory
func NewFileSystemDiscovery(ctx context.Context, rootDir string, rootPackage string) (*FileSystemDiscovery, error) {
	if rootPackage == "" {
		return nil, fmt.Errorf("root package cannot be empty")
	}
	discovery := &FileSystemDiscovery{
		svcFilePaths: make(map[string]string),
		svcMetas:     make(map[ServiceID]*ServiceMetadata),
	}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) != ".graphql" {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %q: %w", path, err)
		}

		pkgPath := filepath.Dir(relPath)
		pkgParts := strings.Split(rootPackage, ".")
		if pkgPath != "." {
			pkgParts = append(pkgParts, filepath.SplitList(pkgPath)...)
		}

		svcName := strings.TrimSuffix(d.Name(), ".graphql")
		svcID := ServiceID(svcName)

		discovery.svcFilePaths[string(svcID)] = path
		discovery.svcMetas[svcID] = &ServiceMetadata{
			ID:       svcID,
			Name:     svcName,
			PkgPath:  pkgParts,
			FilePath: relPath,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk root directory %q: %w", rootDir, err)
	}
	return discovery, nil
}

// ListPackages returns the list of packages discovered in the filesystem
func (d *FileSystemDiscovery) ListMetadata(ctx context.Context) ([]*ServiceMetadata, error) {
	pkgs := make([]*ServiceMetadata, 0, len(d.svcMetas))
	for _, pkg := range d.svcMetas {
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// ReadServiceSDL reads the GraphQL SDL content for a given service
func (d *FileSystemDiscovery) ReadServiceSDL(ctx context.Context, serviceID ServiceID) (string, error) {
	fp, ok := d.svcFilePaths[string(serviceID)]
	if !ok {
		return "", fmt.Errorf("service %q not found", serviceID)
	}
	content, err := os.ReadFile(fp)
	if err != nil {
		return "", fmt.Errorf("failed to read service SDL for %q: %w", serviceID, err)
	}
	return string(content), nil
}

// Load is a convenience function that creates a FileSystemDiscovery and builds the project
func Load(rootDir string, rootPackage string) (*Project, error) {
	discovery, err := NewFileSystemDiscovery(context.Background(), rootDir, rootPackage)
	if err != nil {
		return nil, err
	}
	return Build(context.Background(), discovery)
}
