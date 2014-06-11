#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go fmt
go fmt tools/importappengine/*.go

