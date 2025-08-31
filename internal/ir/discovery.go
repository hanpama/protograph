package ir

import (
	"context"
)

type ServiceMetadata struct {
	ID       ServiceID
	Name     string
	PkgPath  []string
	FilePath string
}

type Discovery interface {
	ListMetadata(ctx context.Context) ([]*ServiceMetadata, error)
	ReadServiceSDL(ctx context.Context, id ServiceID) (string, error)
}
