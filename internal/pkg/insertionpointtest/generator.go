package insertionpointtest

import (
	"fmt"
	"github.com/samlitowitz/graphqlc/pkg/graphqlc"
)

type Generator struct {
	*graphqlc.Generator
}

func New() *Generator {
	g := new(Generator)
	g.Generator = graphqlc.New()
	g.LogPrefix = "graphqlc-gen-insertionpointtest"
	return g
}

func (g *Generator) GenerateAllFiles() {
	for _, fd := range g.Request.GraphqlFile {
		for _, i := range []int{0, 1, 2, 3, 4, 5} {
			g.Response.File = append(g.Response.File, &graphqlc.CodeGeneratorResponse_File{
				Name:           fd.Name + ".test",
				InsertionPoint: "COMMENT_TEST",
				Content:        fmt.Sprintf("comment %d", i),
			})

			g.Response.File = append(g.Response.File, &graphqlc.CodeGeneratorResponse_File{
				Name:           fd.Name + ".test",
				InsertionPoint: "INDENT_TEST",
				Content:        fmt.Sprintf("indent %d", i),
			})
		}
	}
}
