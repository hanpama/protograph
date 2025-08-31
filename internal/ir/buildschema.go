package ir

import (
	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) processSchemaDefinitions() error {
	for _, doc := range b.serviceDocs {
		for _, schemaDef := range doc.Schema {
			if b.Schema != nil {
				b.addViolation(violationSchemaAlreadyDefined(schemaDef.Position))
				continue
			}
			b.Schema = &Schema{}
			for _, opType := range schemaDef.OperationTypes {
				switch opType.Operation {
				case language.Query:
					b.Schema.QueryType = opType.Type
				case language.Mutation:
					b.Schema.MutationType = opType.Type
				case language.Subscription:
					b.Schema.SubscriptionType = opType.Type
				}
			}
		}
	}

	// Schema definition is required
	if b.Schema == nil {
		b.addViolation(violationSchemaDefinitionRequired())
	} else {
		// Validate schema root types exist and are Object types
		if b.Schema.QueryType != "" {
			if def, ok := b.Definitions[b.Schema.QueryType]; !ok {
				b.addViolation(violationRootTypeNotFound("Query", b.Schema.QueryType))
			} else if def.Object == nil {
				b.addViolation(violationRootTypeNotObject("Query", b.Schema.QueryType))
			}
		}

		if b.Schema.MutationType != "" {
			if def, ok := b.Definitions[b.Schema.MutationType]; !ok {
				b.addViolation(violationRootTypeNotFound("Mutation", b.Schema.MutationType))
			} else if def.Object == nil {
				b.addViolation(violationRootTypeNotObject("Mutation", b.Schema.MutationType))
			}
		}

		if b.Schema.SubscriptionType != "" {
			if def, ok := b.Definitions[b.Schema.SubscriptionType]; !ok {
				b.addViolation(violationRootTypeNotFound("Subscription", b.Schema.SubscriptionType))
			} else if def.Object == nil {
				b.addViolation(violationRootTypeNotObject("Subscription", b.Schema.SubscriptionType))
			}
		}
	}

	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}
	return nil
}
