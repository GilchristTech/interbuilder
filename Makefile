MODULE_SRC := $(wildcard *.go behaviors/*.go)
CMD_SRC    := $(wildcard cmd/*.go)
CMD        := interbuilder

DEPS_CHECK = .deps_checked
COVERAGE_FILE = coverage.out

WATCHER := npx nodemon -w . -w Makefile -e go,mod,sum,json -i .deps_checked -i build/ -x

$(CMD): $(DEPS_CHECK) $(CMD_SRC) $(MODULE_SRC)
	go build -o $(CMD) $(CMD_SRC)

.PHONY: all deps build cli clean watch test test-watch

all:   $(CMD)
build: $(CMD)
cli:   $(CMD)
deps:  $(DEPS_CHECK)

clean:
	rm $(CMD)
	rm -rf build/
	rm $(DEPS_CHECK)
	rm $(COVERAGE_FILE)

$(DEPS_CHECK): go.mod go.sum
	go mod tidy
	go mod download
	touch $(DEPS_CHECK)

test: $(DEPS_CHECK) $(MODULE_SRC)
	go test ./ ./behaviors/ ./cmd/ $(TEST_ARGS)
test-watch:
	$(WATCHER) 'make && make test || exit 1'

test-coverage: $(COVERAGE_FILE)
test-coverage-browser: $(COVERAGE_FILE)
	go tool cover -html=$(COVERAGE_FILE)

$(COVERAGE_FILE): $(DEPS_CHECK) $(MODULE_SRC)
	go test -coverprofile=$(COVERAGE_FILE) ./ ./behaviors
