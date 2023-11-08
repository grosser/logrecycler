.PHONY: default test

default:
	go build .

# somehow github actions have an open stdin so we need to close it
test: default
	go-testcov . -covermode atomic </dev/null
	ruby test.rb -v </dev/null
	go mod tidy && git diff --exit-code
	go fmt . && git diff --exit-code
