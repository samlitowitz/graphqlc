#!/bin/bash
rm -f *.echo.graphql
go install ../../graphqlc
go install ../
graphqlc --echo_out=ignored=test:. *.graphql