package directive

import (
	"context"
	"fmt"

	"github.com/graphql-go/graphql"
)

//directive @_value(
//	string: String,
//	int: Int,
//	bool: Boolean,
//  arg: String,
//) on ENUM_VALUE

type Value struct {
	resolveValueFrom string // string/integer/boolean/argumentKey...

	string      string
	integer     int32
	boolean     bool
	argumentKey string
}

func NewValue(args Arguments, nodeKind string) (Directive, error) {
	d := &Value{}
	for key, value := range args {
		switch key {
		case "string":
			d.string = value.GetValue().(string)
			d.resolveValueFrom = "string"
		case "int":
			d.integer = value.GetValue().(int32)
			d.resolveValueFrom = "integer"
		case "bool":
			d.boolean = value.GetValue().(bool)
			d.resolveValueFrom = "boolean"
		case "arg":
			d.argumentKey = value.GetValue().(string)
			d.resolveValueFrom = "argumentKey"
		default:
			return nil, fmt.Errorf("unknown argument: %s", key)
		}
	}

	return d, nil
}

func (d *Value) Execute(
	ctx context.Context,
	source interface{}, // parent object. Not map[string]interface{} for scalars resolvers or nil
	resolvedValue interface{}, // previously resolved value
	fieldArgs map[string]interface{}, // field arguments value
) (interface{}, context.Context, error) { // resolved value with updated context or error
	var value interface{}
	switch d.resolveValueFrom {
	case "string":
		value = d.string
	case "integer":
		value = d.integer
	case "boolean":
		value = d.boolean
	case "argumentKey":
		value = fieldArgs[d.argumentKey] // TODO: handle not found behaviour
	}
	return value, ctx, nil
}

func (d *Value) Define(kind string, obj interface{}) error {
	switch kind {
	case "*graphql.EnumValueConfig":
		enumConfig := obj.(*graphql.EnumValueConfig)
		enumConfig.Value = d.getValue(nil) // nil - because its not argument values at define time
	case "*graphql.Field":
		return nil
	default:
		return fmt.Errorf("unsupported kind '%s' for @_value", kind)
	}
	return nil
}

func (d *Value) getValue(fieldArgs map[string]interface{}) interface{} {
	var value interface{}
	switch d.resolveValueFrom {
	case "string":
		value = d.string
	case "integer":
		value = d.integer
	case "boolean":
		value = d.boolean
	case "argumentKey":
		value = fieldArgs[d.argumentKey] // TODO: handle not found behaviour
	}
	return value
}
