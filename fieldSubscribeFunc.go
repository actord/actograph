package graphscm

import (
	"errors"

	"github.com/graphql-go/graphql"
)

func (scm *GraphScm) getFieldSubscribeFunc() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		return nil, errors.New("subscription is not implemented yet")
	}
}
