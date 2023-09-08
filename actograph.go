package actograph

import (
	"context"
	"fmt"
	"log"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"

	"github.com/actord/actograph/directive"
)

var hardcodedDirectives = []string{"enumPrivacy", "enumVal"}

type Actograph struct {
	directiveDeclarations map[string]directive.Definition

	// fill definitions while parse "schema.graphql" file
	schema                 *ast.SchemaDefinition
	directiveDefinitions   map[string]*ast.DirectiveDefinition
	objectDefinitions      map[string]*ast.ObjectDefinition
	inputObjectDefinitions map[string]*ast.InputObjectDefinition
	enumDefinitions        map[string]*ast.EnumDefinition
	declaredScalars        map[string]ScalarDefinition // map name to description
	extensionDefinitions   map[string][]*ast.TypeExtensionDefinition

	// resulting objects, fill while making schema
	enums        map[string]*graphql.Enum
	objects      map[string]*graphql.Object
	inputObjects map[string]*graphql.InputObject
	scalars      map[string]*graphql.Scalar

	lazySchema           *graphql.Schema
	lazySchemaDirectives []directive.Directive
}

func (agh *Actograph) RegisterDirective(dir directive.Definition) error {
	if _, has := agh.directiveDeclarations[dir.Name()]; has {
		return fmt.Errorf("directive @%s already registered", dir.Name())
	}

	_, has := agh.directiveDefinitions[dir.Name()]
	if !has {
		return fmt.Errorf("directive @%s is not defined in schema", dir.Name())
	}

	// TODO: validate declared arguments, maybe

	agh.directiveDeclarations[dir.Name()] = dir
	return nil
}

func (agh *Actograph) RegisterDirectives(dirs ...directive.Definition) error {
	var err error
	for i, dir := range dirs {
		err = agh.RegisterDirective(dir)
		if err != nil {
			return fmt.Errorf("while registering directives at index '%d': %v", i, err)
		}
	}
	return nil
}

func (agh *Actograph) RegisterScalar(cfg ScalarConfig) error {
	newS := graphql.NewScalar(graphql.ScalarConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Serialize: func(value interface{}) interface{} {
			return cfg.Serialize(value)
		},
		ParseValue: func(value interface{}) interface{} {
			return cfg.ParseValue(value)
		},
		ParseLiteral: func(valueAST ast.Value) interface{} {
			return cfg.ParseLiteral(valueAST)
		},
	})
	agh.scalars[cfg.Name] = newS

	return nil
}

func (agh *Actograph) RegisterScalars(cfgs ...ScalarConfig) error {
	var err error
	for i, cfg := range cfgs {
		err = agh.RegisterScalar(cfg)
		if err != nil {
			return fmt.Errorf("while registering scalar at index '%d': %v", i, err)
		}
	}
	return nil
}

func (agh *Actograph) ConstructDirective(dir *ast.Directive, node ast.Node) (directive.Directive, error) {
	name := dir.Name.Value
	declaration, has := agh.directiveDeclarations[name]
	if !has {
		return nil, fmt.Errorf("undefined declaration for directive @%s", name)
	}

	dirDefinition := agh.directiveDefinitions[name]
	arguments := agh.makeDirectiveArguments(dir, dirDefinition)

	return declaration.Construct(arguments, node)
}

func (agh *Actograph) makeDirectiveArguments(dir *ast.Directive, dirDefinition *ast.DirectiveDefinition) directive.Arguments {
	arguments := directive.Arguments{}

	if dirDefinition == nil || dir == nil {
		panic(fmt.Errorf("no dir or definition: %v / %v", dir, dirDefinition))
	}

	for _, argDefinition := range dirDefinition.Arguments {
		if argDefinition.DefaultValue != nil {
			arguments[argDefinition.Name.Value] = argDefinition.DefaultValue
		}
	}

	for _, arg := range dir.Arguments {
		key := arg.Name.Value
		value := arg.Value
		arguments[key] = value
	}

	return arguments
}

func (agh *Actograph) Parse(graphqlFile []byte) error {
	astDoc, err := parser.Parse(parser.ParseParams{
		Source: &source.Source{
			Body: graphqlFile,
		},
	})
	if err != nil {
		return fmt.Errorf("err while parsing: %v", err)
	}

	for _, node := range astDoc.Definitions {
		switch node.GetKind() {
		case "DirectiveDefinition":
			n := node.(*ast.DirectiveDefinition)
			agh.addDirective(n)
		case "ObjectDefinition":
			n := node.(*ast.ObjectDefinition)
			agh.addObject(n)
		case "InputObjectDefinition":
			n := node.(*ast.InputObjectDefinition)
			agh.addInputObject(n)
		case "SchemaDefinition":
			n := node.(*ast.SchemaDefinition)
			agh.addSchema(n)
		case "EnumDefinition":
			n := node.(*ast.EnumDefinition)
			agh.addEnum(n)
		case "ScalarDefinition":
			n := node.(*ast.ScalarDefinition)
			agh.addScalar(n)
		case "TypeExtensionDefinition":
			n := node.(*ast.TypeExtensionDefinition)
			agh.addExtensionDefinition(n)
		default:
			panic(fmt.Errorf("unknown node kind: %s", node.GetKind()))
		}
	}

	return nil
}

func (agh *Actograph) Validate() error {
	// when schema created - validation already passed
	if agh.lazySchema != nil {
		return nil
	}
	_, err := agh.makeSchema()
	return err
}

func (agh *Actograph) Schema() (graphql.Schema, error) {
	if agh.lazySchema != nil {
		return *agh.lazySchema, nil
	}
	schema, err := agh.makeSchema()
	if err != nil {
		return graphql.Schema{}, err
	}
	agh.lazySchema = &schema
	return schema, nil
}

func (agh *Actograph) makeSchema() (graphql.Schema, error) {
	// check is all declared directions are defined
	for directiveDefinitionName := range agh.directiveDefinitions {
		if _, has := agh.directiveDeclarations[directiveDefinitionName]; !has {
			hasHardcoded := false
			for _, hardcodedDirective := range hardcodedDirectives {
				if hardcodedDirective == directiveDefinitionName {
					hasHardcoded = true
					break
				}
			}
			if hasHardcoded {
				continue
			}
			return graphql.Schema{}, fmt.Errorf("directive '%s' was declared in schema, but not registered", directiveDefinitionName)
		}
	}

	// make lazySchemaDirectives
	agh.lazySchemaDirectives = make([]directive.Directive, len(agh.schema.Directives))
	for i, dir := range agh.schema.Directives {
		var err error
		agh.lazySchemaDirectives[i], err = agh.ConstructDirective(dir, agh.schema)
		if err != nil {
			panic(fmt.Errorf("error when contructing directive: %v", err))
		}
	}

	for _, scalarDefinition := range agh.declaredScalars {
		if _, has := agh.scalars[scalarDefinition.Name]; !has {
			panic(fmt.Errorf("scalar '%s' was declared by not defined", scalarDefinition.Name))
		}
	}

	gconf := graphql.SchemaConfig{}
	agh.makeEmptyObjects()
	agh.fillCachedObjectsWithFields()

	var queryTypename, mutationTypename string
	for _, ot := range agh.schema.OperationTypes {
		switch ot.Operation {
		case "query":
			queryTypename = ot.Type.Name.Value
		case "mutation":
			mutationTypename = ot.Type.Name.Value
		default:
			log.Panicf("unknown operation type in schema definition: %s", ot.Operation)
		}
	}

	if queryTypename != "" {
		queryType, has := agh.objects[queryTypename]
		if !has {
			panic(fmt.Errorf("not found object declared as schema.query: %s", queryTypename))
		}
		gconf.Query = queryType
	}

	if mutationTypename != "" {
		mutationType, has := agh.objects[mutationTypename]
		if !has {
			panic(fmt.Errorf("not found object declared as schema.query: %s", mutationTypename))
		}
		gconf.Mutation = mutationType
	}

	// TODO: implement subscription pls ^_^

	return graphql.NewSchema(gconf)
}

func (agh *Actograph) Do(request RequestQuery) (*Result, error) {
	schema, err := agh.Schema()
	if err != nil {
		return nil, fmt.Errorf("when taking schema: %v", err)
	}

	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}

	var rootObject map[string]interface{}
	if request.RootObject == nil {
		rootObject = map[string]interface{}{}
	} else {
		rootObject = request.RootObject
	}

	var resolvedValue interface{}
	resolvedValue, ctx, err = agh.executeDirectives(ctx, rootObject, rootObject, map[string]interface{}{}, agh.lazySchemaDirectives)
	// schema directives should return map[string]interface{}
	if resolvedValueMap, ok := resolvedValue.(map[string]interface{}); ok {
		rootObject = resolvedValueMap
	}

	result := graphql.Do(graphql.Params{
		Schema:         schema,
		RequestString:  request.RequestString,
		VariableValues: request.VariableValues,
		OperationName:  request.OperationName,
		RootObject:     rootObject,
		Context:        ctx,
	})
	if result == nil {
		return nil, fmt.Errorf("unknown result")
	}

	return &Result{
		Data:       result.Data,
		Errors:     result.Errors,
		Extensions: result.Extensions,
	}, nil
}

func (agh *Actograph) fillCachedObjectsWithFields() {

	for enumName, enumDefinition := range agh.enumDefinitions {
		var description string
		if enumDefinition.Description != nil {
			description = enumDefinition.Description.Value
		}

		values := graphql.EnumValueConfigMap{}
		for _, valueDefinition := range enumDefinition.Values {

			name := valueDefinition.Name.Value
			var valueDescription string
			if valueDefinition.Description != nil {
				valueDescription = valueDefinition.Description.Value
			}
			valCfg := &graphql.EnumValueConfig{
				Value:       name,
				Description: valueDescription,
			}

			if len(valueDefinition.Directives) > 0 {
				directiveExecutables := agh.makeDirectives(valueDefinition, valueDefinition.Directives)
				if err := agh.executeDefineDirectives(directiveExecutables, "*graphql.EnumValueConfig", valCfg); err != nil {
					panic(err)
				}
			}
			values[name] = valCfg
		}

		enum := graphql.NewEnum(graphql.EnumConfig{
			Name:        enumName,
			Values:      values,
			Description: description,
		})
		agh.enums[enumName] = enum
	}

	for objName, objDefinition := range agh.objectDefinitions {
		for _, fieldDefinition := range objDefinition.Fields {
			fieldName := fieldDefinition.Name.Value
			fieldConfig := agh.makeField(fieldDefinition)
			agh.objects[objName].AddFieldConfig(fieldName, fieldConfig)
		}

		if extended, has := agh.extensionDefinitions[objName]; has {
			for _, ext := range extended {
				for _, fieldDefinition := range ext.Definition.Fields {
					fieldName := fieldDefinition.Name.Value
					fieldConfig := agh.makeField(fieldDefinition)
					agh.objects[objName].AddFieldConfig(fieldName, fieldConfig)
				}
			}
		}
	}

	for inputObjName, inputObjDefinition := range agh.inputObjectDefinitions {
		for _, fieldDefinition := range inputObjDefinition.Fields {
			fieldName := fieldDefinition.Name.Value
			fieldConfig := agh.makeInputField(fieldDefinition)
			agh.inputObjects[inputObjName].AddFieldConfig(fieldName, fieldConfig)
		}
	}
}

func (agh *Actograph) makeInputField(fieldDefinition *ast.InputValueDefinition) *graphql.InputObjectFieldConfig {
	if len(fieldDefinition.Directives) > 0 {
		panic("input field directives are not supported yet")
	}

	var description string
	if fieldDefinition.Description != nil {
		description = fieldDefinition.Description.Value
	}

	return &graphql.InputObjectFieldConfig{
		Type:         agh.getType(fieldDefinition.Type),
		DefaultValue: fieldDefinition.DefaultValue,
		Description:  description,
	}
}

func (agh *Actograph) makeField(fieldDefinition *ast.FieldDefinition) *graphql.Field {
	var args graphql.FieldConfigArgument
	if len(fieldDefinition.Arguments) > 0 {
		args = graphql.FieldConfigArgument{}
		for _, argDefinition := range fieldDefinition.Arguments {
			name := argDefinition.Name.Value
			argType := agh.getType(argDefinition.Type)
			// TODO: maybe we should check as argType is scalar or inputObject, because objects is not allowed as arguments
			var defaultValue interface{}
			if argDefinition.DefaultValue != nil {
				defaultValue = argDefinition.DefaultValue.GetValue()
			}
			var description string
			if fieldDefinition.Description != nil {
				description = fieldDefinition.Description.Value
			}

			args[name] = &graphql.ArgumentConfig{
				Type:         argType,
				DefaultValue: defaultValue,
				Description:  description,
			}
		}
	}

	var description string
	if fieldDefinition.Description != nil {
		description = fieldDefinition.Description.Value
	}

	directiveExecutables := agh.makeDirectives(fieldDefinition, fieldDefinition.Directives)

	f := &graphql.Field{
		Name:        fieldDefinition.Name.Value,
		Type:        agh.getType(fieldDefinition.Type),
		Args:        args,
		Resolve:     agh.getFieldResolveFunc(directiveExecutables),
		Subscribe:   agh.getFieldSubscribeFunc(),
		Description: description,
	}

	if err := agh.executeDefineDirectives(directiveExecutables, "*graphql.Field", f); err != nil {
		panic(err)
	}

	return f
}

func (agh *Actograph) makeDirectives(node ast.Node, directiveDefinitions []*ast.Directive) []directive.Directive {
	directiveExecutables := make([]directive.Directive, len(directiveDefinitions))
	for i, directiveUsageDefinition := range directiveDefinitions {
		name := directiveUsageDefinition.Name.Value
		args := map[string]ast.Value{}
		for _, arg := range directiveUsageDefinition.Arguments {
			args[arg.Name.Value] = arg.Value
		}

		directiveDefinition := agh.directiveDefinitions[name]
		dirArguments := agh.makeDirectiveArguments(directiveUsageDefinition, directiveDefinition)
		directiveExecutable, err := agh.directiveDeclarations[name].Construct(dirArguments, node)
		if err != nil {
			panic(fmt.Errorf("cant construct directive usage for @%s: %v", name, err))
		}
		directiveExecutables[i] = directiveExecutable
	}
	return directiveExecutables
}

func (agh *Actograph) getType(typeDefinition ast.Type) graphql.Type {
	// unwrap if necessary
	switch typeDefinition.GetKind() {
	case "NonNull":
		return graphql.NewNonNull(agh.getType(typeDefinition.(*ast.NonNull).Type))
	case "List":
		return graphql.NewList(agh.getType(typeDefinition.(*ast.List).Type))
	case "Named":
		// Named are main case, so we expect to work with Named kind after switch
	default:
		panic(fmt.Errorf("unknown kind of typeDefinition: %s", typeDefinition.GetKind()))
	}

	name := typeDefinition.(*ast.Named).Name.Value

	if scalar, isScalar := agh.scalars[name]; isScalar {
		return scalar
	}

	if object, isObject := agh.objects[name]; isObject {
		return object
	}

	if inputObject, isInputObject := agh.inputObjects[name]; isInputObject {
		return inputObject
	}

	if enum, isEnum := agh.enums[name]; isEnum {
		if enum == nil {
			panic("fix this")
		}
		return enum
	}

	panic(fmt.Errorf("unknown named type: %s", name))
}

// makeEmptyObjects just will create references for necessary objects before we create types and fields for avoiding
// deadlock when create object that depends on object that depends on objects we're currently trying to create
func (agh *Actograph) makeEmptyObjects() {
	// TODO: make sure its necessary
	for name := range agh.enumDefinitions {
		agh.enums[name] = nil
	}

	for name, objDefinition := range agh.objectDefinitions {
		var description string
		if objDefinition.Description != nil {
			description = objDefinition.Description.Value
		}
		obj := graphql.NewObject(graphql.ObjectConfig{
			Name:        name,
			IsTypeOf:    nil,
			Fields:      graphql.Fields{},
			Description: description,
		})
		agh.objects[name] = obj
	}

	for name, objDefinition := range agh.inputObjectDefinitions {
		var description string
		if objDefinition.Description != nil {
			description = objDefinition.Description.Value
		}
		obj := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:        name,
			Fields:      graphql.InputObjectConfigFieldMap{},
			Description: description,
		})
		agh.inputObjects[name] = obj
	}
}

func (agh *Actograph) addDirective(n *ast.DirectiveDefinition) {
	name := n.Name.Value
	if _, has := agh.directiveDefinitions[name]; has {
		log.Panicf("directive with name '%s' already defined", name)
	}
	agh.directiveDefinitions[name] = n
}

func (agh *Actograph) addObject(n *ast.ObjectDefinition) {
	name := n.Name.Value
	if _, has := agh.objectDefinitions[name]; has {
		log.Panicf("object with name '%s' already defined", name)
	}
	agh.objectDefinitions[name] = n
}

func (agh *Actograph) addInputObject(n *ast.InputObjectDefinition) {
	name := n.Name.Value
	if _, has := agh.inputObjectDefinitions[name]; has {
		log.Panicf("input object with name '%s' already defined", name)
	}
	agh.inputObjectDefinitions[name] = n
}

func (agh *Actograph) addSchema(node *ast.SchemaDefinition) {
	if agh.schema != nil {
		panic("schema already defined")
	}
	agh.schema = node
}

func (agh *Actograph) addEnum(node *ast.EnumDefinition) {
	name := node.Name.Value
	if _, has := agh.enumDefinitions[name]; has {
		log.Panicf("enum with name '%s' already defined", name)
	}
	agh.enumDefinitions[name] = node
}

func (agh *Actograph) addScalar(node *ast.ScalarDefinition) {
	// TODO: implement scalar directives when found use cases :)
	if len(node.Directives) > 0 {
		panic("directives under scalar is not implemented yet")
	}
	name := node.Name.Value
	var description string
	if node.Description != nil {
		description = node.Description.Value
	}

	if _, has := agh.declaredScalars[name]; has {
		panic(fmt.Errorf("scalar '%s' already declared", name))
	}

	agh.declaredScalars[name] = ScalarDefinition{
		Name:        name,
		Description: description,
	}
}

func (agh *Actograph) addExtensionDefinition(node *ast.TypeExtensionDefinition) {
	name := node.Definition.Name.Value
	agh.extensionDefinitions[name] = append(agh.extensionDefinitions[name], node)
}

func (agh *Actograph) executeDirectives(
	ctx context.Context,
	source interface{},
	resolvedValue interface{},
	fieldArgs map[string]interface{},
	directives []directive.Directive,
) (interface{}, context.Context, error) {
	var err error
	for _, dir := range directives {
		resolvedValue, ctx, err = dir.Execute(ctx, source, resolvedValue, fieldArgs)
		if err != nil {
			if err == directive.ErrStopExecutionWithoutError {
				err = nil
			}
			break
		}
	}

	return resolvedValue, ctx, err
}

func (agh *Actograph) executeDefineDirectives(directives []directive.Directive, kind string, obj interface{}) error {
	var err error
	for i, dir := range directives {
		err = dir.Define(kind, obj)
		if err != nil {
			return fmt.Errorf("when executeDefine in directive #%d: %v", i, err)
		}
	}
	return nil
}
