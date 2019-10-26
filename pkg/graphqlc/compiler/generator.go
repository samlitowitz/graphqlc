package compiler

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/samlitowitz/graphqlc/pkg/graphqlc"
)

// because Google did it with protobuf/
// major * 10^6 + minor * 10^3 + patch
const GRAPHQLC_VERSION = 0000000

// A suffix string for alpha, beta or rc releases. Empty for stable releases.
const GRAPHQLC_VERSION_SUFFIX = "alpha"

type FileDescriptor struct {
	*graphqlc.FileDescriptorGraphql
	path    string
	doc     *ast.Document
	typeMap map[string]interface{}
}

type PluginMeta struct {
	Params, Path string
}

type Generator struct {
	*graphqlc.Generator

	PluginParams map[string]*PluginMeta // Map from plugin suffix to parameters

	genFiles []*FileDescriptor // Files to be generated
	file     *FileDescriptor   // File we are compiling now
}

func New() *Generator {
	g := new(Generator)
	g.Generator = graphqlc.New()
	g.LogPrefix = "graphqlc"
	return g
}

func (g *Generator) CommandLineArguments(arguments []string) {
	g.PluginParams = make(map[string]*PluginMeta)
	g.genFiles = make([]*FileDescriptor, 0)

	for _, arg := range arguments {
		if arg[:2] == "--" {
			suffix, params, path := parsePluginArgument(arg[2:])
			g.PluginParams[suffix] = &PluginMeta{Params: params, Path: path}
		} else {
			files, err := filepath.Glob(arg)
			if err != nil {
				g.Error(err)
			}
			for _, file := range files {
				g.genFiles = append(g.genFiles, &FileDescriptor{
					FileDescriptorGraphql: &graphqlc.FileDescriptorGraphql{
						Name: file,
					},
				})
			}
		}
	}
}

func (g *Generator) BuildTypeMap() {
	for _, fd := range g.genFiles {
		data, err := ioutil.ReadFile(fd.Name)
		if err != nil {
			g.Error(err)
		}
		doc, err := parser.Parse(parser.ParseParams{
			Source: string(data),
		})
		if err != nil {
			g.Error(err)
		}
		fd.doc = doc
		err = buildFileTypeMap(fd)
		if err != nil {
			g.Error(err)
		}
	}
}

func (g *Generator) BuildTypes() {
	for _, fd := range g.genFiles {
		for _, node := range fd.doc.Definitions {
			switch def := node.(type) {
			case *ast.SchemaDefinition:
				desc := fd.typeMap[def.Kind].(*graphqlc.SchemaDescriptorProto)
				err := buildSchemaDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.FileDescriptorGraphql.Schema = desc
			case *ast.ScalarDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.ScalarTypeDefinitionDescriptorProto)
				err := buildScalarsDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.Scalars = append(fd.Scalars, desc)
			case *ast.ObjectDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.ObjectTypeDefinitionDescriptorProto)
				err := buildObjectDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.Objects = append(fd.Objects, desc)
			case *ast.InterfaceDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.InterfaceTypeDefinitionDescriptorProto)
				err := buildInterfaceDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.Interfaces = append(fd.Interfaces, desc)
			case *ast.UnionDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.UnionTypeDefinitionDescriptorProto)
				err := buildUnionDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.Unions = append(fd.Unions, desc)
			case *ast.EnumDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.EnumTypeDefinitionDescriptorProto)
				err := buildEnumDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.Enums = append(fd.Enums, desc)
			case *ast.InputObjectDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.InputObjectTypeDefinitionDescriptorProto)
				err := buildInputObjectDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.InputObjects = append(fd.InputObjects, desc)
			case *ast.DirectiveDefinition:
				desc := fd.typeMap[def.Name.Value].(*graphqlc.DirectiveDefinitionDescriptorProto)
				err := buildDirectiveDefinitionDescriptor(fd, desc, def)
				if err != nil {
					g.Error(err)
				}
				fd.Directives = append(fd.Directives, desc)
			default:
				g.Error(fmt.Errorf("%s: unknown type %T", fd.Name, node))
			}
		}

		if fd.FileDescriptorGraphql.Schema == nil {
			fd.FileDescriptorGraphql.Schema = &graphqlc.SchemaDescriptorProto{}
			if desc, ok := fd.typeMap["Query"]; ok {
				fd.FileDescriptorGraphql.Schema.Query = desc.(*graphqlc.ObjectTypeDefinitionDescriptorProto)
			} else {
				queryDef := &graphqlc.ObjectTypeDefinitionDescriptorProto{
					Name:       "Query",
					Implements: []*graphqlc.InterfaceTypeDefinitionDescriptorProto{},
					Directives: []*graphqlc.DirectiveDescriptorProto{},
					Fields:     []*graphqlc.FieldDefinitionDescriptorProto{},
				}
				fd.FileDescriptorGraphql.Objects = append(fd.FileDescriptorGraphql.Objects, queryDef)
				fd.FileDescriptorGraphql.Schema.Query = queryDef
			}
			if desc, ok := fd.typeMap["Mutation"]; ok {
				fd.FileDescriptorGraphql.Schema.Mutation = desc.(*graphqlc.ObjectTypeDefinitionDescriptorProto)
			}
			if desc, ok := fd.typeMap["Subscription"]; ok {
				fd.FileDescriptorGraphql.Schema.Subscription = desc.(*graphqlc.ObjectTypeDefinitionDescriptorProto)
			}
		}
	}
}

func (g *Generator) GenerateAllFiles() {
	g.buildRequest()

	var stdout, stderr bytes.Buffer
	os.Setenv("PATH", os.Getenv("PATH")+":"+os.Getenv("GOPATH")+"/bin")

	for suffix, meta := range g.PluginParams {
		stdout.Reset()
		g.Request.Parameter = meta.Params

		data, err := proto.Marshal(g.Request)
		if err != nil {
			g.Error(err)
		}

		cmd := exec.Command("graphqlc-gen-" + suffix)
		cmd.Env = os.Environ()
		cmd.Stdin = bytes.NewReader(data)
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout

		if err := cmd.Run(); err != nil {
			g.Error(err, stderr.String())
		}

		err = proto.Unmarshal(stdout.Bytes(), g.Response)
		if err != nil {
			g.Error(err)
		}

		for i, file := range g.Response.File {
			switch {
			// Append to previous file
			case file.Name == "" && file.InsertionPoint == "":
				if i < 1 {
					g.Fail("unable to append to file, no previous file exists")
				}
				file.Name = g.Response.File[i-1].Name
				err := appendPreviousFile(meta.Path, file)
				if err != nil {
					g.Error(err)
				}
			// Write new file
			case file.Name != "" && file.InsertionPoint == "":
				err := writeNewFile(meta.Path, file)
				if err != nil {
					g.Error(err)
				}
			// Write insertion point
			case file.Name != "" && file.InsertionPoint != "":
				err := writeInsertionPoint(meta.Path, file)
				if err != nil {
					g.Error(err)
				}
			case file.Name == "" && file.InsertionPoint != "":
				g.Fail("insertion point defined, file name expected")
			}
		}
	}
}

func (g *Generator) buildRequest() {
	g.Request = new(graphqlc.CodeGeneratorRequest)
	g.Request.CompilerVersion = &graphqlc.Version{
		Major:  GRAPHQLC_VERSION / 1000000,
		Minor:  GRAPHQLC_VERSION / 1000 % 1000,
		Patch:  GRAPHQLC_VERSION % 1000,
		Suffix: GRAPHQLC_VERSION_SUFFIX,
	}
	for _, fd := range g.genFiles {
		g.Request.FileToGenerate = append(g.Request.FileToGenerate, fd.Name)
		g.Request.GraphqlFile = append(g.Request.GraphqlFile, fd.FileDescriptorGraphql)
	}
}

func buildFileTypeMap(fd *FileDescriptor) error {
	fd.typeMap = make(map[string]interface{})
	for _, node := range fd.doc.Definitions {
		switch def := node.(type) {
		case *ast.SchemaDefinition:
			if _, ok := fd.typeMap[def.Kind]; ok {
				return fmt.Errorf("multiple `schema` definition")
			}
			fd.typeMap[def.Kind] = &graphqlc.SchemaDescriptorProto{}

		case *ast.ScalarDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.ScalarTypeDefinitionDescriptorProto)
		case *ast.ObjectDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.ObjectTypeDefinitionDescriptorProto)
		case *ast.InterfaceDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.InterfaceTypeDefinitionDescriptorProto)
		case *ast.UnionDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.UnionTypeDefinitionDescriptorProto)
		case *ast.EnumDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.EnumTypeDefinitionDescriptorProto)
		case *ast.InputObjectDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.InputObjectTypeDefinitionDescriptorProto)
		case *ast.DirectiveDefinition:
			fd.typeMap[def.Name.Value] = new(graphqlc.DirectiveDefinitionDescriptorProto)
		default:
			return fmt.Errorf("%s: unknown type %T", fd.Name, node)
		}
	}
	return nil
}

// Top level definitions
func buildDirectiveDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.DirectiveDefinitionDescriptorProto, node *ast.DirectiveDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	for _, argDef := range node.Arguments {
		argDesc, err := buildInputValueDefinitionDescriptor(argDef)
		if err != nil {
			return err
		}
		desc.Arguments = append(desc.Arguments, argDesc)
	}

	for _, locDef := range node.Locations {
		if v, ok := graphqlc.ExecutableDirectiveLocation_value[locDef.Value]; ok {
			desc.Locations = append(desc.Locations, &graphqlc.DirectiveLocationDescriptorProto{
				Location: &graphqlc.DirectiveLocationDescriptorProto_ExecutableLocation{
					ExecutableLocation: graphqlc.ExecutableDirectiveLocation(v),
				},
			})
		} else if v, ok := graphqlc.TypeSystemDirectiveLocation_value[locDef.Value]; ok {
			desc.Locations = append(desc.Locations, &graphqlc.DirectiveLocationDescriptorProto{
				Location: &graphqlc.DirectiveLocationDescriptorProto_TypeSystemLocation{
					TypeSystemLocation: graphqlc.TypeSystemDirectiveLocation(v),
				},
			})
		} else {
			return fmt.Errorf("unknown directive loction %q", locDef.Value)
		}
	}

	return nil
}

func buildEnumDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.EnumTypeDefinitionDescriptorProto, node *ast.EnumDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs

	for _, enumValDef := range node.Values {
		directiveDescs, err := buildDirectiveDescriptors(enumValDef.Directives)
		if err != nil {
			return err
		}
		enumValDesc := &graphqlc.EnumValueDefinitionDescription{
			Value:      enumValDef.Name.Value,
			Directives: directiveDescs,
		}
		if enumValDef.Description != nil {
			enumValDesc.Description = enumValDef.Description.Value
		}
		desc.Values = append(desc.Values, enumValDesc)
	}

	return nil
}

func buildInputObjectDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.InputObjectTypeDefinitionDescriptorProto, node *ast.InputObjectDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs

	for _, fieldDef := range node.Fields {
		fieldDesc, err := buildInputValueDefinitionDescriptor(fieldDef)
		if err != nil {
			return err
		}
		desc.Fields = append(desc.Fields, fieldDesc)

	}

	return nil
}

func buildInterfaceDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.InterfaceTypeDefinitionDescriptorProto, node *ast.InterfaceDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs

	for _, fieldDef := range node.Fields {
		fieldDesc, err := buildFieldDefinitionDescriptor(fieldDef)
		if err != nil {
			return err
		}
		desc.Fields = append(desc.Fields, fieldDesc)

	}

	return nil
}

func buildObjectDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.ObjectTypeDefinitionDescriptorProto, node *ast.ObjectDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	for _, interfaceDef := range node.Interfaces {
		desc.Implements = append(desc.Implements, fd.typeMap[interfaceDef.Name.Value].(*graphqlc.InterfaceTypeDefinitionDescriptorProto))
	}

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs

	for _, fieldDef := range node.Fields {
		fieldDesc, err := buildFieldDefinitionDescriptor(fieldDef)
		if err != nil {
			return err
		}
		desc.Fields = append(desc.Fields, fieldDesc)

	}

	return nil
}

func buildScalarsDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.ScalarTypeDefinitionDescriptorProto, node *ast.ScalarDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs

	return nil
}

func buildSchemaDescriptor(fd *FileDescriptor, desc *graphqlc.SchemaDescriptorProto, node *ast.SchemaDefinition) error {
	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs
	for _, operationType := range node.OperationTypes {
		typeName := operationType.Type.Name.Value
		objectDesc := fd.typeMap[typeName].(*graphqlc.ObjectTypeDefinitionDescriptorProto)

		switch operationType.Operation {
		case "query":
			desc.Query = objectDesc
		case "mutation":
			desc.Mutation = objectDesc
		case "subscription":
			desc.Subscription = objectDesc
		}
	}
	return nil
}

func buildUnionDefinitionDescriptor(fd *FileDescriptor, desc *graphqlc.UnionTypeDefinitionDescriptorProto, node *ast.UnionDefinition) error {
	if node.Description != nil {
		desc.Description = node.Description.Value
	}
	desc.Name = node.Name.Value

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return err
	}
	desc.Directives = directiveDescs

	for _, memberDef := range node.Types {
		desc.MemberTypes = append(desc.MemberTypes, &graphqlc.NamedTypeDescriptorProto{Name: memberDef.Name.Value})
	}

	return nil
}

// Not top level definitions
func buildFieldDefinitionDescriptor(node *ast.FieldDefinition) (*graphqlc.FieldDefinitionDescriptorProto, error) {
	fieldDesc := &graphqlc.FieldDefinitionDescriptorProto{
		Name: node.Name.Value,
	}

	if node.Description != nil {
		fieldDesc.Description = node.Description.Value
	}

	for _, argument := range node.Arguments {
		if argument != nil {
			argumentDesc, err := buildInputValueDefinitionDescriptor(argument)
			if err != nil {
				return nil, err
			}
			fieldDesc.Arguments = append(fieldDesc.Arguments, argumentDesc)
		}
	}

	typDesc, err := buildTypeDescriptorProto(node.Type)
	if err != nil {
		return nil, err
	}
	fieldDesc.Type = &graphqlc.TypeDescriptorProto{Type: typDesc}

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return nil, err
	}
	fieldDesc.Directives = directiveDescs

	return fieldDesc, nil
}

func buildInputValueDefinitionDescriptor(node *ast.InputValueDefinition) (*graphqlc.InputValueDefinitionDescriptorProto, error) {
	inputValDesc := &graphqlc.InputValueDefinitionDescriptorProto{}

	if node.Name != nil {
		inputValDesc.Name = node.Name.Value
	}

	if node.Description != nil {
		inputValDesc.Description = node.Description.Value
	}

	typDesc, err := buildTypeDescriptorProto(node.Type)
	if err != nil {
		return nil, err
	}
	inputValDesc.Type = &graphqlc.TypeDescriptorProto{Type: typDesc}

	if node.DefaultValue != nil {
		valDesc, err := buildValueDescriptor(node.DefaultValue)
		if err != nil {
			return nil, err
		}
		inputValDesc.DefaultValue = &graphqlc.ValueDescriptorProto{Value: valDesc}
	}

	directiveDescs, err := buildDirectiveDescriptors(node.Directives)
	if err != nil {
		return nil, err
	}
	inputValDesc.Directives = directiveDescs

	return inputValDesc, nil
}

func buildTypeDescriptorProto(def ast.Type) (graphqlc.TypeDescriptorProto_Type, error) {
	switch typ := def.(type) {
	case *ast.Named:
		return &graphqlc.TypeDescriptorProto_NamedType{
			NamedType: &graphqlc.NamedTypeDescriptorProto{
				Name: typ.Name.Value,
			},
		}, nil
	case *ast.List:
		listTyp, err := buildTypeDescriptorProto(typ.Type)
		if err != nil {
			return nil, err
		}
		return &graphqlc.TypeDescriptorProto_ListType{
			ListType: &graphqlc.ListTypeDescriptorProto{
				Type: &graphqlc.TypeDescriptorProto{
					Type: listTyp,
				},
			},
		}, nil
	case *ast.NonNull:
		typDesc := &graphqlc.TypeDescriptorProto_NonNullType{
			NonNullType: &graphqlc.NonNullTypeDescriptorProto{},
		}
		switch typTypDef := (typ.Type).(type) {
		case *ast.Named:
			typDesc.NonNullType.Type = &graphqlc.NonNullTypeDescriptorProto_NamedType{
				NamedType: &graphqlc.NamedTypeDescriptorProto{
					Name: typTypDef.Name.Value,
				},
			}
		case *ast.List:
			listTyp, err := buildTypeDescriptorProto(typTypDef)
			if err != nil {
				return nil, err
			}
			typDesc.NonNullType.Type = &graphqlc.NonNullTypeDescriptorProto_ListType{
				ListType: &graphqlc.ListTypeDescriptorProto{
					Type: &graphqlc.TypeDescriptorProto{
						Type: listTyp,
					},
				},
			}
		}
		return typDesc, nil
	default:
		return nil, fmt.Errorf("unknown type %T", def)
	}
	return nil, nil
}

func buildDirectiveDescriptors(directives []*ast.Directive) ([]*graphqlc.DirectiveDescriptorProto, error) {
	var descriptors []*graphqlc.DirectiveDescriptorProto

	for _, def := range directives {
		desc, err := buildDirectiveDescriptor(def)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, desc)
	}

	return descriptors, nil
}

func buildDirectiveDescriptor(directive *ast.Directive) (*graphqlc.DirectiveDescriptorProto, error) {
	directiveDesc := &graphqlc.DirectiveDescriptorProto{
		Name: directive.Name.Value,
	}

	for _, argument := range directive.Arguments {
		argumentDesc, err := buildArgumentDescriptor(argument)
		if err != nil {
			return nil, err
		}
		directiveDesc.Arguments = append(directiveDesc.Arguments, argumentDesc)
	}

	return directiveDesc, nil
}

func buildArgumentDescriptor(argument *ast.Argument) (*graphqlc.ArgumentDescriptorProto, error) {
	argumentDef := &graphqlc.ArgumentDescriptorProto{
		Name:  argument.Name.Value,
		Value: &graphqlc.ValueDescriptorProto{},
	}

	val, err := buildValueDescriptor(argument.Value)
	if err != nil {
		return nil, err
	}
	argumentDef.Value.Value = val

	return argumentDef, nil
}

func buildValueDescriptor(value ast.Value) (graphqlc.ValueDescriptorProto_Value, error) {
	switch tVal := value.(type) {
	case *ast.Variable:
		return &graphqlc.ValueDescriptorProto_VariableValue{
			VariableValue: &graphqlc.VariableDescriptorProto{
				Name: tVal.Name.Value,
			},
		}, nil
	case *ast.IntValue:
		intVal, err := strconv.ParseInt(tVal.Value, 10, 32)
		if err != nil {
			return nil, err
		}
		return &graphqlc.ValueDescriptorProto_IntValue{
			IntValue: int32(intVal),
		}, nil
	case *ast.FloatValue:
		floatVal, err := strconv.ParseFloat(tVal.Value, 32)
		if err != nil {
			return nil, err
		}
		return &graphqlc.ValueDescriptorProto_FloatValue{
			FloatValue: float32(floatVal),
		}, nil
	case *ast.StringValue:
		return &graphqlc.ValueDescriptorProto_StringValue{
			StringValue: tVal.Value,
		}, nil
	case *ast.BooleanValue:
		return &graphqlc.ValueDescriptorProto_BooleanValue{
			BooleanValue: tVal.Value,
		}, nil
	case *ast.EnumValue:
		return &graphqlc.ValueDescriptorProto_EnumValue{
			EnumValue: &graphqlc.EnumValueDescriptorProto{
				Value: tVal.Value,
			},
		}, nil
	case *ast.ListValue:
		listValue := &graphqlc.ValueDescriptorProto_ListValue{
			ListValue: &graphqlc.ListValueDescriptorProto{},
		}
		for _, listV := range tVal.Values {
			newV, err := buildValueDescriptor(listV)
			if err != nil {
				return nil, err
			}
			listValue.ListValue.Values = append(listValue.ListValue.Values, &graphqlc.ValueDescriptorProto{Value: newV})
		}
		return listValue, nil
	case *ast.ObjectValue:
		objValue := &graphqlc.ValueDescriptorProto_ObjectValue{ObjectValue: &graphqlc.ObjectValueDescriptorProto{}}
		for _, objV := range tVal.Fields {
			newV, err := buildValueDescriptor(objV.Value)
			if err != nil {
				return nil, err
			}
			objValue.ObjectValue.Fields = append(objValue.ObjectValue.Fields, &graphqlc.ObjectFieldDescriptorProto{
				Name:  objV.Name.Value,
				Value: &graphqlc.ValueDescriptorProto{Value: newV},
			})
		}

		return &graphqlc.ValueDescriptorProto_ObjectValue{
			ObjectValue: &graphqlc.ObjectValueDescriptorProto{},
		}, nil
	}
	return nil, fmt.Errorf("unknown value type, %#v\n", value)
}

// Utility functions
func parsePluginArgument(arg string) (suffix, params, path string) {
	cLoc := strings.Index(arg, ":")
	eqLoc := strings.Index(arg, "_out=")
	if eqLoc == -1 {
		return "", "", ""
	}
	if cLoc == -1 {
		return arg[:eqLoc], "", arg[eqLoc+5:]
	}
	return arg[:eqLoc], arg[eqLoc+5 : cLoc], arg[cLoc+1:]
}

func appendPreviousFile(path string, file *graphqlc.CodeGeneratorResponse_File) error {
	qualifiedName := filepath.Join(path, file.Name)
	f, err := os.OpenFile(qualifiedName, os.O_RDWR|os.O_APPEND, 0755)
	if err != nil {
		return err
	}
	_, err = f.WriteString(file.Content)
	if err != nil {
		return err
	}
	return f.Close()
}

func writeInsertionPoint(path string, file *graphqlc.CodeGeneratorResponse_File) error {
	qualifiedName := filepath.Join(path, file.Name)
	r, err := os.OpenFile(qualifiedName, os.O_RDWR, 0755)
	if err != nil {
		return err
	}
	w, err := os.Create(qualifiedName + ".tmp")
	if err != nil {
		r.Close()
		return err
	}

	insertionPointActual := fmt.Sprintf(insertionPointText, file.InsertionPoint)
	reader := bufio.NewReader(r)
	for line, err := reader.ReadString('\n'); err == nil; line, err = reader.ReadString('\n') {
		if i := strings.Index(line, insertionPointActual); i != -1 {
			w.WriteString(line[:i+1] + file.Content + "\n")
		}
		w.WriteString(line)
	}
	if err != nil && err != io.EOF {
		r.Close()
		w.Close()
		return err
	}

	r.Close()
	w.Close()

	return os.Rename(qualifiedName+".tmp", qualifiedName)
}

func writeNewFile(path string, file *graphqlc.CodeGeneratorResponse_File) error {
	qualifiedName := filepath.Join(path, file.Name)
	err := os.MkdirAll(filepath.Dir(qualifiedName), 0755)
	if err != nil {
		return err
	}
	f, err := os.Create(qualifiedName)
	if err != nil {
		return err
	}
	_, err = f.WriteString(file.Content)
	if err != nil {
		return err
	}
	return f.Close()
}

const insertionPointText = " @@graphqlc_insertion_point(%s)"
