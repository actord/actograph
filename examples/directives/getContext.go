package directives

import (
	"context"
	"github.com/actord/actograph/directive"
)

type DirectiveGetContext struct {
	key string
}

func NewDirectiveGetContext(args directive.Arguments, nodeKind string) (directive.Directive, error) {
	key := args["key"].GetValue().(string)
	return &DirectiveGetContext{
		key: key,
	}, nil
}

func (d *DirectiveGetContext) Execute(
	ctx context.Context,
	source interface{},               // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{},        // previously resolved value
	fieldArgs map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	value := ctx.Value(d.key)
	return value, ctx, nil
}
