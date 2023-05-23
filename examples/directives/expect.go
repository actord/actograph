package directives

import (
	"context"
	"fmt"
	"github.com/actord/actograph/directive"
)

type DirectiveExpect struct {
	expectedKind string

	expectedString string
}

func NewDirectiveExpect(args directive.Arguments, nodeKind string) (directive.Directive, error) {
	if nodeKind != "FieldDefinition" {
		return nil, fmt.Errorf("only FieldDefinition, because we expect value in resolvedValue")
	}
	if expectedString, ok := args["string"]; ok {
		return &DirectiveExpect{
			expectedKind:   "String",
			expectedString: expectedString.GetValue().(string),
		}, nil
	}

	return nil, fmt.Errorf("define what are you expect")
}

func (d *DirectiveExpect) Execute(
	ctx context.Context,
	source interface{}, // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{}, // previously resolved value
	fieldArgs map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	if d.expectedKind == "String" && resolvedValue.(string) == d.expectedString {
		return resolvedValue, ctx, nil
	}
	return nil, ctx, fmt.Errorf("expected string: '%s' but got: '%v'", d.expectedString, resolvedValue)
}
