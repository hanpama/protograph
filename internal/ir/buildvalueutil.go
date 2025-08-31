package ir

import (
	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) getStringValue(node *language.Value) string {
	if node.Kind != language.StringValue {
		b.addViolation(violationExpectedString(node.Position))
		return ""
	}
	return node.Raw
}

func (b *builder) getStringListValue(node *language.Value) []string {
	if node.Kind != language.ListValue {
		b.addViolation(violationExpectedList(node.Position))
		return nil
	}
	var values []string
	for _, item := range node.Children {
		values = append(values, b.getStringValue(item.Value))
	}
	return values
}

func (b *builder) getStringMapValue(node *language.Value) map[string]string {
	if node.Kind != language.ObjectValue {
		b.addViolation(violationExpectedObject(node.Position))
		return nil
	}
	result := make(map[string]string)
	for _, field := range node.Children {
		result[field.Name] = b.getStringValue(field.Value)
	}
	return result
}

func (b *builder) getBoolValue(node *language.Value) bool {
	if node.Kind != language.BooleanValue {
		b.addViolation(violationExpectedBoolean(node.Position))
		return false
	}
	return node.Raw == "true"
}
