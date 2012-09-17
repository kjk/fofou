export GOPATH=$GOPATH:`pwd`/ext
go build -o fofou_app handler_*.go util.go db.go main.go
