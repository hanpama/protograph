package protoreg

import (
	"github.com/hanpama/protograph/internal/ir"
	"github.com/jhump/protoreflect/v2/protobuilder"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (b *builder) addServiceMethods(irSvc *ir.Service) {
	for _, resolverID := range irSvc.Resolvers {
		b.addResolver(irSvc, b.project.Resolvers[resolverID])
	}
	for _, loaderID := range irSvc.Loaders {
		b.addLoader(irSvc, b.project.Loaders[loaderID])
	}
}

func (b *builder) getOrCreateService(irSvf *ir.Service) *protobuilder.ServiceBuilder {
	sb, ok := b.serviceServiceBuilders[irSvf.ID]
	if !ok {
		sb = protobuilder.NewService(nameService(irSvf.Name))
		b.serviceServiceBuilders[irSvf.ID] = sb
		b.serviceFileBuilders[irSvf.ID].AddService(sb)
	}
	return sb
}

func (b *builder) addResolver(irs *ir.Service, irr *ir.ResolverDefinition) {
	serviceBuilder := b.getOrCreateService(irs)

	requestName := nameSingleResolverRequest(irr.Parent, irr.Field)
	requestMB := b.createSingleMethodRequest(requestName, irr.OrderedArgs())

	responseName := nameSingleResolverResponse(irr.Parent, irr.Field)
	responseMB := b.createSingleMethodResponse(responseName, irr.ReturnType)

	if irr.Batch {
		batchRequestName := nameBatchResolverRequest(irr.Parent, irr.Field)
		batchRequestMB := b.createBatchMethodRequest(batchRequestName, requestMB)

		batchResponseName := nameBatchResolverResponse(irr.Parent, irr.Field)
		batchResponseMB := b.createBatchMethodResponse(batchResponseName, responseMB)

		resolverName := nameBatchResolverMethod(irr.Parent, irr.Field)
		methodBuilder := protobuilder.NewMethod(
			resolverName,
			protobuilder.RpcTypeMessage(batchRequestMB, false),
			protobuilder.RpcTypeMessage(batchResponseMB, false),
		)
		methodBuilder.SetComments(comment(irr.Description))
		b.serviceFileBuilders[irs.ID].AddMessage(requestMB)
		b.serviceFileBuilders[irs.ID].AddMessage(responseMB)
		b.serviceFileBuilders[irs.ID].AddMessage(batchRequestMB)
		b.serviceFileBuilders[irs.ID].AddMessage(batchResponseMB)
		serviceBuilder.AddMethod(methodBuilder)

		// Store mapping: [serviceName, methodName] -> [objectType, field]
		b.batchResolverMethods[[2]string{string(serviceBuilder.Name()), string(resolverName)}] = [2]string{irr.Parent, irr.Field}
	} else {
		resolverName := nameSingleResolverMethod(irr.Parent, irr.Field)
		methodBuilder := protobuilder.NewMethod(
			resolverName,
			protobuilder.RpcTypeMessage(requestMB, false),
			protobuilder.RpcTypeMessage(responseMB, false),
		)
		methodBuilder.SetComments(comment(irr.Description))
		b.serviceFileBuilders[irs.ID].AddMessage(requestMB)
		b.serviceFileBuilders[irs.ID].AddMessage(responseMB)
		serviceBuilder.AddMethod(methodBuilder)

		// Store mapping: [serviceName, methodName] -> [objectType, field]
		b.singleResolverMethods[[2]string{string(serviceBuilder.Name()), string(resolverName)}] = [2]string{irr.Parent, irr.Field}
	}
}

func (b *builder) addLoader(irSvc *ir.Service, irl *ir.LoaderDefinition) {
	serviceBuilder := b.getOrCreateService(irSvc)

	requestName := nameSingleLoaderRequest(irl.TargetType, irl.KeyFields)
	requestMB := b.createSingleMethodRequest(requestName, irl.OrderedArgs())

	responseName := nameSingleLoaderResponse(irl.TargetType, irl.KeyFields)
	responseMB := b.createSingleMethodResponse(responseName, &ir.TypeExpr{
		Kind:  ir.TypeExprKindNamed,
		Named: irl.TargetType,
	})

	if irl.Batch {
		batchRequestName := nameBatchLoaderRequest(irl.TargetType, irl.KeyFields)
		batchRequestMB := b.createBatchMethodRequest(batchRequestName, requestMB)

		batchResponseName := nameBatchLoaderResponse(irl.TargetType, irl.KeyFields)
		batchResponseMB := b.createBatchMethodResponse(batchResponseName, responseMB)

		loaderName := nameBatchLoaderMethod(irl.TargetType, irl.KeyFields)
		methodBuilder := protobuilder.NewMethod(
			loaderName,
			protobuilder.RpcTypeMessage(batchRequestMB, false),
			protobuilder.RpcTypeMessage(batchResponseMB, false),
		)
		serviceBuilder.AddMethod(methodBuilder)
		b.serviceFileBuilders[irSvc.ID].AddMessage(batchRequestMB)
		b.serviceFileBuilders[irSvc.ID].AddMessage(batchResponseMB)
		b.serviceFileBuilders[irSvc.ID].AddMessage(requestMB)
		b.serviceFileBuilders[irSvc.ID].AddMessage(responseMB)

		methodBuilder.Build()

		// Store mapping: LoaderID -> [serviceName, methodName]
		b.batchLoaderMethodsByID[irl.ID] = [2]string{string(serviceBuilder.Name()), string(loaderName)}
	} else {
		loaderName := nameSingleLoaderMethod(irl.TargetType, irl.KeyFields)
		methodBuilder := protobuilder.NewMethod(
			loaderName,
			protobuilder.RpcTypeMessage(requestMB, false),
			protobuilder.RpcTypeMessage(responseMB, false),
		)
		serviceBuilder.AddMethod(methodBuilder)
		b.serviceFileBuilders[irSvc.ID].AddMessage(requestMB)
		b.serviceFileBuilders[irSvc.ID].AddMessage(responseMB)

		// Store mapping: LoaderID -> [serviceName, methodName]
		b.singleLoaderMethodsByID[irl.ID] = [2]string{string(serviceBuilder.Name()), string(loaderName)}
	}
}

func (b *builder) createSingleMethodRequest(requestName protoreflect.Name, args []*ir.MethodArg) *protobuilder.MessageBuilder {
	requestMB := protobuilder.NewMessage(requestName)
	requestFields := make([]*protobuilder.FieldBuilder, 0, len(args))
	for _, arg := range args {
		rt := b.resolveTypeExpr(arg.Type)
		fb := protobuilder.NewField(nameProtoField(arg.Name), rt.fieldType)
		fb.SetComments(comment(arg.Description))
		if rt.isOptional {
			fb.SetOptional()
		}
		if rt.isRepeated {
			fb.SetRepeated()
		}
		requestMB.AddField(fb)
		requestFields = append(requestFields, fb)
	}
	allocateFieldNumbers(requestFields)
	return requestMB
}

func (b *builder) createSingleMethodResponse(responseName protoreflect.Name, returnType *ir.TypeExpr) *protobuilder.MessageBuilder {
	responseMB := protobuilder.NewMessage(responseName)
	rt := b.resolveTypeExpr(returnType)
	fb := protobuilder.NewField(nameProtoField("data"), rt.fieldType)
	fb.SetNumber(protoreflect.FieldNumber(1))
	if rt.isOptional {
		fb.SetOptional()
	}
	if rt.isRepeated {
		fb.SetRepeated()
	}
	responseMB.AddField(fb)
	return responseMB
}

func (b *builder) createBatchMethodRequest(requestName protoreflect.Name, singleRequestMB *protobuilder.MessageBuilder) *protobuilder.MessageBuilder {
	batchRequestMB := protobuilder.NewMessage(requestName)
	batchRequestField := protobuilder.NewField(nameProtoField("batches"), protobuilder.FieldTypeMessage(singleRequestMB))
	batchRequestField.SetNumber(protoreflect.FieldNumber(1))
	batchRequestField.SetRepeated()
	batchRequestMB.AddField(batchRequestField)
	return batchRequestMB
}

func (b *builder) createBatchMethodResponse(responseName protoreflect.Name, singleResponseMB *protobuilder.MessageBuilder) *protobuilder.MessageBuilder {
	batchResponseMB := protobuilder.NewMessage(responseName)
	batchResponseField := protobuilder.NewField(nameProtoField("batches"), protobuilder.FieldTypeMessage(singleResponseMB))
	batchResponseField.SetNumber(protoreflect.FieldNumber(1))
	batchResponseField.SetRepeated()
	batchResponseMB.AddField(batchResponseField)
	return batchResponseMB
}
