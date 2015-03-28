#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go tool vet -printfuncs=httpErrorf:1,panicif:1,Noticef,Errorf *.go
gdep go build -o fofou_app *.go
