package main

import (
	"os"

	"github.com/samlitowitz/graphqlc/pkg/graphqlc/generator"
)

func main() {

	g := generator.New()

	g.CommandLineArguments(os.Args[1:])

	g.BuildTypeMap()
	g.BuildTypes()
	g.GenerateAllFiles()
}
