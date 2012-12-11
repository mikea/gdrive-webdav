GOARCH=amd64 GOPATH=`pwd` go build -gcflags "-N -l" main
gdb main

