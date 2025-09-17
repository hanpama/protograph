package protoreg

import (
	"github.com/hanpama/protograph/internal/ir"
	"github.com/jhump/protoreflect/v2/protobuilder"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Build converts an ir project to a grpcrt.Registry implementation
func Build(p *ir.Project) (*Registry, error) {
	// reg := newRegistry()

	b := &builder{
		project:                   p,
		serviceFileBuilders:       make(map[ir.ServiceID]*protobuilder.FileBuilder),
		serviceServiceBuilders:    make(map[ir.ServiceID]*protobuilder.ServiceBuilder),
		definitionMessageBuilders: make(map[string]*protobuilder.MessageBuilder),
		enumBuilders:              make(map[string]*protobuilder.EnumBuilder),
		scalarMapping:             make(map[string]string),
		protoGQLTypeMap:           make(map[protoreflect.Name]string),
		protoGQLFieldMap:          make(map[[2]protoreflect.Name][2]string),

		singleResolverMethods:   make(map[[2]string][2]string),
		batchResolverMethods:    make(map[[2]string][2]string),
		singleLoaderMethodsByID: make(map[ir.LoaderID][2]string),
		batchLoaderMethodsByID:  make(map[ir.LoaderID][2]string),
		fieldLoaderIDs:          make(map[[2]string]ir.LoaderID),
	}

	// Pass 1: create file builders for each service
	for _, irSvc := range p.Services {
		b.buildServiceFileDescriptor(irSvc)
	}

	// Pass 2: add dependencies to each service
	for _, irSvc := range p.Services {
		for _, depSvcId := range irSvc.Dependencies {
			b.addServiceDependency(irSvc, depSvcId)
		}
	}

	for _, def := range p.Definitions {
		if def.Scalar != nil {
			b.addScalar(def.Scalar)
		}
	}

	// Pass 3: add builders/types for definitions
	for _, irSvc := range p.Services {
		for _, defName := range irSvc.Definitions {
			def := p.Definitions[defName]
			if def.Object != nil {
				if p.Schema.QueryType == def.Object.Name ||
					p.Schema.MutationType == def.Object.Name ||
					p.Schema.SubscriptionType == def.Object.Name {
					continue
				}
				b.addObjectSourceMessage(irSvc.ID, def.Object)
			} else if def.Interface != nil {
				b.addInterfaceSourceMessage(irSvc.ID, def.Interface)
			} else if def.Union != nil {
				b.addUnionSourceMessage(irSvc.ID, def.Union)
			} else if def.Input != nil {
				b.addInputObjectMessage(irSvc.ID, def.Input)
			} else if def.Enum != nil {
				b.addEnum(irSvc.ID, def.Enum)
			}
		}
	}

	// Pass 4: add source fields to objects, interfaces, unions, and input objects
	// Also collect field-loader mappings
	for _, def := range p.Definitions {
		if def.Object != nil {
			b.addObjectSourceMessageFields(def.Object)
		} else if def.Interface != nil {
			b.addInterfaceSourceMessageFields(def.Interface)
		} else if def.Union != nil {
			b.addUnionSourceMessageFields(def.Union)
		} else if def.Input != nil {
			b.addInputObjectMessageFields(def.Input)
		}
	}

	// Pass 5: add resolvers and loaders to services
	for _, irSvc := range p.Services {
		b.addServiceMethods(irSvc)
	}

	reg := &Registry{
		fileDescriptors:           []protoreflect.FileDescriptor{},
		sourceFieldDescriptors:    map[[2]string]protoreflect.FieldDescriptor{},
		singleResolverDescriptors: map[[2]string]protoreflect.MethodDescriptor{},
		batchResolverDescriptors:  map[[2]string]protoreflect.MethodDescriptor{},
		singleLoaderDescriptors:   map[[2]string]protoreflect.MethodDescriptor{},
		batchLoaderDescriptors:    map[[2]string]protoreflect.MethodDescriptor{},
		requestFieldSourceMap:     map[[2]string]map[string]string{},
		sourceMessageDescriptors:  map[string]protoreflect.MessageDescriptor{},
	}

	// Build file descriptors and populate registry
	for _, fb := range b.serviceFileBuilders {
		fd, err := fb.Build()
		if err != nil {
			return nil, err
		}
		reg.fileDescriptors = append(reg.fileDescriptors, fd)

		// Populate source field descriptors
		messages := fd.Messages()
		for i := 0; i < messages.Len(); i++ {
			msg := messages.Get(i)
			// Check if this is a Source message by looking up the GraphQL type name
			gqlType := b.protoGQLTypeMap[msg.Name()]
			if gqlType != "" {
				reg.sourceMessageDescriptors[gqlType] = msg
				fields := msg.Fields()
				for j := 0; j < fields.Len(); j++ {
					field := fields.Get(j)
					if gqlFieldNames, ok := b.protoGQLFieldMap[[2]protoreflect.Name{msg.Name(), field.Name()}]; ok {
						reg.sourceFieldDescriptors[[2]string{gqlFieldNames[0], gqlFieldNames[1]}] = field
					}
				}
			}
		}

		// Populate method descriptors using stored mappings
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			svcName := string(svc.Name())
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				methodName := string(method.Name())
				svcMethodKey := [2]string{svcName, methodName}

				// Check single resolver mappings
				if gqlNames, ok := b.singleResolverMethods[svcMethodKey]; ok {
					reg.singleResolverDescriptors[gqlNames] = method
					// Populate request field source mapping from IR
					if def, ok := b.project.Definitions[gqlNames[0]]; ok && def.Object != nil {
						if fld, ok := def.Object.Fields[gqlNames[1]]; ok && fld.ResolveByResolver != nil && len(fld.ResolveByResolver.With) > 0 {
							// copy map
							mp := make(map[string]string, len(fld.ResolveByResolver.With))
							for k, v := range fld.ResolveByResolver.With {
								mp[k] = v
							}
							reg.requestFieldSourceMap[gqlNames] = mp
						}
					}
				}

				// Check batch resolver mappings
				if gqlNames, ok := b.batchResolverMethods[svcMethodKey]; ok {
					reg.batchResolverDescriptors[gqlNames] = method
					// Populate request field source mapping from IR (batch resolver uses same request args shape)
					if def, ok := b.project.Definitions[gqlNames[0]]; ok && def.Object != nil {
						if fld, ok := def.Object.Fields[gqlNames[1]]; ok && fld.ResolveByResolver != nil && len(fld.ResolveByResolver.With) > 0 {
							mp := make(map[string]string, len(fld.ResolveByResolver.With))
							for k, v := range fld.ResolveByResolver.With {
								mp[k] = v
							}
							reg.requestFieldSourceMap[gqlNames] = mp
						}
					}
				}
			}
		}
	}

	// Now connect loader methods through the LoaderID mappings
	for gqlField, loaderID := range b.fieldLoaderIDs {
		// Check if this loader has a single method
		if svcMethod, ok := b.singleLoaderMethodsByID[loaderID]; ok {
			// Find the method descriptor
			for _, fd := range reg.fileDescriptors {
				services := fd.Services()
				for i := 0; i < services.Len(); i++ {
					svc := services.Get(i)
					if string(svc.Name()) == svcMethod[0] {
						methods := svc.Methods()
						for j := 0; j < methods.Len(); j++ {
							method := methods.Get(j)
							if string(method.Name()) == svcMethod[1] {
								reg.singleLoaderDescriptors[gqlField] = method
								break
							}
						}
					}
				}
			}
		}

		// Check if this loader has a batch method
		if svcMethod, ok := b.batchLoaderMethodsByID[loaderID]; ok {
			// Find the method descriptor
			for _, fd := range reg.fileDescriptors {
				services := fd.Services()
				for i := 0; i < services.Len(); i++ {
					svc := services.Get(i)
					if string(svc.Name()) == svcMethod[0] {
						methods := svc.Methods()
						for j := 0; j < methods.Len(); j++ {
							method := methods.Get(j)
							if string(method.Name()) == svcMethod[1] {
								reg.batchLoaderDescriptors[gqlField] = method
								break
							}
						}
					}
				}

				// Populate request field source mapping for loader fields from IR
				if def, ok := b.project.Definitions[gqlField[0]]; ok && def.Object != nil {
					if fld, ok := def.Object.Fields[gqlField[1]]; ok && fld.ResolveByLoader != nil && len(fld.ResolveByLoader.With) > 0 {
						mp := make(map[string]string, len(fld.ResolveByLoader.With))
						for k, v := range fld.ResolveByLoader.With {
							mp[k] = v
						}
						reg.requestFieldSourceMap[gqlField] = mp
					}
				}
			}
		}
	}

	return reg, nil
}

type builder struct {
	project *ir.Project

	serviceFileBuilders       map[ir.ServiceID]*protobuilder.FileBuilder
	serviceServiceBuilders    map[ir.ServiceID]*protobuilder.ServiceBuilder
	definitionMessageBuilders map[string]*protobuilder.MessageBuilder
	enumBuilders              map[string]*protobuilder.EnumBuilder
	scalarMapping             map[string]string
	protoGQLTypeMap           map[protoreflect.Name]string
	protoGQLFieldMap          map[[2]protoreflect.Name][2]string

	// Method mappings for resolvers: [serviceName, methodName] -> [objectType, field]
	singleResolverMethods map[[2]string][2]string
	batchResolverMethods  map[[2]string][2]string

	// Method mappings for loaders: LoaderID -> [serviceName, methodName]
	singleLoaderMethodsByID map[ir.LoaderID][2]string
	batchLoaderMethodsByID  map[ir.LoaderID][2]string

	// Field to loader mappings: [objectType, field] -> LoaderID
	fieldLoaderIDs map[[2]string]ir.LoaderID
}
