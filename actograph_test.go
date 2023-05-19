package actograph_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"testing"

	"github.com/actord/actograph"
	"github.com/actord/actograph/directive"
	"github.com/actord/actograph/examples/directives"
)

const exampleDirectives = "./examples/schema/directives.graphql"
const simpleSchema = "./examples/schema/simple.graphql"
const withUndefinedDirectiveSchema = "./examples/schema/testUndefinedDirective.graphql"
const testContextSchema = "./examples/schema/testContext.graphql"

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

func getGQLSchema(filename string) (*actograph.Actograph, error) {
	gqlSchemaData, err := joinFiles(filename, exampleDirectives)
	gscm, err := actograph.NewActographBytes(gqlSchemaData)
	if err != nil {
		return nil, fmt.Errorf("when parse file: %v", err)
	}
	// RegisterDirectives before parse
	if err := gscm.RegisterDirectives(
		directive.NewDirectiveDefinition("resolveString", directives.NewDirectiveResolveString),
		directive.NewDirectiveDefinition("setContext", directives.NewDirectiveSetContext),
		directive.NewDirectiveDefinition("getContext", directives.NewDirectiveGetContext),
	); err != nil {
		return nil, fmt.Errorf("when registering directives: %v", err)
	}

	if err := gscm.Validate(); err != nil {
		return nil, fmt.Errorf("when validating schema: %v", err)
	}
	return gscm, nil
}

func joinFiles(filenames ...string) ([]byte, error) {
	files := make([]io.Reader, len(filenames))
	for i, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("when reading schema in file %s: %v", filename, err)
		}
		files[i] = f
	}

	return io.ReadAll(io.MultiReader(files...))
}
