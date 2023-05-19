package graphscm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"

	"github.com/actord/actograph/directive"
)

var hardcodedDirectives = []string{"enumPrivacy", "enumVal"}

type GraphScm struct {
	directiveDeclarations map[string]directive.Definition

	// fill definitions while parse "schema.graphql" file
	schema                 *ast.SchemaDefinition
	directiveDefinitions   map[string]*ast.DirectiveDefinition
	objectDefinitions      map[string]*ast.ObjectDefinition
	inputObjectDefinitions map[string]*ast.InputObjectDefinition
	enums                  map[string]map[string]string // $enumName.$enumKey.@enumVal(str=$Value)

	// resulting objects, fill while making schema
	objects      map[string]*graphql.Object
	inputObjects map[string]*graphql.InputObject

	lazySchema           *graphql.Schema
	lazySchemaDirectives []directive.Directive
}

func NewGraphScm() *GraphScm {
	return &GraphScm{
		directiveDeclarations: map[string]directive.Definition{},

		directiveDefinitions:   map[string]*ast.DirectiveDefinition{},
		objectDefinitions:      map[string]*ast.ObjectDefinition{},
		inputObjectDefinitions: map[string]*ast.InputObjectDefinition{},
		enums:                  map[string]map[string]string{},

		objects:      map[string]*graphql.Object{},
		inputObjects: map[string]*graphql.InputObject{},

		lazySchemaDirectives: []directive.Directive{},
	}
}

func NewGraphScmBytes(graphqlFile []byte) (*GraphScm, error) {
	gscm := NewGraphScm()
	return gscm, gscm.Parse(graphqlFile)
}

func (scm *GraphScm) RegisterDirective(dir directive.Definition) error {
	if _, has := scm.directiveDeclarations[dir.Name()]; has {
		return fmt.Errorf("directive @%s already registered", dir.Name())
	}

	//_, has := scm.directiveDefinitions[dir.Name()]
	//if !has {
	//	return fmt.Errorf("directive @%s is not defined in schema", dir.Name())
	//}

	// TODO: validate declared arguments, maybe

	scm.directiveDeclarations[dir.Name()] = dir
	return nil
}

func (scm *GraphScm) RegisterDirectives(dirs ...directive.Definition) error {
	var err error
	for i, dir := range dirs {
		err = scm.RegisterDirective(dir)
		if err != nil {
			return fmt.Errorf("while registering directives at index '%d': %v", i, err)
		}
	}
	return nil
}

func (scm *GraphScm) ConstructDirective(dir *ast.Directive, node ast.Node) (directive.Directive, error) {
	name := dir.Name.Value
	declaration, has := scm.directiveDeclarations[name]
	if !has {
		return nil, fmt.Errorf("undefined declaration for directive @%s", name)
	}

	dirDefinition := scm.directiveDefinitions[name]
	arguments := scm.makeDirectiveArguments(dir, dirDefinition)

	return declaration.Construct(arguments, node)
}

func (scm *GraphScm) makeDirectiveArguments(dir *ast.Directive, dirDefinition *ast.DirectiveDefinition) directive.Arguments {
	arguments := directive.Arguments{}

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

func getEnumValue(enum ast.Value) ast.Value {
	if enum.GetKind() != "EnumValue" {
		panic("this is not EnumValue")
	}
	//enum.
	return enum
}

func (scm *GraphScm) Parse(graphqlFile []byte) error {
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
			scm.addDirective(n)
		case "ObjectDefinition":
			n := node.(*ast.ObjectDefinition)
			scm.addObject(n)
		case "InputObjectDefinition":
			n := node.(*ast.InputObjectDefinition)
			scm.addInputObject(n)
		case "SchemaDefinition":
			n := node.(*ast.SchemaDefinition)
			scm.addSchema(n)
		case "EnumDefinition":
			n := node.(*ast.EnumDefinition)
			scm.addEnum(n)

		default:
			panic(fmt.Errorf("unknown node kind: %s", node.GetKind()))
		}
	}

	return nil
}

func (scm *GraphScm) Validate() error {
	// when schema created - validation already passed
	if scm.lazySchema != nil {
		return nil
	}
	_, err := scm.makeSchema()
	return err
}

func (scm *GraphScm) Schema() (graphql.Schema, error) {
	if scm.lazySchema != nil {
		return *scm.lazySchema, nil
	}
	schema, err := scm.makeSchema()
	if err != nil {
		return graphql.Schema{}, err
	}
	scm.lazySchema = &schema
	return schema, nil
}

func (scm *GraphScm) makeSchema() (graphql.Schema, error) {
	// check is all declared directions are defined
	for directiveDefinitionName := range scm.directiveDefinitions {
		if _, has := scm.directiveDeclarations[directiveDefinitionName]; !has {
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
	scm.lazySchemaDirectives = make([]directive.Directive, len(scm.schema.Directives))
	for i, dir := range scm.schema.Directives {
		var err error
		scm.lazySchemaDirectives[i], err = scm.ConstructDirective(dir, scm.schema)
		if err != nil {
			log.Panicf("error when contructing directive: %v", err)
		}
	}

	gconf := graphql.SchemaConfig{}
	scm.makeEmptyObjects()
	scm.fillCachedObjectsWithFields()

	var queryTypename, mutationTypename string
	for _, ot := range scm.schema.OperationTypes {
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
		queryType, has := scm.objects[queryTypename]
		if !has {
			panic(fmt.Errorf("not found object declared as schema.query: %s", queryTypename))
		}
		gconf.Query = queryType
	}

	if mutationTypename != "" {
		mutationType, has := scm.objects[mutationTypename]
		if !has {
			panic(fmt.Errorf("not found object declared as schema.query: %s", mutationTypename))
		}
		gconf.Mutation = mutationType
	}

	// TODO: implement subscription pls ^_^

	return graphql.NewSchema(gconf)
}

func (scm *GraphScm) Do(request RequestQuery) (*Result, error) {
	schema, err := scm.Schema()
	if err != nil {
		return nil, fmt.Errorf("when taking schema: %v", err)
	}

	var ctx context.Context
	if request.Context == nil {
		ctx = context.Background()
	}

	var rootObject map[string]interface{}
	if request.RootObject == nil {
		rootObject = map[string]interface{}{}
	} else {
		rootObject = request.RootObject
	}

	var resolvedValue interface{}
	resolvedValue, ctx, err = scm.executeDirectives(ctx, rootObject, rootObject, map[string]interface{}{}, scm.lazySchemaDirectives)
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

	a, err := json.Marshal(result)
	fmt.Printf("err: %v\na: %s\n", err, string(a))

	return &Result{
		Data:       result.Data,
		Errors:     result.Errors,
		Extensions: result.Extensions,
	}, nil
}

func (scm *GraphScm) fillCachedObjectsWithFields() {
	for objName, objDefinition := range scm.objectDefinitions {
		for _, fieldDefinition := range objDefinition.Fields {
			fieldName := fieldDefinition.Name.Value
			fieldConfig := scm.makeField(fieldDefinition)
			scm.objects[objName].AddFieldConfig(fieldName, fieldConfig)
		}
	}
}

func (scm *GraphScm) makeField(fieldDefinition *ast.FieldDefinition) *graphql.Field {
	var args graphql.FieldConfigArgument
	if len(fieldDefinition.Arguments) > 0 {
		args = graphql.FieldConfigArgument{}
		for _, argDefinition := range fieldDefinition.Arguments {
			name := argDefinition.Name.Value
			argType := scm.getType(argDefinition.Type)
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

	var deprecationReason string
	// TODO: @deprecated directive

	//TODO``
	directiveExecutables := make([]directive.Directive, len(fieldDefinition.Directives))
	for i, directiveUsageDefinition := range fieldDefinition.Directives {
		name := directiveUsageDefinition.Name.Value
		args := map[string]ast.Value{}
		for _, arg := range directiveUsageDefinition.Arguments {
			args[arg.Name.Value] = arg.Value
		}
		directiveDefinition := scm.directiveDefinitions[name]
		dirArguments := scm.makeDirectiveArguments(directiveUsageDefinition, directiveDefinition)
		directiveExecutable, err := scm.directiveDeclarations[name].Construct(dirArguments, fieldDefinition)
		if err != nil {
			panic(fmt.Errorf("cant construct directive usage for field '%s' and @%s : %v", fieldDefinition.Name.Value, name, err))
		}
		directiveExecutables[i] = directiveExecutable
	}

	f := &graphql.Field{
		Name:              fieldDefinition.Name.Value,
		Type:              scm.getType(fieldDefinition.Type),
		Args:              args,
		Resolve:           scm.getFieldResolveFunc(directiveExecutables),
		Subscribe:         scm.getFieldSubscribeFunc(),
		DeprecationReason: deprecationReason,
		Description:       description,
	}

	return f
}

func (scm *GraphScm) getType(typeDefinition ast.Type) graphql.Type {
	// unwrap if necessary
	switch typeDefinition.GetKind() {
	case "NonNull":
		return graphql.NewNonNull(scm.getType(typeDefinition.(*ast.NonNull).Type))
	case "List":
		return graphql.NewList(scm.getType(typeDefinition.(*ast.List).Type))
	case "Named":
		// Named are main case, so we expect to work with Named kind after switch
	default:
		panic(fmt.Errorf("unknown kind of typeDefinition: %s", typeDefinition.GetKind()))
	}

	name := typeDefinition.(*ast.Named).Name.Value

	// check for scalar or return object
	scalars := map[string]graphql.Type{
		"String":   graphql.String,
		"Int":      graphql.Int,
		"Float":    graphql.Float,
		"Boolean":  graphql.Boolean,
		"ID":       graphql.ID,
		"DateTime": graphql.DateTime,
	}
	if scalar, isScalar := scalars[name]; isScalar {
		return scalar
	}

	if object, isObject := scm.objects[name]; isObject {
		return object
	}

	if inputObject, isInputObject := scm.inputObjects[name]; isInputObject {
		return inputObject
	}

	panic(fmt.Errorf("unknown named type: %s", name))
}

// makeEmptyObjects just will create references for necessary objects before we create types and fields for avoiding
// deadlock when create object that depends on object that depends on objects we're currently trying to create
func (scm *GraphScm) makeEmptyObjects() {
	for name, objDefinition := range scm.objectDefinitions {
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
		scm.objects[name] = obj
	}

	for name, objDefinition := range scm.inputObjectDefinitions {
		var description string
		if objDefinition.Description != nil {
			description = objDefinition.Description.Value
		}
		obj := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:        name,
			Fields:      graphql.InputObjectConfigFieldMap{},
			Description: description,
		})
		scm.inputObjects[name] = obj
	}
}

func (scm *GraphScm) addDirective(n *ast.DirectiveDefinition) {
	name := n.Name.Value
	if _, has := scm.directiveDefinitions[name]; has {
		log.Panicf("directive with name '%s' already defined", name)
	}
	scm.directiveDefinitions[name] = n
}

func (scm *GraphScm) addObject(n *ast.ObjectDefinition) {
	name := n.Name.Value
	if _, has := scm.objectDefinitions[name]; has {
		log.Panicf("object with name '%s' already defined", name)
	}
	scm.objectDefinitions[name] = n
}

func (scm *GraphScm) addInputObject(n *ast.InputObjectDefinition) {
	name := n.Name.Value
	if _, has := scm.inputObjectDefinitions[name]; has {
		log.Panicf("input object with name '%s' already defined", name)
	}
	scm.inputObjectDefinitions[name] = n
}

func (scm *GraphScm) addSchema(node *ast.SchemaDefinition) {
	if scm.schema != nil {
		panic("schema already defined")
	}
	scm.schema = node
}

func (scm *GraphScm) addEnum(node *ast.EnumDefinition) {
	name := node.Name.Value
	if _, has := scm.enums[name]; has {
		panic(fmt.Errorf("enum with name '%s' already defined", name))
	}
	allowInBackend := false
	allowInFrontend := false
	hasEnumPrivacyDirective := false
	for _, dir := range node.Directives {
		switch dir.Name.Value {
		case "enumPrivacy":
			hasEnumPrivacyDirective = true
			for _, arg := range dir.Arguments {
				switch arg.Name.Value {
				case "backend":
					allowInBackend = arg.Value.GetValue().(bool)
				case "frontend":
					allowInFrontend = arg.Value.GetValue().(bool)
				}
			}
		default:
			panic(fmt.Errorf("unknown directive '%s' on enum '%s'", name, dir.Name.Value))
		}
	}
	if !hasEnumPrivacyDirective {
		panic(fmt.Errorf("enum '%s' should has @enumPrivacy directive", name))
	}
	if allowInBackend == false && allowInFrontend == false {
		panic(fmt.Errorf("enum '%s' should be allowed for backend or frontend using @enumPrivacy directive	", name))
	}

	// TODO: move createDirectives from ast.Directive co common function, because its used a lot
	enumKeyToVal := map[string]string{}
	for _, value := range node.Values {
		key := value.Name.Value
		var val string
		for _, dir := range value.Directives {
			switch dir.Name.Value {
			case "enumVal":
				val = dir.Arguments[0].Value.GetValue().(string) // TODO: this is potential crash
			}
		}
		enumKeyToVal[key] = val
	}

	scm.enums[name] = enumKeyToVal
}

func (scm *GraphScm) executeDirectives(
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
