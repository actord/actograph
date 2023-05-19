package actograph

import (
	"github.com/graphql-go/graphql"

	"github.com/actord/actograph/directive"
)

func (agh *Actograph) getFieldResolveFunc(directives []directive.Directive) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		currentFieldName := p.Info.FieldName
		source := p.Source
		args := p.Args
		ctx := p.Context
		var resolvedValue interface{}

		// if object is a map - try to find key like field name as resolved value
		if sourceMap, ok := source.(map[string]interface{}); ok {
			if val, ok := sourceMap[currentFieldName]; ok {
				resolvedValue = val
			}
		}

		// apply directives
		var err error
		resolvedValue, ctx, err = agh.executeDirectives(ctx, source, resolvedValue, args, directives)

		return resolvedValue, err
	}
}
