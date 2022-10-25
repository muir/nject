

all:
	go install golang.org/x/tools/...@latest
	go generate
	go test
	golangci-lint run

coverage: calculate_coverage
	go tool cover -html=coverage.txt

golanglint:
	# binary will be $(go env GOPATH)/bin/golangci-lint
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.48.0
	golangci-lint --version

lint:;
	golangci-lint run
	
misspell:;
	go install github.com/client9/misspell/cmd/misspell@latest
	misspell -w `find . -name \*.md`

