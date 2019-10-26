package main

import (
	"github.com/samlitowitz/graphqlc/pkg/graphqlc/compiler"
	"os"
)

func main() {

	g := compiler.New()

	g.CommandLineArguments(os.Args[1:])

	g.BuildTypeMap()
	g.BuildTypes()
	g.GenerateAllFiles()
}
