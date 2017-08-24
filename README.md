# EcsChecker

Welcome to EcsChecker.

Ensure env variables GOPATH and GOROOT are set properly, and this folder is at the following location:
`${GOPATH}/github.com/yangb8/`

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
