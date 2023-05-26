package directive

import (
	"context"
	"fmt"

	"github.com/graphql-go/graphql/language/ast"
)

// ErrStopExecutionWithoutError can be returned from directive.Execute and works like "break" in for loops but on directives chain
var ErrStopExecutionWithoutError = fmt.Errorf("stop execution without error")

type Arguments map[string]ast.Value

type Directive interface {
	// Execute defined directive (runtime)
	Execute(
		ctx context.Context,
		source interface{}, // parent object. Not map[string]interface{} for scalars resolvers or nil
		resolvedValue interface{}, // previously resolved value
		fieldArgs map[string]interface{}, // field arguments value
	) (interface{}, context.Context, error) // resolved value with updated context or error

	// Define will take pointer to graphql object and can modify it on making schema stage
	//   kind => obj
	//   "field" => *graphql.Field
	Define(kind string, obj interface{}) error
}

type ConstructorFun = func(args Arguments, nodeKind string) (Directive, error)

type Definition struct {
	name        string
	constructor ConstructorFun
}

func NewDirectiveDefinition(name string, constructor ConstructorFun) Definition {
	return Definition{name, constructor}
}

func (d Definition) Name() string {
	return d.name
}

func (d Definition) Construct(arguments Arguments, node ast.Node) (Directive, error) {
	if d.constructor == nil {
		panic(fmt.Errorf("constructor function is nil"))
	}

	return d.constructor(arguments, node.GetKind())
}
