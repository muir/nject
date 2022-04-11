

all:
	go install golang.org/x/tools/...@latest
	go generate
	go test
