package directives

import (
	"context"
	"errors"
	"github.com/actord/actograph/directive"
)

type DirectiveResolveString struct {
	val string
}

func NewDirectiveResolveString(args directive.Arguments, nodeKind string) (directive.Directive, error) {
	val, ok := args["val"]
	if !ok {
		return nil, errors.New("val not in arguments")
	}
	return &DirectiveResolveString{
		val: val.GetValue().(string),
	}, nil
}

func (d *DirectiveResolveString) Execute(
	ctx context.Context,
	source interface{}, // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{}, // previously resolved value
	fieldArgs map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	return d.val, ctx, nil
}

func (d *DirectiveResolveString) Define(_ string, _ interface{}) error {
	return nil
}
