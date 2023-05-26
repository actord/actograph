package directive

import (
	"context"
	"fmt"
	"github.com/graphql-go/graphql"
)

type Deprecated struct {
	reason string
}

func NewDeprecated(args Arguments, nodeKind string) (Directive, error) {
	reason, ok := args["reason"]
	if !ok {
		return nil, fmt.Errorf("reason shoul be defined")
	}

	return &Deprecated{
		reason: reason.GetValue().(string),
	}, nil
}

func (d *Deprecated) Execute(
	ctx context.Context,
	_ interface{}, // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{}, // previously resolved value
	_ map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	return resolvedValue, ctx, nil
}

func (d *Deprecated) Define(kind string, obj interface{}) error {
	switch kind {
	case "*graphql.Field":
		field := obj.(*graphql.Field)
		field.DeprecationReason = d.reason
	case "*graphql.EnumValueConfig":
		enumConfig := obj.(*graphql.EnumValueConfig)
		enumConfig.DeprecationReason = d.reason
	default:
		return fmt.Errorf("unknown kind: %s", kind)
	}

	return nil
}
