package graphqlc

import (
	"log"
	"os"
	"strings"
)

type Generator struct {
	Request  *CodeGeneratorRequest
	Response *CodeGeneratorResponse

	LogPrefix string
}

func New() *Generator {
	g := new(Generator)
	g.Request = new(CodeGeneratorRequest)
	g.Response = new(CodeGeneratorResponse)
	return g
}

// Error reports a problem, including an error, and exits the program.
func (g *Generator) Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Printf("%s: error: %s", g.LogPrefix, s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func (g *Generator) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Printf("%s: error: %s", g.LogPrefix, s)
}
