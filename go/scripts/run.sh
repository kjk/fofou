export GOPATH=`pwd`/ext:$GOPATH
go build -o fofou_app *.go || exit 1
./fofou_app
