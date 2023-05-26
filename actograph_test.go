package actograph_test

import (
	"encoding/json"
	"fmt"
	"github.com/actord/actograph/examples/scalars"
	"log"
	"testing"

	"github.com/actord/actograph"
	"github.com/actord/actograph/directive"
	"github.com/actord/actograph/examples/directives"
)

const exampleDirectives = "./examples/schema/directives.graphql"
const simpleSchema = "./examples/schema/simple.graphql"
const withUndefinedDirectiveSchema = "./examples/schema/testUndefinedDirective.graphql"
const testContextSchema = "./examples/schema/testContext.graphql"
const testScalarSchema = "./examples/schema/testScalar.graphql"
const testInputType = "./examples/schema/testInputType.graphql"
const testDefineDirectivesSchema = "./examples/schema/testDefineDirectives.graphql"

// Test todo:
//  - check is Enum definition without @enumPrivacy directive fired error

func TestErrorUndefinedDirective(t *testing.T) {
	_, err := getGQLSchema(withUndefinedDirectiveSchema)
	if err == nil {
		t.Fatalf("error should happend")
	}
	log.Println("error expected: ", err)
}

func TestSimpleWorkflow(t *testing.T) {
	gscm, err := getGQLSchema(simpleSchema)
	if err != nil {
		t.Fatalf("error when creating schema: %v", err)
	}

	// we can safely ignore error, because its schema validation error only
	result, _ := gscm.Do(actograph.RequestQuery{
		RequestString: `query Test { hello }`,
	})

	hello, ok := result.Data.(map[string]interface{})["hello"].(string)
	if !ok {
		t.Fatalf("!ok")
	}
	if hello != "world" {
		t.Fatalf("hello != world")
	}
}

func TestContextWorkflow(t *testing.T) {
	gscm, err := getGQLSchema(testContextSchema)
	if err != nil {
		t.Fatalf("error when creating schema: %v", err)
	}

	// we can safely ignore error, because its schema validation error only
	result, _ := gscm.Do(actograph.RequestQuery{
		RequestString: `query Test {
			test_string
			test_root
			test_args(arg_key: "test_args_value")
			global_set_context
		}`,
		RootObject: map[string]interface{}{
			"key_in_root_obj": "this is from_root key in root object",
		},
	})

	if result.Data == nil {
		errors, _ := json.Marshal(result.Errors)
		log.Println("result:", string(errors))
		t.Fatalf("result.Data is nil")
	}

	testString, ok := result.Data.(map[string]interface{})["test_string"].(string)
	if !ok || testString != "this is @setContext string" {
		t.Fatalf("test_string != 'this is @setContext string'")
	}

	testRoot, ok := result.Data.(map[string]interface{})["test_root"].(string)
	if !ok || testRoot != "this is from_root key in root object" {
		t.Fatalf("test_root != 'this is from_root key in root object'")
	}

	testArgs, ok := result.Data.(map[string]interface{})["test_args"].(string)
	if !ok || testArgs != "test_args_value" {
		t.Fatalf("test_args != 'test_args_value'")
	}

	testGlobalSetContext, ok := result.Data.(map[string]interface{})["global_set_context"]
	if !ok || testGlobalSetContext != nil {
		t.Fatalf("global_set_context != nil")
	}

}

func TestScalar(t *testing.T) {
	gscm, err := getGQLSchema(testScalarSchema)
	if err != nil {
		t.Fatalf("error when creating schema: %v", err)
	}

	// we can safely ignore error, because its schema validation error only
	result, _ := gscm.Do(actograph.RequestQuery{
		RequestString: `query Test($val: DoubleString!) {
			serializeValue
			parseLiteral:parse(arg: "provided in argument!")
			parseValue:parse(arg: $val)
		}`,
		VariableValues: map[string]interface{}{
			"val": "provided in values!",
		},
	})

	serializeValue, ok := result.Data.(map[string]interface{})["serializeValue"].(string)
	if !ok {
		t.Fatalf("!ok")
	}
	if serializeValue != "this is resolved string!this is resolved string!" {
		t.Fatalf("serializedValue != this is resolved string!this is resolved string!")
	}

	parseLiteral, ok := result.Data.(map[string]interface{})["parseLiteral"].(string)
	if !ok {
		t.Fatalf("!ok")
	}
	if parseLiteral != "provided in argument!provided in argument!" {
		t.Fatalf("parsedValue != provided in argument!provided in argument!")
	}

	parseValue, ok := result.Data.(map[string]interface{})["parseValue"].(string)
	if !ok {
		t.Fatalf("!ok")
	}
	if parseValue != "provided in values!provided in values!" {
		t.Fatalf("parsedValue != provided in values!provided in values!")
	}
}

func TestInputObject(t *testing.T) {
	gscm, err := getGQLSchema(testInputType)
	if err != nil {
		t.Fatalf("error when creating schema: %v", err)
	}

	result, _ := gscm.Do(actograph.RequestQuery{
		RequestString: `query Test($arg: InputType!) {
			test(arg: $arg) {
				field1
				field2
				# fieldEnum
			}
		}`,
		VariableValues: map[string]interface{}{
			"arg": map[string]interface{}{
				"field1": "value1",
				"field2": "value2",
				//"fieldEnum": "VALUE2",
			},
		},
	})
	log.Println("result", result)

	f1, ok1 := result.Data.(map[string]interface{})["test"].(map[string]interface{})["field1"].(string)
	f2, ok2 := result.Data.(map[string]interface{})["test"].(map[string]interface{})["field2"].(string)
	//fEnum, okEnum := result.Data.(map[string]interface{})["test"].(map[string]interface{})["fieldEnum"].(string)
	if !ok1 || !ok2 /*|| !okEnum*/ {
		t.Fatalf("field1 or field2 or fieldEnum not presented in OutputType")
	}

	if f1 != "value1" || f2 != "value2" /*|| fEnum != "VALUE2"*/ {
		t.Fatalf("field1 or field2 or fieldEnum has unexpected value")
	}
}

func TestDefineDirectives(t *testing.T) {
	introspectionQuery := `
		{
		  __type(name: "Enum") {
              __typename
			  name
			  kind
			  fields(includeDeprecated: true) {
				name
				isDeprecated
				deprecationReason
				description
			  }
			}
          deprecatedField
		}
	`
	gscm, err := getGQLSchema(testDefineDirectivesSchema)
	if err != nil {
		t.Fatalf("error when creating schema: %v", err)
	}

	result, _ := gscm.Do(actograph.RequestQuery{
		RequestString: introspectionQuery,
	})
	log.Println("result", result)
}

func getGQLSchema(filename string) (*actograph.Actograph, error) {
	agh, err := actograph.NewActographFiles(filename, exampleDirectives)
	if err != nil {
		return nil, fmt.Errorf("when parse file: %v", err)
	}

	// RegisterDirectives before parse
	if err := agh.RegisterDirectives(
		// todo: move default directives elsewhere
		directive.NewDirectiveDefinition("deprecated", directive.NewDeprecated),
		directive.NewDirectiveDefinition("_value", directive.NewValue),

		directive.NewDirectiveDefinition("resolveString", directives.NewDirectiveResolveString),
		directive.NewDirectiveDefinition("resolveArg", directives.NewDirectiveResolveArg),
		directive.NewDirectiveDefinition("setContext", directives.NewDirectiveSetContext),
		directive.NewDirectiveDefinition("getContext", directives.NewDirectiveGetContext),
		directive.NewDirectiveDefinition("expect", directives.NewDirectiveExpect),
	); err != nil {
		return nil, fmt.Errorf("when registering directives: %v", err)
	}

	agh.RegisterScalar(scalars.DoubleStringScalarConfig)

	if err := agh.Validate(); err != nil {
		return nil, fmt.Errorf("when validating schema: %v", err)
	}
	return agh, nil
}
