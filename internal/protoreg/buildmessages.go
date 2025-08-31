package protoreg

import (
	"path/filepath"
	"strings"

	"github.com/hanpama/protograph/internal/ir"
	"github.com/jhump/protoreflect/v2/protobuilder"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// buildServiceProto creates a FileDescriptorProto for a service
func (b *builder) buildServiceFileDescriptor(irSvc *ir.Service) {
	fp := filepath.Join(irSvc.PackagePath...)
	fp = filepath.Join(fp, strings.TrimSuffix(irSvc.FilePath, ".graphql")+".proto")

	pkgQn := strings.Join(irSvc.PackagePath, ".")

	pb := protobuilder.NewFile(fp)
	pb.SetPackageName(protoreflect.FullName(pkgQn))
	pb.SetSyntax(protoreflect.Proto3)

	b.serviceFileBuilders[irSvc.ID] = pb
}

func (b *builder) addServiceDependency(irSvc *ir.Service, depSvcID ir.ServiceID) {
	b.serviceFileBuilders[irSvc.ID].AddDependency(b.serviceFileBuilders[depSvcID])
}

func (b *builder) addObjectSourceMessage(irSvcID ir.ServiceID, irObj *ir.ObjectDefinition) {
	messageName := nameProtoSource(irObj.Name)
	mb := protobuilder.NewMessage(messageName)
	mb.SetComments(comment(irObj.Description))
	b.definitionMessageBuilders[irObj.Name] = mb
	b.protoGQLTypeMap[messageName] = irObj.Name
	b.serviceFileBuilders[irSvcID].AddMessage(b.definitionMessageBuilders[irObj.Name])
}
func (b *builder) addInterfaceSourceMessage(irSvcID ir.ServiceID, irIface *ir.InterfaceDefinition) {
	messageName := nameProtoSource(irIface.Name)
	mb := protobuilder.NewMessage(messageName)
	mb.SetComments(comment(irIface.Description))
	b.definitionMessageBuilders[irIface.Name] = mb
	b.protoGQLTypeMap[messageName] = irIface.Name
	b.serviceFileBuilders[irSvcID].AddMessage(b.definitionMessageBuilders[irIface.Name])
}
func (b *builder) addUnionSourceMessage(irSvcID ir.ServiceID, irUnion *ir.UnionDefinition) {
	messageName := nameProtoSource(irUnion.Name)
	mb := protobuilder.NewMessage(messageName)
	mb.SetComments(comment(irUnion.Description))
	b.definitionMessageBuilders[irUnion.Name] = mb
	b.protoGQLTypeMap[messageName] = irUnion.Name
	b.serviceFileBuilders[irSvcID].AddMessage(b.definitionMessageBuilders[irUnion.Name])
}
func (b *builder) addInputObjectMessage(irSvcID ir.ServiceID, irInputObj *ir.InputDefinition) {
	messageName := nameProtoSource(irInputObj.Name)
	mb := protobuilder.NewMessage(messageName)
	mb.SetComments(comment(irInputObj.Description))
	b.definitionMessageBuilders[irInputObj.Name] = mb
	b.protoGQLTypeMap[messageName] = irInputObj.Name
	b.serviceFileBuilders[irSvcID].AddMessage(b.definitionMessageBuilders[irInputObj.Name])
}
func (b *builder) addEnum(irSvcID ir.ServiceID, irEnum *ir.EnumDefinition) {
	enumName := nameProtoSource(irEnum.Name)
	eb := protobuilder.NewEnum(enumName)
	eb.SetComments(comment(irEnum.Description))
	b.enumBuilders[irEnum.Name] = eb
	b.protoGQLTypeMap[enumName] = irEnum.Name

	// Add default ZERO value: <ENUM>_UNSPECIFIED = 0
	zeroName := nameProtoEnumValue(irEnum.Name, "UNSPECIFIED")
	zero := protobuilder.NewEnumValue(zeroName)
	zero.SetNumber(0)
	eb.AddValue(zero)

	evbs := make([]*protobuilder.EnumValueBuilder, 0, len(irEnum.Values))
	for _, v := range irEnum.OrderedValues() {
		name := strings.ToUpper(v.Name)
		if name == "UNSPECIFIED" {
			continue
		}
		// nameProtoEnumValue(irEnum.Name, v.Name)
		prefix := strings.ToUpper(snakeCase(irEnum.Name))

		evb := protobuilder.NewEnumValue(protoreflect.Name(prefix + "_" + name))
		evb.SetComments(comment(v.Description))
		eb.AddValue(evb)
		evbs = append(evbs, evb)
	}
	allocateEnumValueNumbers(evbs)

	b.serviceFileBuilders[irSvcID].AddEnum(eb)
}
func (b *builder) addScalar(irScalar *ir.ScalarDefinition) {
	b.scalarMapping[irScalar.Name] = irScalar.MappedToProtoType
	b.protoGQLTypeMap[protoreflect.Name(irScalar.Name)] = irScalar.Name
}

func (b *builder) addObjectSourceMessageFields(irObj *ir.ObjectDefinition) {
	mb := b.definitionMessageBuilders[irObj.Name]

	messageFields := make([]*ir.FieldDefinition, 0, len(irObj.Fields))
	for _, field := range irObj.OrderedFields() {
		if field.IsInternal || field.ResolveBySource != nil {
			messageFields = append(messageFields, field)
		}
	}
	fieldBuilders := make([]*protobuilder.FieldBuilder, 0, len(messageFields))
	for _, field := range messageFields {
		rt := b.resolveTypeExpr(field.Type)
		fieldName := nameProtoField(field.Name)

		fb := protobuilder.NewField(fieldName, rt.fieldType)
		fb.SetComments(comment(field.Description))
		if rt.isOptional {
			fb.SetOptional()
		}
		if rt.isRepeated {
			fb.SetRepeated()
		}
		mb.AddField(fb)
		fieldBuilders = append(fieldBuilders, fb)
		b.protoGQLFieldMap[[2]protoreflect.Name{mb.Name(), fb.Name()}] = [2]string{irObj.Name, field.Name}
	}

	for _, field := range irObj.Fields {
		if field.ResolveByLoader != nil {
			b.fieldLoaderIDs[[2]string{irObj.Name, field.Name}] = field.ResolveByLoader.LoaderID
		}
	}
	allocateFieldNumbers(fieldBuilders)
}

func (b *builder) addInterfaceSourceMessageFields(irIface *ir.InterfaceDefinition) {
	mb := b.definitionMessageBuilders[irIface.Name]

	oneOfBuilder := protobuilder.NewOneof(protoreflect.Name("value"))
	mb.AddOneOf(oneOfBuilder)

	fieldBuilders := make([]*protobuilder.FieldBuilder, 0, len(irIface.PossibleTypes))
	for _, typ := range irIface.PossibleTypes {
		fb := protobuilder.NewField(protoreflect.Name(typ), protobuilder.FieldTypeMessage(b.definitionMessageBuilders[typ]))
		fieldBuilders = append(fieldBuilders, fb)
		oneOfBuilder.AddChoice(fb)
	}
	allocateFieldNumbers(fieldBuilders)
}

func (b *builder) addUnionSourceMessageFields(irUnion *ir.UnionDefinition) {
	mb := b.definitionMessageBuilders[irUnion.Name]

	oneOfBuilder := protobuilder.NewOneof(protoreflect.Name("value"))
	mb.AddOneOf(oneOfBuilder)

	fieldBuilders := make([]*protobuilder.FieldBuilder, 0, len(irUnion.Types))
	for _, typ := range irUnion.Types {
		fb := protobuilder.NewField(protoreflect.Name(typ.Name), protobuilder.FieldTypeMessage(b.definitionMessageBuilders[typ.Name]))
		fieldBuilders = append(fieldBuilders, fb)
		oneOfBuilder.AddChoice(fb)
	}
	allocateFieldNumbers(fieldBuilders)
}

func (b *builder) addInputObjectMessageFields(irInputObj *ir.InputDefinition) {
	mb := b.definitionMessageBuilders[irInputObj.Name]

	messageFields := make([]*ir.InputValueDefinition, 0, len(irInputObj.InputValues))
	for _, field := range irInputObj.OrderedInputValues() {
		messageFields = append(messageFields, field)
	}

	fieldBuilders := make([]*protobuilder.FieldBuilder, 0, len(messageFields))
	for _, field := range messageFields {
		rt := b.resolveTypeExpr(field.Type)
		fieldName := nameProtoField(field.Name)

		fb := protobuilder.NewField(fieldName, rt.fieldType)
		fb.SetComments(comment(field.Description))
		if rt.isOptional {
			fb.SetOptional()
		}
		if rt.isRepeated {
			fb.SetRepeated()
		}
		mb.AddField(fb)
		fieldBuilders = append(fieldBuilders, fb)
		b.protoGQLFieldMap[[2]protoreflect.Name{mb.Name(), fb.Name()}] = [2]string{irInputObj.Name, field.Name}
	}
	allocateFieldNumbers(fieldBuilders)
}
