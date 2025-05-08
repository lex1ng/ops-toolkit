

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ops cmd/ops-tool/main.go

local-build:
	go build -ldflags "-s -w" -o local-ops cmd/ops-tool/main.go