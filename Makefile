.PHONY: default test style

default:
	go build .

# keep in sync with .github/workflows/test.yaml
test: go-testcov
	$(GOTESTCOV) . -covermode atomic
	ruby test.rb -v
	make style

style:
	go mod tidy && git diff --exit-code
	go fmt . && git diff --exit-code

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
GOTESTCOV ?= $(LOCALBIN)/go-testcov
GOTESTCOV_VERSION ?= v1.12.2

.PHONY: go-testcov
go-testcov: $(LOCALBIN) # Download go-testcov (replace existing if incorrect version)
	@(test -f $(GOTESTCOV) && $(GOTESTCOV) version | grep "$(GOTESTCOV_VERSION)" >/dev/null) || \
	(rm -f $(GOTESTCOV) && echo "Installing $(GOTESTCOV) $(GOTESTCOV_VERSION)" && \
	GOBIN=$(LOCALBIN) go install github.com/grosser/go-testcov@$(GOTESTCOV_VERSION))
