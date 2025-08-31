package protoreg

import (
	"github.com/hanpama/protograph/internal/ir"
	"github.com/jhump/protoreflect/v2/protobuilder"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type resolvedType struct {
	isRepeated bool
	isOptional bool
	fieldType  *protobuilder.FieldType
}

func (b *builder) resolveTypeExpr(typeExpr *ir.TypeExpr) resolvedType {
	switch typeExpr.Kind {
	case ir.TypeExprKindNamed:
		return resolvedType{
			isRepeated: false,
			isOptional: true,
			fieldType:  b.mapNamedType(typeExpr.Named),
		}
	case ir.TypeExprKindList:
		elemType := b.resolveTypeExpr(typeExpr.OfType)
		return resolvedType{
			isRepeated: true,
			isOptional: false,
			fieldType:  elemType.fieldType,
		}
	case ir.TypeExprKindNonNull:
		innerType := b.resolveTypeExpr(typeExpr.OfType)
		return resolvedType{
			isRepeated: innerType.isRepeated,
			isOptional: false,
			fieldType:  innerType.fieldType,
		}
	}
	panic("unreachable")
}

func (b *builder) mapNamedType(typeName string) *protobuilder.FieldType {
	if protoType, ok := b.scalarMapping[typeName]; ok {
		return protobuilder.FieldTypeScalar(scalars[protoType])
	}
	if mb, ok := b.definitionMessageBuilders[typeName]; ok {
		return protobuilder.FieldTypeMessage(mb)
	}
	if eb, ok := b.enumBuilders[typeName]; ok {
		return protobuilder.FieldTypeEnum(eb)
	}
	panic("unreachable: " + typeName)
}

var scalars = map[string]protoreflect.Kind{
	protoreflect.BoolKind.String():     protoreflect.BoolKind,
	protoreflect.EnumKind.String():     protoreflect.EnumKind,
	protoreflect.Int32Kind.String():    protoreflect.Int32Kind,
	protoreflect.Sint32Kind.String():   protoreflect.Sint32Kind,
	protoreflect.Uint32Kind.String():   protoreflect.Uint32Kind,
	protoreflect.Int64Kind.String():    protoreflect.Int64Kind,
	protoreflect.Sint64Kind.String():   protoreflect.Sint64Kind,
	protoreflect.Uint64Kind.String():   protoreflect.Uint64Kind,
	protoreflect.Sfixed32Kind.String(): protoreflect.Sfixed32Kind,
	protoreflect.Fixed32Kind.String():  protoreflect.Fixed32Kind,
	protoreflect.FloatKind.String():    protoreflect.FloatKind,
	protoreflect.Sfixed64Kind.String(): protoreflect.Sfixed64Kind,
	protoreflect.Fixed64Kind.String():  protoreflect.Fixed64Kind,
	protoreflect.DoubleKind.String():   protoreflect.DoubleKind,
	protoreflect.StringKind.String():   protoreflect.StringKind,
	protoreflect.BytesKind.String():    protoreflect.BytesKind,
	protoreflect.MessageKind.String():  protoreflect.MessageKind,
	protoreflect.GroupKind.String():    protoreflect.GroupKind,
}
