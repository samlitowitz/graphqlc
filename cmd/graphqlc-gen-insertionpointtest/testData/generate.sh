#!/bin/bash
rm -f *.test
go install ../../graphqlc
go install ../
graphqlc --appendtest_out=. --insertionpointtest_out=.  *.graphql
