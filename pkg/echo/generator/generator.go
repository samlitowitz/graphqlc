package generator

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/graphql-go/graphql/language/printer"

	"github.com/graphql-go/graphql/language/ast"

	"github.com/samlitowitz/graphqlc/pkg/graphqlc"

	"github.com/samlitowitz/graphqlc/pkg/graphqlc/compiler"
)

// file rename function
type FnRenameFile func(string) string

type Generator struct {
	*bytes.Buffer
	Request  *compiler.CodeGeneratorRequest  // The input
	Response *compiler.CodeGeneratorResponse // The output

	FnRenameFile FnRenameFile // File renamer
}

func New() *Generator {
	g := new(Generator)
	g.Buffer = new(bytes.Buffer)
	g.Request = new(compiler.CodeGeneratorRequest)
	g.Response = new(compiler.CodeGeneratorResponse)
	return g
}

// Error reports a problem, including an error, and exits the program.
func (g *Generator) Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("graphqlc-gen-test: error:", s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func (g *Generator) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("graphqlc-gen-test: error:", s)
	os.Exit(1)
}

func (g *Generator) GenerateAllFiles() {
	genFiles := make(map[string]bool)
	for _, file := range g.Request.FileToGenerate {
		genFiles[file] = true
	}

	if g.FnRenameFile == nil {
		g.FnRenameFile = graphqlEchoFileName
	}

	for _, fd := range g.Request.GraphqlFile {
		if gen, ok := genFiles[fd.Name]; !ok || !gen {
			continue
		}

		g.Reset()
		err := g.generate(fd)
		if err != nil {
			g.Error(err)
		}
		g.Response.File = append(g.Response.File, &compiler.CodeGeneratorResponse_File{
			Name:    g.FnRenameFile(fd.Name),
			Content: g.String(),
		})
	}
}

func (g *Generator) generate(fd *graphqlc.FileDescriptorGraphql) error {
	doc := ast.NewDocument(&ast.Document{})

	schemaDef, err := buildSchemaDefinition(fd.Schema)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, schemaDef)

	objectDefs, err := buildObjectTypeDefinitions(fd.Objects)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, objectDefs...)

	inputObjectDefs, err := buildInputObjectDefinitions(fd.InputObjects)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, inputObjectDefs...)

	interfaceDefs, err := buildInterfaceDefinitions(fd.Interfaces)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, interfaceDefs...)

	unionDefs, err := buildUnionDefinitions(fd.Unions)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, unionDefs...)

	directiveDefs, err := buildDirectiveDefinitions(fd.Directives)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, directiveDefs...)

	scalarDefs, err := buildScalarDefinitions(fd.Scalars)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, scalarDefs...)

	enumDefs, err := buildEnumDefinitions(fd.Enums)
	if err != nil {
		return err
	}
	doc.Definitions = append(doc.Definitions, enumDefs...)

	data, _ := printer.Print(doc).(string)
	g.WriteString(data)

	return nil
}

func buildSchemaDefinition(desc *graphqlc.SchemaDescriptorProto) (*ast.SchemaDefinition, error) {
	if desc == nil {
		return nil, nil
	}
	def := ast.NewSchemaDefinition(&ast.SchemaDefinition{})

	directives, err := buildDirectives(desc.Directives)
	if err != nil {
		return nil, err
	}
	def.Directives = directives

	if desc.Query != nil {
		query := buildOperationTypeDefinition(desc.Query, "query")
		def.OperationTypes = append(def.OperationTypes, query)
	}

	if desc.Mutation != nil {
		mutation := buildOperationTypeDefinition(desc.Mutation, "mutation")
		def.OperationTypes = append(def.OperationTypes, mutation)
	}

	if desc.Subscription != nil {
		subscription := buildOperationTypeDefinition(desc.Subscription, "subscription")
		def.OperationTypes = append(def.OperationTypes, subscription)
	}

	return def, nil
}

func buildDirectiveDefinitions(descs []*graphqlc.DirectiveDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node
	for _, desc := range descs {
		args, err := buildInputValueDefinitions(desc.Arguments)
		if err != nil {
			return nil, err
		}
		var locs []*ast.Name
		for _, locDesc := range desc.Locations {
			switch lval := locDesc.Location.(type) {
			case *graphqlc.DirectiveLocationDescriptorProto_ExecutableLocation:
				locs = append(locs, ast.NewName(&ast.Name{Value: graphqlc.ExecutableDirectiveLocation_name[int32(lval.ExecutableLocation)]}))
			case *graphqlc.DirectiveLocationDescriptorProto_TypeSystemLocation:
				locs = append(locs, ast.NewName(&ast.Name{Value: graphqlc.TypeSystemDirectiveLocation_name[int32(lval.TypeSystemLocation)]}))
			}
		}

		defs = append(defs, ast.NewDirectiveDefinition(&ast.DirectiveDefinition{
			Name:        ast.NewName(&ast.Name{Value: desc.Name}),
			Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
			Arguments:   args,
			Locations:   locs,
		}))
	}
	return defs, nil
}

func buildScalarDefinitions(descs []*graphqlc.ScalarTypeDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node

	for _, desc := range descs {
		directives, err := buildDirectives(desc.Directives)
		if err != nil {
			return nil, err
		}
		defs = append(defs, ast.NewScalarDefinition(&ast.ScalarDefinition{
			Name: ast.NewName(&ast.Name{
				Value: desc.Name,
			}),
			Description: ast.NewStringValue(&ast.StringValue{
				Value: desc.Description,
			}),
			Directives: directives,
		}))
	}

	return defs, nil
}

func buildObjectTypeDefinitions(descs []*graphqlc.ObjectTypeDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node

	for _, desc := range descs {
		directives, err := buildDirectives(desc.Directives)
		if err != nil {
			return nil, err
		}
		fields, err := buildFieldDefinitions(desc.Fields)
		if err != nil {
			return nil, err
		}

		def := ast.NewObjectDefinition(&ast.ObjectDefinition{
			Name:        ast.NewName(&ast.Name{Value: desc.Name}),
			Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
			Directives:  directives,
			Fields:      fields,
			Interfaces:  buildInterfaces(desc.Implements),
		})

		defs = append(defs, def)
	}

	return defs, nil
}

func buildInterfaceDefinitions(descs []*graphqlc.InterfaceTypeDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node

	for _, desc := range descs {
		directives, err := buildDirectives(desc.Directives)
		if err != nil {
			return nil, err
		}
		fields, err := buildFieldDefinitions(desc.Fields)
		if err != nil {
			return nil, err
		}

		def := ast.NewInterfaceDefinition(&ast.InterfaceDefinition{
			Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
			Name:        ast.NewName(&ast.Name{Value: desc.Name}),
			Directives:  directives,
			Fields:      fields,
		})

		defs = append(defs, def)
	}

	return defs, nil
}

func buildUnionDefinitions(descs []*graphqlc.UnionTypeDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node

	for _, desc := range descs {
		directives, err := buildDirectives(desc.Directives)
		if err != nil {
			return nil, err
		}
		typs, err := buildMemberTypes(desc.MemberTypes)
		if err != nil {
			return nil, err
		}
		defs = append(defs, ast.NewUnionDefinition(&ast.UnionDefinition{
			Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
			Name:        ast.NewName(&ast.Name{Value: desc.Name}),
			Directives:  directives,
			Types:       typs,
		}))
	}

	return defs, nil
}

func buildEnumDefinitions(descs []*graphqlc.EnumTypeDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node
	for _, desc := range descs {
		directives, err := buildDirectives(desc.Directives)
		if err != nil {
			return nil, err
		}
		values, err := buildEnumValueDefinitions(desc.Values)
		if err != nil {
			return nil, err
		}
		def := ast.NewEnumDefinition(&ast.EnumDefinition{
			Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
			Name:        ast.NewName(&ast.Name{Value: desc.Name}),
			Directives:  directives,
			Values:      values,
		})
		defs = append(defs, def)
	}
	return defs, nil
}

func buildInputObjectDefinitions(descs []*graphqlc.InputObjectTypeDefinitionDescriptorProto) ([]ast.Node, error) {
	var defs []ast.Node
	for _, desc := range descs {
		directives, err := buildDirectives(desc.Directives)
		if err != nil {
			return nil, err
		}
		fields, err := buildInputValueDefinitions(desc.Fields)
		if err != nil {
			return nil, err
		}

		def := ast.NewInputObjectDefinition(&ast.InputObjectDefinition{
			Name:        ast.NewName(&ast.Name{Value: desc.Name}),
			Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
			Directives:  directives,
			Fields:      fields,
		})

		defs = append(defs, def)
	}
	return defs, nil
}

func buildEnumValueDefinitions(descs []*graphqlc.EnumValueDefinitionDescription) ([]*ast.EnumValueDefinition, error) {
	var defs []*ast.EnumValueDefinition
	for _, desc := range descs {
		def, err := buildEnumValueDefinition(desc)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func buildEnumValueDefinition(desc *graphqlc.EnumValueDefinitionDescription) (*ast.EnumValueDefinition, error) {
	directives, err := buildDirectives(desc.Directives)
	if err != nil {
		return nil, err
	}

	return ast.NewEnumValueDefinition(&ast.EnumValueDefinition{
		Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
		Name:        ast.NewName(&ast.Name{Value: desc.Value}),
		Directives:  directives,
	}), nil
}

func buildInterfaces(descs []*graphqlc.InterfaceTypeDefinitionDescriptorProto) []*ast.Named {
	var defs []*ast.Named
	for _, desc := range descs {
		defs = append(defs, ast.NewNamed(&ast.Named{Name: ast.NewName(&ast.Name{Value: desc.Name})}))
	}
	return defs
}

func buildFieldDefinitions(descs []*graphqlc.FieldDefinitionDescriptorProto) ([]*ast.FieldDefinition, error) {
	var defs []*ast.FieldDefinition
	for _, desc := range descs {
		def, err := buildFieldDefinition(desc)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func buildFieldDefinition(desc *graphqlc.FieldDefinitionDescriptorProto) (*ast.FieldDefinition, error) {
	directives, err := buildDirectives(desc.Directives)
	if err != nil {
		return nil, err
	}
	typ, err := buildType(desc.Type)
	if err != nil {
		return nil, err
	}
	arguments, err := buildInputValueDefinitions(desc.Arguments)
	if err != nil {
		return nil, err
	}

	def := ast.NewFieldDefinition(&ast.FieldDefinition{
		Name:        ast.NewName(&ast.Name{Value: desc.Name}),
		Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
		Directives:  directives,
		Type:        typ,
		Arguments:   arguments,
	})

	return def, nil
}

func buildInputValueDefinitions(descs []*graphqlc.InputValueDefinitionDescriptorProto) ([]*ast.InputValueDefinition, error) {
	var defs []*ast.InputValueDefinition
	for _, desc := range descs {
		def, err := buildInputValueDefinition(desc)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func buildInputValueDefinition(desc *graphqlc.InputValueDefinitionDescriptorProto) (*ast.InputValueDefinition, error) {
	directives, err := buildDirectives(desc.Directives)
	if err != nil {
		return nil, err
	}
	typ, err := buildType(desc.Type)
	if err != nil {
		return nil, err
	}
	def := ast.NewInputValueDefinition(&ast.InputValueDefinition{
		Name:        ast.NewName(&ast.Name{Value: desc.Name}),
		Description: ast.NewStringValue(&ast.StringValue{Value: desc.Description}),
		Type:        typ,
		Directives:  directives,
	})

	if desc.DefaultValue != nil {
		defaultValue, err := buildValue(desc.DefaultValue)
		if err != nil {
			return nil, err
		}
		def.DefaultValue = defaultValue
	}

	return def, nil
}

func buildMemberTypes(descs []*graphqlc.NamedTypeDescriptorProto) ([]*ast.Named, error) {
	var defs []*ast.Named
	for _, desc := range descs {
		defs = append(defs, ast.NewNamed(&ast.Named{Name: ast.NewName(&ast.Name{Value: desc.Name})}))
	}
	return defs, nil
}

func buildType(desc *graphqlc.TypeDescriptorProto) (ast.Type, error) {
	switch typDesc := desc.Type.(type) {
	case *graphqlc.TypeDescriptorProto_NamedType:
		return ast.NewNamed(&ast.Named{
			Name: ast.NewName(&ast.Name{
				Value: typDesc.NamedType.Name,
			}),
		}), nil
	case *graphqlc.TypeDescriptorProto_ListType:
		typDef, err := buildType(typDesc.ListType.Type)
		if err != nil {
			return nil, err
		}
		return ast.NewList(&ast.List{
			Type: typDef,
		}), nil
	case *graphqlc.TypeDescriptorProto_NonNullType:
		var nnDef *ast.NonNull
		switch typtypDesc := (typDesc.NonNullType.Type).(type) {
		case *graphqlc.NonNullTypeDescriptorProto_NamedType:
			nnDef = &ast.NonNull{
				Type: ast.NewNamed(&ast.Named{
					Name: ast.NewName(&ast.Name{Value: typtypDesc.NamedType.Name}),
				}),
			}
		case *graphqlc.NonNullTypeDescriptorProto_ListType:
			listDef, err := buildType(typtypDesc.ListType.Type)
			if err != nil {
				return nil, err
			}
			nnDef = &ast.NonNull{
				Type: listDef,
			}

		default:
			return nil, fmt.Errorf("unknown non-null type %#v", typDesc.NonNullType.Type)
		}
		return ast.NewNonNull(nnDef), nil
	}
	return nil, fmt.Errorf("unknown type %#v", desc.Type)
}

func buildOperationTypeDefinition(desc *graphqlc.ObjectTypeDefinitionDescriptorProto, operation string) *ast.OperationTypeDefinition {
	return ast.NewOperationTypeDefinition(&ast.OperationTypeDefinition{
		Operation: operation,
		Type: ast.NewNamed(&ast.Named{
			Name: ast.NewName(&ast.Name{
				Value: desc.Name,
			}),
		}),
	})
}

func buildDirectives(descs []*graphqlc.DirectiveDescriptorProto) ([]*ast.Directive, error) {
	var defs []*ast.Directive
	for _, desc := range descs {
		def, err := buildDirective(desc)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func buildDirective(desc *graphqlc.DirectiveDescriptorProto) (*ast.Directive, error) {
	def := ast.NewDirective(&ast.Directive{
		Name: ast.NewName(&ast.Name{Value: desc.Name}),
	})

	if desc.Arguments != nil {
		arguments, err := buildArguments(desc.Arguments)
		if err != nil {
			return nil, err
		}
		def.Arguments = arguments
	}

	return def, nil
}

func buildArguments(descs []*graphqlc.ArgumentDescriptorProto) ([]*ast.Argument, error) {
	var defs []*ast.Argument
	for _, desc := range descs {
		def, err := buildArgument(desc)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func buildArgument(desc *graphqlc.ArgumentDescriptorProto) (*ast.Argument, error) {
	def := ast.NewArgument(&ast.Argument{
		Name: ast.NewName(&ast.Name{Value: desc.Name}),
	})

	if desc.Value != nil {
		value, err := buildValue(desc.Value)
		if err != nil {
			return nil, err
		}
		def.Value = value
	}

	return def, nil
}

func buildValue(desc *graphqlc.ValueDescriptorProto) (ast.Value, error) {
	var def ast.Value
	switch v := desc.Value.(type) {
	case *graphqlc.ValueDescriptorProto_VariableValue:
		def = ast.NewVariable(&ast.Variable{
			Name: ast.NewName(&ast.Name{
				Value: v.VariableValue.Name,
			}),
		})
	case *graphqlc.ValueDescriptorProto_IntValue:
		def = ast.NewIntValue(&ast.IntValue{
			Value: fmt.Sprintf("%d", v.IntValue),
		})
	case *graphqlc.ValueDescriptorProto_FloatValue:
		def = ast.NewFloatValue(&ast.FloatValue{
			Value: fmt.Sprintf("%.6f", v.FloatValue),
		})
	case *graphqlc.ValueDescriptorProto_BooleanValue:
		def = ast.NewBooleanValue(&ast.BooleanValue{
			Value: v.BooleanValue,
		})
	case *graphqlc.ValueDescriptorProto_StringValue:
		def = ast.NewStringValue(&ast.StringValue{
			Value: v.StringValue,
		})
	case *graphqlc.ValueDescriptorProto_NullValue:
		// What does this look like from ast parse, nil?
	case *graphqlc.ValueDescriptorProto_EnumValue:
		def = ast.NewEnumValue(&ast.EnumValue{
			Value: v.EnumValue.Value,
		})
	case *graphqlc.ValueDescriptorProto_ListValue:
		list := ast.NewListValue(&ast.ListValue{})
		for _, lvDesc := range v.ListValue.Values {
			lvDef, err := buildValue(lvDesc)
			if err != nil {
				return nil, err
			}
			list.Values = append(list.Values, lvDef)
		}
		def = list
	case *graphqlc.ValueDescriptorProto_ObjectValue:
		obj := ast.NewObjectValue(&ast.ObjectValue{})
		for _, fieldDesc := range v.ObjectValue.Fields {
			fvDef, err := buildValue(fieldDesc.Value)
			if err != nil {
				return nil, err
			}
			obj.Fields = append(obj.Fields, ast.NewObjectField(&ast.ObjectField{
				Name:  ast.NewName(&ast.Name{Value: fieldDesc.Name}),
				Value: fvDef,
			}))
		}
		def = obj
	default:
		return nil, fmt.Errorf("unknown value type %#v\n", desc.Value)
	}
	return def, nil
}

func graphqlEchoFileName(name string) string {
	if ext := path.Ext(name); ext == ".graphql" {
		name = name[:len(name)-len(ext)]
	}
	name += ".echo.graphql"

	return name
}
