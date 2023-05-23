package actograph

import (
	"context"
	"github.com/graphql-go/graphql/language/ast"

	"github.com/graphql-go/graphql/gqlerrors"
)

type RequestQuery struct {
	// A GraphQL language formatted string representing the requested operation.
	RequestString string

	// A mapping of variable name to runtime value to use for all variables
	// defined in the requestString.
	VariableValues map[string]interface{}

	// The name of the operation to use if requestString contains multiple
	// possible operations. Can be omitted if requestString contains only
	// one operation.
	OperationName string

	// The value provided as the first argument to resolver functions on the top
	// level type (e.g. the query object type).
	RootObject map[string]interface{}

	// Context may be provided to pass application-specific per-request
	// information to resolve functions.
	Context context.Context
}

// Result has the response, errors and extensions from the resolved schema
type Result struct {
	Data       interface{}                `json:"data"`
	Errors     []gqlerrors.FormattedError `json:"errors,omitempty"`
	Extensions map[string]interface{}     `json:"extensions,omitempty"`
}

// SerializeFn is a function type for serializing a GraphQLScalar type value
type SerializeFn func(value interface{}) interface{}

// ParseValueFn is a function type for parsing the value of a GraphQLScalar type
type ParseValueFn func(value interface{}) interface{}

// ParseLiteralFn is a function type for parsing the literal value of a GraphQLScalar type
type ParseLiteralFn func(valueAST ast.Value) interface{}

// ScalarConfig options for creating a new GraphQLScalar
type ScalarConfig struct {
	Name         string
	Description  string
	Serialize    SerializeFn
	ParseValue   ParseValueFn
	ParseLiteral ParseLiteralFn
}

type ScalarDefinition struct {
	Name        string
	Description string
	// TODO: Directives
}
