GOLANGCI_LINT = $(GOPATH)/bin/golangci-lint

.PHONY: lint
lint: $(GOLANGCI_LINT)
	@echo "==> Linting codebase"
	@$(GOLANGCI_LINT) run

.PHONY: test
test:
	@echo "==> Running tests"
	GO111MODULE=on go test -v

.PHONY: test-cover
test-cover:
	@echo "==> Running Tests with coverage"
	GO111MODULE=on go test -cover .

$(GOLANGCI_LINT):
	# If the command is run in the current directory it will be added  to the
	# modules along with all of its dependencies. Changing directory to skip
	# that. go modules must be enabled to set to specific version. Forcing to
	# be on with env var.
	cd / && GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.17.1