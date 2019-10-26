//go:generate protoc -I./api/protobuf --go_out=paths=source_relative:./pkg/graphqlc api/protobuf/descriptor.proto api/protobuf/plugin.proto
package graphqlc
