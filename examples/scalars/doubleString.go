package scalars

import (
	"fmt"
	"github.com/actord/actograph"
	"github.com/graphql-go/graphql/language/ast"
)

var DoubleStringScalarConfig = actograph.ScalarConfig{
	Name:        "DoubleString",
	Description: "Just double the original string",
	Serialize: func(value interface{}) interface{} {
		strVal, ok := value.(string)
		if !ok {
			panic("value should be string")
		}
		return fmt.Sprintf("%s%s", strVal, strVal)
	},
	ParseValue: func(value interface{}) interface{} {
		stringValue, ok := value.(string)
		if !ok {
			panic("value should be in string")
		}
		return fmt.Sprintf("%s%s", stringValue, stringValue)
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		if valueAST.GetKind() != "StringValue" {
			panic("only strings as literal allowed")
		}
		val := valueAST.GetValue().(string)
		return fmt.Sprintf("%s%s", val, val)
	},
}
