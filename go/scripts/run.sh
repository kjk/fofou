export GOPATH=$GOPATH:`pwd`/ext
go run handler_*.go util.go main.go
#go run handler_*.go util.go main.go -log fofou.log
