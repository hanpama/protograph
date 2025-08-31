package ir

import (
	"fmt"

	language "github.com/hanpama/protograph/internal/language"
)

// buildServiceDependencies populates Service.Dependencies based on cross-type references with ownership-awareness.
// A service A depends on service B iff A, in its own SDL (definitions) or in resolvers/loaders it owns,
// references a top-level type owned by B. Extensions made by other services do not attribute dependencies to the
// canonical owner of the extended type.
func (b *builder) buildServiceDependencies() error {
	// Map: type name -> owning service (only true definitions, not extensions)
	owner := make(map[string]ServiceID)
	for sid, svc := range b.Services {
		for _, defName := range svc.Definitions {
			owner[defName] = sid
		}
	}

	// For each service, collect dependencies following ownership rules
	for sid, svc := range b.Services {
		depSet := make(map[ServiceID]struct{})

		// 1) Types declared by this service: consider only fields declared in this service's SDL definitions
		doc := b.serviceDocs[sid]
		if doc != nil {
			for _, node := range doc.Definitions {
				def := b.Definitions[node.Name]
				switch node.Kind {
				case language.Object:
					// Only fields declared on this node
					obj := def.Object
					for _, f := range node.Fields {
						fd := obj.Fields[f.Name]
						if fd == nil {
							continue
						}
						if base := fd.Type.unwrap(); base != "" {
							if o, ok := owner[base]; ok && o != svc.ID {
								depSet[o] = struct{}{}
							}
						}
						for _, arg := range f.Arguments {
							ad := fd.Args[arg.Name]
							if ad == nil {
								continue
							}
							if base := ad.Type.unwrap(); base != "" {
								if o, ok := owner[base]; ok && o != svc.ID {
									depSet[o] = struct{}{}
								}
							}
						}
					}
				case language.InputObject:
					in := def.Input
					for _, f := range node.Fields {
						iv := in.InputValues[f.Name]
						if iv == nil {
							continue
						}
						if base := iv.Type.unwrap(); base != "" {
							if o, ok := owner[base]; ok && o != svc.ID {
								depSet[o] = struct{}{}
							}
						}
					}
				}
			}
		}

		// 2) Interface/Union owned by this service: depend on all member types' owners
		for _, defName := range svc.Definitions {
			def := b.Definitions[defName]
			if def.Interface != nil {
				for _, typ := range def.Interface.PossibleTypes {
					if o, ok := owner[typ]; ok && o != svc.ID {
						depSet[o] = struct{}{}
					}
				}
			}
			if def.Union != nil {
				for member := range def.Union.Types {
					if o, ok := owner[member]; ok && o != svc.ID {
						depSet[o] = struct{}{}
					}
				}
			}
		}

		// 3) Resolvers/Loaders owned by this service: consider arg and return types
		for _, rid := range svc.Resolvers {
			res := b.Resolvers[rid]
			for _, a := range res.OrderedArgs() {
				if base := a.Type.unwrap(); base != "" {
					if o, ok := owner[base]; ok && o != svc.ID {
						depSet[o] = struct{}{}
					}
				}
			}
			if base := res.ReturnType.unwrap(); base != "" {
				if o, ok := owner[base]; ok && o != svc.ID {
					depSet[o] = struct{}{}
				}
			}
		}
		for _, lid := range svc.Loaders {
			ld := b.Loaders[lid]
			for _, a := range ld.OrderedArgs() {
				if base := a.Type.unwrap(); base != "" {
					if o, ok := owner[base]; ok && o != svc.ID {
						depSet[o] = struct{}{}
					}
				}
			}
			// Loader return type equals target object; owner is this service (by construction) -> no dep
		}

		// Assign deterministic order
		if len(depSet) > 0 {
			deps := make([]ServiceID, 0, len(depSet))
			for d := range depSet {
				deps = append(deps, d)
			}
			// simple insertion sort for stability
			for i := 0; i < len(deps)-1; i++ {
				for j := i + 1; j < len(deps); j++ {
					if deps[j] < deps[i] {
						deps[i], deps[j] = deps[j], deps[i]
					}
				}
			}
			svc.Dependencies = deps
		} else {
			// ensure empty slice vs nil is not significant downstream
			svc.Dependencies = nil
		}
	}

	// Detect cycles using DFS
	visited := make(map[ServiceID]int) // 0=unvisited,1=visiting,2=done
	var stack []ServiceID
	var cycleErr error
	var dfs func(ServiceID)
	dfs = func(s ServiceID) {
		if cycleErr != nil {
			return
		}
		state := visited[s]
		if state == 1 {
			path := append([]ServiceID{}, stack...)
			path = append(path, s)
			cycleErr = ValidationError([]*Violation{{
				Message: fmt.Sprintf("Service dependency cycle: %v", path),
			}})
			return
		}
		if state == 2 {
			return
		}
		visited[s] = 1
		stack = append(stack, s)
		for _, d := range b.Services[s].Dependencies {
			dfs(d)
			if cycleErr != nil {
				return
			}
		}
		stack = stack[:len(stack)-1]
		visited[s] = 2
	}
	for sid := range b.Services {
		dfs(sid)
		if cycleErr != nil {
			return cycleErr
		}
	}
	return nil
}
