#!/bin/bash
#cp translog.go util.go langs.go tools/importsumatra
cd tools/importappengine
go build -o importappeng *.go
if [ "$?" -ne 0 ]; then echo "failed to build"; exit 1; fi 
#rm translog.go util.go langs.go # we used you so now we discard you
cd ../..
./tools/importappengine/importappeng
rm tools/importappengine/importappeng
