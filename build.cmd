set GOPATH=%CD%
go get -u code.google.com/p/google-api-go-client/drive/v2
go get -u code.google.com/p/goauth2/oauth
go get -u github.com/cihub/seelog
go get -u github.com/pmylund/go-cache
go build main