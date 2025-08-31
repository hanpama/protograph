package protoreg

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func nameProtoSource(graphQLName string) protoreflect.Name {
	return protoreflect.Name(graphQLName + "Source")
}

func nameProtoField(graphQLName string) protoreflect.Name {
	return protoreflect.Name(snakeCase(graphQLName))
}

func nameProtoEnumValue(graphQLEnumName string, graphQLEnumValueName string) protoreflect.Name {
	prefix := strings.ToUpper(snakeCase(graphQLEnumName))
	value := graphQLEnumValueName
	return protoreflect.Name(prefix + "_" + value)
}

func nameService(serviceName string) protoreflect.Name {
	return protoreflect.Name(capitalize(serviceName) + "Service")
}

func nameSingleResolverMethod(objectType string, fieldName string) protoreflect.Name {
	return protoreflect.Name("Resolve" + capitalize(objectType) + capitalize(fieldName))
}
func nameBatchResolverMethod(objectType string, fieldName string) protoreflect.Name {
	return protoreflect.Name("BatchResolve" + capitalize(objectType) + capitalize(fieldName))
}
func nameSingleResolverRequest(objectType string, fieldName string) protoreflect.Name {
	return protoreflect.Name(string(nameSingleResolverMethod(objectType, fieldName)) + "Request")
}
func nameSingleResolverResponse(objectType string, fieldName string) protoreflect.Name {
	return protoreflect.Name(string(nameSingleResolverMethod(objectType, fieldName)) + "Response")
}
func nameBatchResolverRequest(objectType string, fieldName string) protoreflect.Name {
	return protoreflect.Name(string(nameBatchResolverMethod(objectType, fieldName)) + "Request")
}
func nameBatchResolverResponse(objectType string, fieldName string) protoreflect.Name {
	return protoreflect.Name(string(nameBatchResolverMethod(objectType, fieldName)) + "Response")
}

func nameSingleLoaderMethod(targetType string, keyFields []string) protoreflect.Name {
	capitalizedKeys := make([]string, len(keyFields))
	for i, k := range keyFields {
		capitalizedKeys[i] = capitalize(k)
	}
	return protoreflect.Name("Load" + capitalize(targetType) + "By" + strings.Join(capitalizedKeys, ""))
}
func nameSingleLoaderResponse(targetType string, keyFields []string) protoreflect.Name {
	return protoreflect.Name(string(nameSingleLoaderMethod(targetType, keyFields)) + "Response")
}
func nameSingleLoaderRequest(targetType string, keyFields []string) protoreflect.Name {
	return protoreflect.Name(string(nameSingleLoaderMethod(targetType, keyFields)) + "Request")
}

func nameBatchLoaderMethod(targetType string, keyFields []string) protoreflect.Name {
	capitalizedKeys := make([]string, len(keyFields))
	for i, k := range keyFields {
		capitalizedKeys[i] = capitalize(k)
	}
	return protoreflect.Name("BatchLoad" + capitalize(targetType) + "By" + strings.Join(capitalizedKeys, ""))
}
func nameBatchLoaderResponse(targetType string, keyFields []string) protoreflect.Name {
	return protoreflect.Name(string(nameBatchLoaderMethod(targetType, keyFields)) + "Response")
}
func nameBatchLoaderRequest(targetType string, keyFields []string) protoreflect.Name {
	return protoreflect.Name(string(nameBatchLoaderMethod(targetType, keyFields)) + "Request")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// snakeCase converts a string from CamelCase or PascalCase to snake_case.
func snakeCase(s string) string {
	result := ""
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result += "_"
		}
		result += string(r)
	}
	return strings.ToLower(result)
}
