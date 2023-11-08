.PHONY: default test style

default:
	go build .

# keep in sync with .github/workflows/test.yaml
test:
	go-testcov . -covermode atomic
	ruby test.rb -v
	make style

style:
	go mod tidy && git diff --exit-code
	go fmt . && git diff --exit-code
