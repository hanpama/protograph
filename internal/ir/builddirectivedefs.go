package ir

func (b *builder) populateDirectiveDefinitions() error {
	for _, doc := range b.serviceDocs {
		for _, directive := range doc.Directives {
			if _, ok := b.Directives[directive.Name]; ok {
				b.addViolation(violationDirectiveAlreadyDefined(directive.Name, directive.Position))
				continue
			}

			def := &DirectiveDefinition{
				Name:        directive.Name,
				Description: directive.Description,
				Args:        make(map[string]*ArgumentDefinition, len(directive.Arguments)),
				Repeatable:  directive.IsRepeatable,
				Locations:   make([]string, len(directive.Locations)),
			}

			// Convert locations to strings
			for i, loc := range directive.Locations {
				def.Locations[i] = string(loc)
			}

			// Process arguments
			for _, argNode := range directive.Arguments {
				def.Args[argNode.Name] = b.projectArgumentDefinition(len(def.Args), argNode)
			}

			b.Directives[directive.Name] = def
		}
	}

	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}
	return nil
}
