# GraphQL Compiler (graphqlc)

`graphqlc` is a `protoc` style code generator for GraphQL.
 The project attempts to adhere to `protoc` standards whenever possible.
 
 ## Supported
   * `protoc` style plugins and parameter passing (pkg/graphqlc/plugin.proto)
   * `protoc` style insertion points 
 
 # Installation
 `go get -u github.com/samlitowitz/graphqlc/cmd/graphqlc`
 
 # Usage
 Install `graphqlc-gen-echo` plugin. 
 This plugin generates a new schema from the input schema renaming it `*.echo.graphql`, e.g. `schema.graphql` becomes `schema.echo.graphql`.
 
 `go get -u github.com/samlitowitz/graphqlc/cmd/graphqlc-gen-echo`
 
 `graphqlc --echo_out=. path/to/*.graphql`
 
 

https://graphql.github.io/graphql-spec/June2018/
