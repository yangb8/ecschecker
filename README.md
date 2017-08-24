# EcsChecker

Welcome to EcsChecker.

If new to golang, please make sure env variables GOPATH and GOROOT are set properly, and this project shall be put at the following location:
`${GOPATH}/src/github.com/yangb8/ecschecker`

Just an example
```
$ echo $GOROOT
/usr/local/Cellar/go/1.8.3/libexec
$ echo $GOPATH
/Users/myusername/go
```

### Requirements

* [Golang](https://golang.org/dl/) 1.8
* [glide](https://github.com/Masterminds/glide)

### Build

```
glide install
go build -o verify verify.go
```

In case, you need to build for another platform different from your local
```
go get github.com/mitchellh/gox
gox -output="../bin/{{.Dir}}-{{.OS}}-{{.Arch}}"
```
