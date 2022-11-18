package graphql

import (
	"errors"
	"fmt"
	"log"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/parser"
)

func BuildSchema(source string) (*Schema, error) {
	astDoc, err := parser.Parse(parser.ParseParams{
		Source:  source,
		Options: parser.ParseOptions{},
	})
	if err != nil {
		log.Printf("Error with graphql-go parser.Parse: %v", err)
		return nil, err
	}

	schema, err := BuildAstSchema(astDoc)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

func BuildAstSchema(documentNode *ast.Document) (*Schema, error) {
	if documentNode == nil || documentNode.Kind != kinds.Document {
		return nil, errors.New("Must provide valid Document AST.")
	}
	// Get standard types we want in the schema
	stdTypeMap := map[string]Type{}
	for _, ttype := range append(GetIntrospectionTypes(), getSpecifiedScalarTypes()...) {
		stdTypeMap[ttype.Name()] = ttype
	}

	builder := SchemaConfigBuilder{
		typeExtensionsMap: make(map[string][]*ast.TypeExtensionDefinition),
		typeMap:           make(map[string]Type),
		stdTypeMap:        stdTypeMap,
	}

	config, err := builder.buildSchemaConfig(documentNode)

	if err != nil {
		return nil, err
	}

	schema, err := NewSchema(*config)
	if err != nil {
		log.Printf("Error with graphql-go NewSchema: %v", err)
		return nil, err
	}

	return &schema, nil
}

// graphql-go doesn't have an interface for type definitions with names, even though many
// type definitions support a GetName() function. We need to be able to call GetName(),
// so this custom interface allows us to cast interfaces to such a generic type.
// TODO: Add a similar interface to graphql-go
type NamedTypeDefinition interface {
	ast.TypeDefinition
	GetName() *ast.Name
}

type SchemaConfigBuilder struct {
	typeExtensionsMap map[string][]*ast.TypeExtensionDefinition
	typeMap           map[string]Type
	stdTypeMap        map[string]Type
}

// Convert a graphql-go *ast.Document of a schema to a graphql-go *graphql.SchemaConfig
func (c *SchemaConfigBuilder) buildSchemaConfig(documentAST *ast.Document) (*SchemaConfig, error) {
	schemaConfig := SchemaConfig{}
	typeDefs := []interface{}{}
	directiveDefs := []*ast.DirectiveDefinition{}
	unionDefs := []*ast.UnionDefinition{}
	var schemaDef *ast.SchemaDefinition

	// Iterate over all definitions in the ast document, grouping similar definitions to handle
	// each kind separately
	for _, def := range documentAST.Definitions {
		switch node := def.(type) {
		case *ast.SchemaDefinition:
			schemaDef = node
		case *ast.TypeExtensionDefinition:
			extendedTypeName := node.Definition.Name.Value
			if v, ok := c.typeExtensionsMap[extendedTypeName]; ok {
				c.typeExtensionsMap[extendedTypeName] = append(v, node)
			} else {
				c.typeExtensionsMap[extendedTypeName] = []*ast.TypeExtensionDefinition{node}
			}
		case *ast.DirectiveDefinition:
			directiveDefs = append(directiveDefs, node)
		case *ast.ObjectDefinition, *ast.InterfaceDefinition, *ast.EnumDefinition, *ast.ScalarDefinition, *ast.InputObjectDefinition:
			typeDefs = append(typeDefs, node)
		// See below TODO (ECO-3253) for why we are keeping track of unions separately
		case *ast.UnionDefinition:
			unionDefs = append(unionDefs, node)
		}
	}

	if len(typeDefs) == 0 && len(c.typeExtensionsMap) == 0 && len(directiveDefs) == 0 && schemaDef == nil {
		return &schemaConfig, nil
	}

	// TODO (ECO-3253): In a GraphQL SDL, Unions may contain types that have yet to be defined,
	// which means when we are creating a graphql.Union from an ast.UnionDefinition, we may not know
	// about all of the Union's type definitions (ast.TypeDefinitions) yet, and so we haven't
	// converted them to graphql.Types. This workaround creates graphql.Union types last so that we
	// _do_ know other types.
	//
	// In graphql-js, the types field of the corresponding GraphQLUnionTypeConfig is a
	// function/thunk, which solves the problem of not knowing all union types (because the function
	// is not called until after all types have been collected/converted). We should update
	// graphql-go to have this as well.
	for _, unionDef := range unionDefs {
		typeDefs = append(typeDefs, unionDef)
	}

	// convert all ast.TypeDefinitions to Types
	for _, typeNode := range typeDefs {
		if namedNode, ok := typeNode.(NamedTypeDefinition); ok {
			name := namedNode.GetName().Value
			// If it's a standard type, use the corresponding Type already in graphql-go
			if stdType, ok := c.stdTypeMap[name]; ok {
				c.typeMap[name] = stdType
			} else {
				// Else, we build the type from the typedef
				builtType, err := c.buildType(typeNode)
				if err != nil {
					return nil, err
				}

				if ttype, ok := builtType.(Type); ok {
					c.typeMap[name] = ttype
				} else {
					return nil, errors.New(fmt.Sprintf("SchemaConfigBuilder.buildType did not return a Type: %v", builtType))
				}
			}
		}
	}

	//  Add the converted types to the schema config
	schemaConfig.Types = []Type{}
	for name, ttype := range c.typeMap {
		schemaConfig.Types = append(schemaConfig.Types, ttype)

		// Find the special Query/Mutations and add those to the schemaConfig separately
		if name == "Query" {
			if objectType, ok := ttype.(*Object); ok {
				schemaConfig.Query = objectType
			}
		} else if name == "Mutation" {
			if objectType, ok := ttype.(*Object); ok {
				schemaConfig.Mutation = objectType
			}
		}
		// TODO (ECO-3254): Add subscriptions (not urgent because subscriptions are not in federation and we don't use them at Square)
	}

	// If there is a top-level schema definition, replace the Query/Mutation/Subscription types with those specified
	if schemaDef != nil {
		schemaOperationTypes, err := c.getSchemaOperationTypes(schemaDef)
		if err != nil {
			return nil, err
		}
		if schemaOperationTypes.Query != nil {
			schemaConfig.Query = schemaOperationTypes.Query
		}
		if schemaOperationTypes.Mutation != nil {
			schemaConfig.Mutation = schemaOperationTypes.Mutation
		}
		if schemaOperationTypes.Subscription != nil {
			schemaConfig.Subscription = schemaOperationTypes.Subscription
		}
	}

	// Convert ast.DirectiveDefinitions to Directive types
	schemaConfig.Directives = []*Directive{}
	for _, d := range directiveDefs {
		directiveType, err := c.buildDirective(d)
		if err != nil {
			return nil, err
		}

		schemaConfig.Directives = append(schemaConfig.Directives, directiveType)
	}

	// If specified directives were not explicitly declared, also add them to the schema
	// See TODO (ECO-3255) for why we are also including the "specifiedBy" directive
	specifiedDirectives := append(SpecifiedDirectives, SpecifiedByDirective)
	for _, specifiedDirective := range specifiedDirectives {
		hasDiretive := false
		for _, declaredDirective := range schemaConfig.Directives {
			if declaredDirective.Name == specifiedDirective.Name {
				hasDiretive = true
				break
			}
		}
		if !hasDiretive {
			schemaConfig.Directives = append(schemaConfig.Directives, specifiedDirective)
		}
	}

	return &schemaConfig, nil
}

// Given an ast.TypeDefinition object, return a corresponding graphql.Type object
func (c *SchemaConfigBuilder) buildType(astNode interface{}) (interface{}, error) {
	switch node := astNode.(type) {
	case *ast.ObjectDefinition:
		description := ""
		if node.Description != nil {
			description = node.Description.Value
		}

		// Build fields
		fieldsThunk, err := c.buildFieldsThunk(node)
		if err != nil {
			return nil, err
		}

		return NewObject(ObjectConfig{
			Name:        node.Name.Value,
			Description: description,
			Interfaces:  c.buildInterfacesThunk(node),
			Fields:      fieldsThunk,
		}), nil

	case *ast.InterfaceDefinition:
		description := ""
		if node.Description != nil {
			description = node.Description.Value
		}

		fieldsThunk, err := c.buildFieldsThunk(node)
		if err != nil {
			return nil, err
		}

		return NewInterface(InterfaceConfig{
			Name:        node.Name.Value,
			Description: description,
			Fields:      fieldsThunk,
		}), nil

	case *ast.EnumDefinition:
		description := ""
		if node.Description != nil {
			description = node.Description.Value
		}

		enums := EnumValueConfigMap{}
		for _, enumValue := range node.Values {
			description := ""
			if enumValue.Description != nil {
				description = enumValue.Description.Value
			}

			enums[enumValue.Name.Value] = &EnumValueConfig{
				Description:       description,
				DeprecationReason: getDeprecationReason(node),
			}
		}

		return NewEnum(EnumConfig{
			Name:        node.Name.Value,
			Description: description,
			Values:      enums,
		}), nil

	case *ast.UnionDefinition:
		description := ""
		if node.Description != nil {
			description = node.Description.Value
		}

		types := []*Object{}
		for _, nodeType := range node.Types {
			nodeTypeaName := nodeType.Name.Value
			ttype, err := c.getNamedType(nodeTypeaName)
			if err != nil {
				return nil, err
			}

			objectType, ok := ttype.(*Object)
			if ok {
				types = append(types, objectType)
			} else {
				return nil, errors.New(fmt.Sprintf("Union type \"%s\" is not an Object", nodeTypeaName))
			}
		}

		return NewUnion(UnionConfig{
			Name:        node.Name.Value,
			Description: description,
			Types:       types,
			// ResolveType needs to be defined but we're not using it (because we're not executing against this schema)
			ResolveType: func(p ResolveTypeParams) *Object {
				return nil
			},
		}), nil

	case *ast.ScalarDefinition:
		description := ""
		if node.Description != nil {
			description = node.Description.Value
		}

		return NewScalar(ScalarConfig{
			Name:        node.Name.Value,
			Description: description,
			// Custom scalars need to be defined but we're not using them (because we're not executing against this schema)
			Serialize: func(value interface{}) interface{} {
				return nil
			},
		}), nil

	case *ast.InputObjectDefinition:
		description := ""
		if node.Description != nil {
			description = node.Description.Value
		}

		fieldsThunk, err := c.buildInputObjectFieldsThunk(node)
		if err != nil {
			return nil, err
		}

		return NewInputObject(InputObjectConfig{
			Name:        node.Name.Value,
			Description: description,
			Fields:      fieldsThunk,
		}), nil
	}

	return nil, errors.New(fmt.Sprintf("Unexpected type definition node: %v", astNode))
}

func (c *SchemaConfigBuilder) buildDirective(directiveDef *ast.DirectiveDefinition) (*Directive, error) {
	locations := []string{}
	for _, l := range directiveDef.Locations {
		locations = append(locations, l.Value)
	}

	argMap, err := c.buildArgumentMap(directiveDef.Arguments)
	if err != nil {
		return nil, err
	}

	var description string
	if directiveDef.Description != nil {
		description = directiveDef.Description.Value
	}

	return NewDirective(DirectiveConfig{
		Name:        directiveDef.Name.Value,
		Description: description,
		Locations:   locations,
		Args:        argMap,
	}), nil
}

func (c *SchemaConfigBuilder) buildInterfacesThunk(node *ast.ObjectDefinition) InterfacesThunk {
	return func() []*Interface {
		interfaces := []*Interface{}

		// Add interfaces of the object
		for _, i := range node.Interfaces {
			namedInterface, err := c.getNamedType(i.Name.Value)
			if err != nil {
				// InterfaceThunks do not return errors, so panic here
				panic(err)
			}

			if convertedInterface, ok := namedInterface.(Interface); ok {
				interfaces = append(interfaces, &convertedInterface)
			}
		}

		// Add interfaces of extenions
		for _, en := range c.typeExtensionsMap[node.Name.Value] {
			for _, i := range en.Definition.Interfaces {
				namedInterface, err := c.getNamedType(i.Name.Value)
				if err != nil {
					// InterfaceThunks do not return errors, so panic here
					panic(err)
				}
				if convertedInterface, ok := namedInterface.(*Interface); ok {
					interfaces = append(interfaces, convertedInterface)
				}
			}
		}

		return interfaces
	}
}

func (c *SchemaConfigBuilder) buildFieldsThunk(astNode interface{}) (FieldsThunk, error) {
	fieldDefs := []*ast.FieldDefinition{}
	name := ""

	switch node := astNode.(type) {
	case *ast.ObjectDefinition:
		fieldDefs = append(fieldDefs, node.Fields...)
		name = node.Name.Value
	case *ast.InterfaceDefinition:
		fieldDefs = append(fieldDefs, node.Fields...)
		name = node.Name.Value
	default:
		return nil, errors.New("buildFieldsThunk called with unsupported node type")
	}

	for _, extensionNode := range c.typeExtensionsMap[name] {
		fieldDefs = append(fieldDefs, extensionNode.Definition.Fields...)
	}

	return func() Fields {
		fields, err := c.buildFieldMap(fieldDefs)
		if err != nil {
			// FieldThunks do not return errors, so panic here
			panic(err)
		}

		return fields
	}, nil
}

func (c *SchemaConfigBuilder) buildInputObjectFieldsThunk(node *ast.InputObjectDefinition) (InputObjectConfigFieldMapThunk, error) {
	return func() InputObjectConfigFieldMap {
		inputFieldMap := InputObjectConfigFieldMap{}
		for _, field := range node.Fields {
			fieldType, _ := c.getWrappedType(field.Type)
			if castedField, ok := fieldType.(Input); ok {
				description := ""
				if node.Description != nil {
					description = node.Description.Value
				}

				inputFieldMap[field.Name.Value] = &InputObjectFieldConfig{
					Type:         castedField,
					DefaultValue: valueFromAST(field.DefaultValue, castedField, nil),
					Description:  description,
				}
			}
		}
		return inputFieldMap
	}, nil
}

func (c *SchemaConfigBuilder) buildFieldMap(fieldDefs []*ast.FieldDefinition) (Fields, error) {
	fields := Fields{}

	for _, f := range fieldDefs {
		field := Field{
			Name: f.Name.Value,
		}

		if f.Description != nil {
			field.Description = f.Description.Value
		}

		wrapped, err := c.getWrappedType(f.Type)
		if err != nil {
			return nil, err
		}
		argMap, err := c.buildArgumentMap(f.Arguments)
		if err != nil {
			return nil, err
		}
		field.Args = argMap

		if castedWrapped, ok := wrapped.(Output); ok {
			field.Type = castedWrapped
		} else {
			return nil, errors.New(fmt.Sprintf("Casting %v to Output failed", wrapped))
		}

		fields[f.Name.Value] = &field
	}

	return fields, nil
}

type SchemaOperations struct {
	Query        *Object
	Mutation     *Object
	Subscription *Object
}

func (c *SchemaConfigBuilder) getSchemaOperationTypes(node *ast.SchemaDefinition) (SchemaOperations, error) {
	schemaOperations := SchemaOperations{}
	if node.OperationTypes != nil {
		for _, operationType := range node.OperationTypes {
			ttype, err := c.getNamedType(operationType.Type.Name.Value)
			if err != nil {
				return schemaOperations, err
			}

			objectType, ok := ttype.(*Object)
			if ok {
				if operationType.Operation == "query" {
					schemaOperations.Query = objectType
				} else if operationType.Operation == "mutation" {
					schemaOperations.Mutation = objectType
				} else if operationType.Operation == "subscription" {
					schemaOperations.Subscription = objectType
				}
			}
		}
	}
	return schemaOperations, nil
}

// Given a type's name, retrieves the type from the SchemaConfigBuilder's typeMap and returns it as a Named type
func (c *SchemaConfigBuilder) getNamedType(name string) (interface{}, error) {
	var ttype Type
	ok := false

	ttype, ok = c.stdTypeMap[name]
	if !ok {
		ttype, ok = c.typeMap[name]
	}

	if ok && ttype != nil {
		return GetNamed(ttype), nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unknown type: \"%s\"", name))
	}
}

// Converts a list of *ast.InputValueDefinitions to a map of *ArgumentConfigs
func (c *SchemaConfigBuilder) buildArgumentMap(args []*ast.InputValueDefinition) (FieldConfigArgument, error) {
	argMap := map[string]*ArgumentConfig{}

	for _, arg := range args {
		argType, err := c.getWrappedType(arg.Type)
		if err != nil {
			return argMap, err
		}
		if castedArg, ok := argType.(Input); ok {
			var description string
			if arg.Description != nil {
				description = arg.Description.Value
			}

			argMap[arg.Name.Value] = &ArgumentConfig{
				Type:         castedArg,
				Description:  description,
				DefaultValue: valueFromAST(arg.DefaultValue, castedArg, nil),
			}
		}
	}

	return argMap, nil
}

// Converts ast.List, ast.NonNull, and ast.Named definitions to their corresponding types
// Retrieves the actual type from the SchemaConfigBuilder's registered types by calling
// SchemaConfigBuilder.getNamedType
func (c *SchemaConfigBuilder) getWrappedType(ttype ast.Type) (interface{}, error) {
	switch t := ttype.(type) {
	case *ast.List:
		wrapped, err := c.getWrappedType(t.Type)
		if err != nil {
			return nil, err
		}
		if castedWrapped, ok := wrapped.(Type); ok {
			return NewList(castedWrapped), nil
		}
	case *ast.NonNull:
		wrapped, err := c.getWrappedType(t.Type)
		if err != nil {
			return nil, err
		}
		if castedWrapped, ok := wrapped.(Type); ok {
			return NewNonNull(castedWrapped), nil
		}
	case *ast.Named:
		return c.getNamedType(t.Name.Value)
	}

	return nil, errors.New(fmt.Sprintf("buildWrappedType received an ast.Type that was not a List, NonNull, or Named: %v", ttype))
}

// Certain TypeDefinitions have directives so we use the type to cast interfaces that correspond to
// those TypeDefinitions
type DefinitionWithDirectives struct {
	Directives []*ast.Directive
}

func getDeprecationReason(def interface{}) string {
	if d, ok := def.(DefinitionWithDirectives); ok {
		deprecated := getDirectiveValues(*DeprecatedDirective, d)
		if reason, ok := deprecated["reason"]; ok {
			return reason.(string)
		}
	}

	return ""
}

func getSpecifiedByUrl(def interface{}) string {
	if d, ok := def.(DefinitionWithDirectives); ok {
		deprecated := getDirectiveValues(*SpecifiedByDirective, d)
		if reason, ok := deprecated["reason"]; ok {
			return reason.(string)
		}
	}

	return ""
}

func getDirectiveValues(directive Directive, node DefinitionWithDirectives) map[string]interface{} {
	var directiveNode *ast.Directive
	for _, directiveDef := range node.Directives {
		if directive.Name == directiveDef.Name.Value {
			directiveNode = directiveDef
			break
		}
	}

	if directiveNode != nil {
		return getArgumentValues(directive.Args, directiveNode.Arguments, nil)
	}

	return nil
}
