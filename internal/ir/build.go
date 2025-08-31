package ir

import (
	"context"

	language "github.com/hanpama/protograph/internal/language"
)

type builder struct {
	Services    map[ServiceID]*Service
	Schema      *Schema
	Definitions map[string]*Definition
	Directives  map[string]*DirectiveDefinition
	Loaders     map[LoaderID]*LoaderDefinition
	Resolvers   map[ResolverID]*ResolverDefinition

	fields      map[[2]string]*FieldDefinition
	violations  []*Violation
	discovery   Discovery
	serviceDocs map[ServiceID]*language.SchemaDocument
}

func Build(ctx context.Context, disc Discovery) (*Project, error) {
	b := &builder{
		Services:    make(map[ServiceID]*Service),
		Schema:      nil,
		Definitions: make(map[string]*Definition),
		Directives:  make(map[string]*DirectiveDefinition),
		Loaders:     make(map[LoaderID]*LoaderDefinition),
		Resolvers:   make(map[ResolverID]*ResolverDefinition),
		fields:      make(map[[2]string]*FieldDefinition),
		violations:  nil,
		discovery:   disc,
		serviceDocs: make(map[ServiceID]*language.SchemaDocument),
	}

	if err := b.build(ctx); err != nil {
		return nil, err
	}

	return &Project{
		// Packages:    b.Packages,
		Services:    b.Services,
		Schema:      b.Schema,
		Definitions: b.Definitions,
		Directives:  b.Directives,
		Loaders:     b.Loaders,
		Resolvers:   b.Resolvers,
	}, nil
}

func (b *builder) build(ctx context.Context) (err error) {
	svcs, err := b.discovery.ListMetadata(ctx)
	if err != nil {
		return err
	}

	for _, sm := range svcs {
		s := &Service{
			ID:          sm.ID,
			Name:        sm.Name,
			PackagePath: sm.PkgPath,
			FilePath:    sm.FilePath,
			Definitions: nil,
			Directives:  nil,
		}
		b.Services[s.ID] = s
	}

	// Parse service SDL files
	for svcId, svcMeta := range b.Services {
		sdl, err := b.discovery.ReadServiceSDL(ctx, svcId)
		if err != nil {
			return err
		}
		document, err := language.ParseSchema(svcMeta.FilePath, sdl)
		if err != nil {
			return err
		}
		b.serviceDocs[svcId] = document
	}

	// Load built-in scalars
	b.Definitions["String"] = &Definition{Scalar: StringType}
	b.Definitions["Int"] = &Definition{Scalar: IntType}
	b.Definitions["Float"] = &Definition{Scalar: FloatType}
	b.Definitions["Boolean"] = &Definition{Scalar: BooleanType}
	b.Definitions["ID"] = &Definition{Scalar: IDType}

	// Populate definitions
	if err = b.populateDefinitions(); err != nil {
		return err
	}

	// Process schema definitions
	if err = b.processSchemaDefinitions(); err != nil {
		return err
	}

	// Populate references including fields, input values and union types
	if err = b.populateReferences(); err != nil {
		return err
	}
	// Populate interface implementations and union members
	if err = b.populateImplementations(); err != nil {
		return err
	}

	// Populate directives
	if err = b.populateDirectiveDefinitions(); err != nil {
		return err
	}

	// Populate directive uses (loaders, resolvers, deprecations, etc.)
	if err = b.populateDirectiveUses(); err != nil {
		return err
	}

	if err = b.setFieldResolution(); err != nil {
		return err
	}

	// Build service dependency graph and validate DAG
	if err = b.buildServiceDependencies(); err != nil {
		return err
	}

	return nil
}

func (b *builder) addViolation(v ...*Violation) {
	b.violations = append(b.violations, v...)
}
