package appendtest

import (
	"github.com/samlitowitz/graphqlc/pkg/graphqlc"
)

type Generator struct {
	*graphqlc.Generator
}

func New() *Generator {
	g := new(Generator)
	g.Generator = graphqlc.New()
	g.LogPrefix = "graphqlc-gen-appendtest"
	return g
}

func (g *Generator) GenerateAllFiles() {
	for _, fd := range g.Request.GraphqlFile {
		g.Response.File = append(g.Response.File, &graphqlc.CodeGeneratorResponse_File{
			Name:    fd.Name + ".test",
			Content: "// @@graphqlc_insertion_point(COMMENT_TEST)\n",
		}, &graphqlc.CodeGeneratorResponse_File{
			Content: "    @@graphqlc_insertion_point(INDENT_TEST)\n",
		})
	}
}
