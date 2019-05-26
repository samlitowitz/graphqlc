package generator

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/samlitowitz/graphqlc/pkg/graphqlc/compiler"
)

type Generator struct {
	Request  *compiler.CodeGeneratorRequest  // The input
	Response *compiler.CodeGeneratorResponse // The output
}

func New() *Generator {
	g := new(Generator)
	g.Request = new(compiler.CodeGeneratorRequest)
	g.Response = new(compiler.CodeGeneratorResponse)
	return g
}

// Error reports a problem, including an error, and exits the program.
func (g *Generator) Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("graphqlc-gen-appendtest: error:", s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func (g *Generator) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("graphqlc-gen-appendtest: error:", s)
	os.Exit(1)
}

func (g *Generator) GenerateAllFiles() {
	for _, fd := range g.Request.GraphqlFile {
		for _, i := range []int{0, 1, 2, 3, 4, 5} {
			g.Response.File = append(g.Response.File, &compiler.CodeGeneratorResponse_File{
				Name:           fd.Name + ".test",
				InsertionPoint: "COMMENT_TEST",
				Content:        fmt.Sprintf("comment %d", i),
			})

			g.Response.File = append(g.Response.File, &compiler.CodeGeneratorResponse_File{
				Name:           fd.Name + ".test",
				InsertionPoint: "INDENT_TEST",
				Content:        fmt.Sprintf("indent %d", i),
			})
		}
	}
}
