package actograph

import (
	"fmt"
	"io"
	"os"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"

	"github.com/actord/actograph/directive"
)

func NewActograph() *Actograph {
	return &Actograph{
		directiveDeclarations: map[string]directive.Definition{},

		directiveDefinitions:   map[string]*ast.DirectiveDefinition{},
		objectDefinitions:      map[string]*ast.ObjectDefinition{},
		inputObjectDefinitions: map[string]*ast.InputObjectDefinition{},
		enumDefinitions:        map[string]*ast.EnumDefinition{},
		declaredScalars:        map[string]ScalarDefinition{},

		enums:        map[string]*graphql.Enum{},
		objects:      map[string]*graphql.Object{},
		inputObjects: map[string]*graphql.InputObject{},
		scalars: map[string]*graphql.Scalar{
			// check for scalar or return object
			"String":   graphql.String,
			"Int":      graphql.Int,
			"Float":    graphql.Float,
			"Boolean":  graphql.Boolean,
			"ID":       graphql.ID,
			"DateTime": graphql.DateTime,
		},

		lazySchemaDirectives: []directive.Directive{},
	}
}

func NewActographBytes(graphqlFile []byte) (*Actograph, error) {
	gscm := NewActograph()
	return gscm, gscm.Parse(graphqlFile)
}

func NewActographFiles(filenames ...string) (*Actograph, error) {
	files := make([]io.Reader, len(filenames))
	for i, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("when reading schema in file %s: %v", filename, err)
		}
		files[i] = f
	}

	gqlSchemaData, err := io.ReadAll(io.MultiReader(files...))
	if err != nil {
		return nil, fmt.Errorf("when reading files: %v", err)
	}
	agh, err := NewActographBytes(gqlSchemaData)
	if err != nil {
		return nil, fmt.Errorf("when parse file: %v", err)
	}

	return agh, nil
}
