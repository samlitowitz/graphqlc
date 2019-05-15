//go:generate protoc -I./pkg --go_out=paths=source_relative:./pkg pkg/graphqlc/descriptor.proto
//go:generate protoc -I./pkg --go_out=paths=source_relative:./pkg pkg/graphqlc/compiler/plugin.proto
package graphqlc
