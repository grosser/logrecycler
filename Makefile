.PHONY: default test

default:
	go build .

test: default
	@# go-testcov . -covermode atomic # TODO: this fails on github action with "generating coverage report: write |1: file already closed"
	go test
	ruby test.rb -v
	go mod tidy && git diff --exit-code
	go fmt . && git diff --exit-code
