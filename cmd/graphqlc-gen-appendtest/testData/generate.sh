#!/bin/bash
rm -f *.test
go install ../../graphqlc
go install ../
graphqlc --appendtest_out=. *.graphql