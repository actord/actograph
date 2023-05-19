package directives

import (
	"context"
	"fmt"
	"github.com/actord/actograph/directive"
)

type DirectiveSetContext struct {
	key               string
	value             string
	valueType         string
	isDefinedOnSchema bool
}

func NewDirectiveSetContext(args directive.Arguments, nodeKind string) (directive.Directive, error) {
	return &DirectiveSetContext{
		key:               args["key"].GetValue().(string),
		value:             args["val"].GetValue().(string),
		valueType:         args["valType"].GetValue().(string),
		isDefinedOnSchema: nodeKind == "SchemaDefinition",
	}, nil
}

func (d *DirectiveSetContext) Execute(
	ctx context.Context,
	source interface{},               // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{},        // previously resolved value
	fieldArgs map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	var value interface{}
	switch d.valueType {
	case "STRING":
		value = d.value
	case "SOURCE_KEY":
		if d.isDefinedOnSchema {
			// resolvedValue - it's a hack, for directive on schema we pass raw object as a resolved value
			s := resolvedValue.(map[string]interface{})
			value = s[d.value]
		} else {
			s := source.(map[string]interface{})
			value = s[d.value]
		}
	case "ARG_KEY":
		value = fieldArgs[d.value]
	default:
		panic(fmt.Errorf("unknown valType: %s", d.valueType))
	}

	ctx = context.WithValue(ctx, d.key, value)
	return resolvedValue, ctx, nil
}
