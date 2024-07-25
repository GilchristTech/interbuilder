MODULE_SRC := $(wildcard *.go behaviors/*.go)
CMD_SRC    := $(wildcard cmd/*.go)
CMD        := interbuilder

WATCHER := npx nodemon -w . -w Makefile -e go,mod,json -i build -x

$(CMD): $(CMD_SRC) $(MODULE_SRC)
	go build -o $(CMD) $(CMD_SRC)

.PHONY: all build cli run clean watch test test-watch

all:   $(CMD)
build: $(CMD)
cli:   $(CMD)

clean:
	rm $(CMD)

test: $(MODULE_SRC)
	go test $(TEST_ARGS)
test-watch:
	$(WATCHER) 'make && make test || exit 1'

# TODO: remove. These build targets are temporary; The main build target is a CLI tool which requires arguments.
# TODO: with these build targets being temporary, so is the specs.json which the current main function reads from
run: $(CMD)
	./$(CMD)
run-watch:
	$(WATCHER) 'make && make run || exit 1'
