package directives

import (
	"context"
	"errors"
	"fmt"
	"github.com/actord/actograph/directive"
)

type DirectiveResolveArg struct {
	argName string
}

func NewDirectiveResolveArg(args directive.Arguments, nodeKind string) (directive.Directive, error) {
	argName, ok := args["argName"]
	if !ok {
		return nil, errors.New("arg not in arguments")
	}
	return &DirectiveResolveArg{
		argName: argName.GetValue().(string),
	}, nil
}

func (d *DirectiveResolveArg) Execute(
	ctx context.Context,
	source interface{}, // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{}, // previously resolved value
	fieldArgs map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	value, ok := fieldArgs[d.argName]
	if !ok {
		return nil, ctx, fmt.Errorf("key '%s' not found in args", d.argName)
	}
	return value, ctx, nil
}

func (d *DirectiveResolveArg) Define(_ string, _ interface{}) error {
	return nil
}
