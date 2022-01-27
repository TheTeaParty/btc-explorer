linter:
	golangci-lint run --enable-all -D interfacer --fix
proto:
	protoc --proto_path=${GOPATH}/src:. --micro_out=pkg --go_out=pkg api/proto/*.proto