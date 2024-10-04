# Interbuilder: Declarative Build Pipelining

Interbuilder is a declarative workflow and build pipeline tool,
intended for use with static web assets and the programs that
generate them.

This software's initial version is still in development.

This repository contains the core Interbuilder package in the
repository's root, the behaviors package in the `behaviors`
directory, and the CLI package in the `cmd` directory. 

## CLI Usage

Depending on the subcommand used, the Interbuilder CLI can run
existing build specifications, and create simple asset pipelines.

### `interbuilder run`: Run a build specification file

### `interbuilder assets`: Run simple asset pipelines

### Controlling asset outputs

Wherever asset outputs are able to be specified in the CLI, they
can be preceded with any number of arguments which change the
asset format and filtering. By default, an output will receive
all assets from the root spec in the form of JSON, with content
encoded in plain or base64 encoded strings, along with asset URLs
and MIME types. The format can be controlled with a positional
argument starting with `format:`, followed by a comma-separated
list of tags which control what to include in the formatting.
This default behavior can be manually-specified like so:

```bash
interbuilder run example.spec.json \
  format:json,url,mimetype,string,base64 assets.json
```

These these can be enabled with the shorthand tag, `default`, and
then those tags can be disabled by prefixing them with `no-`.
Also, the format can be set from `json` to `text`. For example:

```bash
interbuilder assets --input assets.json \
  format:default,text,no-string,no-mimetype output.json
```

Asset outputs can also be filtered using a similar syntax. For
example, to define two outputs, one which takes assets with a
file extension of `.html` and another output which takes all
pictures from a path with a path prefix of `/static/`, one could
use the following:
```bash
interbuilder assets --input - \
  filter:extension=html html-assets.json \
  filter:prefix=/static/,mime=picture/ static-picture-assets.json
```

## Spec JSON usage

## Compilation, running, and tests:

Most actions related to compilation and testing are defined in
the `Makefile`. This Makefile also tracks dependencies as a
target, and therefore dependencies are installed automatically,
but dependencies can be manually installed by running `make
deps`.

To compile the CLI, run any of the following:
```bash
make
make build
make cli
```
This will produce an executable called `interbuilder`.

To run tests:
```
make test
```

If developing, tests can be live-updated with `make test-watch`,
and test coverage can be viewed with `make test-coverage` or
`make test-coverage-browser`.

## Pipeline Concepts

For the user, an Interbuilder pipeline is meant to be defined in
a short JSON file. This is meant to unburden the user with managing the pipeline and every particular of their build process when working at a high level.

### Build Specifications (Specs)
  Interbuilder organizes data pipelines into a tree of Specs
  running in parallel. Each Spec runs a serial list of Tasks, and
  Tasks can tell the Spec to emit Assets as output, usually to
  the Spec's parent.

### Spec Properties (Props)
  Each Spec contains a typically user-defined JSON-like data
  structure for holding metadata and hints or instructions of
  which tasks are to be executed.  Tasks, Asset callback
  functions, Resolvers, TaskResolvers, and Tasks read from these
  as a configuration data structure.

### Tasks
  While Specs are ran in parallel, within each Spec is a
  serially-ran queue of Tasks. Each task can change what comes
  later in the task queue.
  
### Assets
  An asset represents one or more things which gets passed
  through the pipeline. Usually, these represent files or sets of
  files. An asset can be singular and readable, or pluralistic
  and expandable into more assets.

### Interbuilder URLs (ib://)
  Interbuilder uses URLs with the `ib://` scheme to denote
  different resources internally.

### Prop Resolution (Resolvers)

### Task Resolution and Handlers (TaskResolvers)

### Path Transformations
  Each Spec has an optional array of path transformations, which
  apply a change to the URL path of each asset emmited. Tasks can
  read these path transformations
