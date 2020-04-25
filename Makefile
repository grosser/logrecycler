.PHONY: default test

default:
	go build .

# keep in sync with .travis.yml
test: default
	go-testcov . -covermode atomic
	ruby test.rb -v
	go mod tidy && git diff --exit-code
	go fmt . && git diff --exit-code
