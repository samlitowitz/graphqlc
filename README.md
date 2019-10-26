# GraphQL Compiler (graphqlc)
[![Go Report Card](https://goreportcard.com/badge/github.com/samlitowitz/graphqlc)](https://goreportcard.com/report/github.com/samlitowitz/graphqlc)
[![GoDoc](https://godoc.org/github.com/samlitowitz/graphqlc/pkg/graphqlc?status.svg)](https://godoc.org/github.com/samlitowitz/graphqlc/pkg/graphqlc)

`graphqlc` is a `protoc` style code generator for GraphQL.
 The project attempts to adhere to `protoc` standards whenever possible.
 
 ## Supported
   * `protoc` style plugins and parameter passing
   * `protoc` style insertion points 

See [api/protobuf](api/protobuf) for specification.
 
 # Installation
 `go get -u github.com/samlitowitz/graphqlc/cmd/graphqlc`
 
 # Usage
 Install `graphqlc-gen-*` plugin.

 `graphqlc --*_out=. path/to/*.graphql`
 
# Reference
1. [GraphQL Specification](https://graphql.github.io/graphql-spec/June2018/)
