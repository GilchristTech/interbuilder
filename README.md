# Interbuilder: Declarative Build Pipelining

Interbuilder is a workflow and build pipeline tool for static web
assets and the programs which generate them. It parallelizes
programs which write files, such as static site generators, and
sends their output through a concurrent pipeline to merge their
file trees and update content where files link to each other.
If the URLs of any HTML or CSS files are changed, the content of
the files containing these references are updated to point at the
new URL values.

It consists of a command-line program, expression syntax, schemas
in formats like JSON, Go modules with specific data and
concurrency structures, and more.  This `README` is an overview
of the repository itself, along with brief usage notes for
the CLI user. For more specific information, see [the
documentation](./docs/README.md) and the `docs/` directory in
this repository.

This software's initial version is still in development.

## Compiling the CLI

Interbuilder uses a Makefile to manage its compilation. To
compile the CLI, run:
```bash
git clone https://github.com/GilchristTech/interbuilder
cd interbuilder/
make
```
This will produce an executable called `interbuilder`. It can be
installed by putting the binary somewhere in your `PATH`.

## Spec files

For information on Specs and Spec files, see the [Spec
documentation](./docs/specs.md)

Build specifications can be defined in JSON or YAML. Interbuilder
uses these to build a concurrent process and file pipeline prior
to running. 

### `example.spec.json`
```json
{
  "source_nest": "build",

  "subspecs": {
    "site-a": {
      "source": "git://example.com/my-nodejs-static-site",
      "transform": {
        "prefix": "join-this-onto-all-url-paths"
      }
    },
    "site-b": {
      "source": "git://example.com/other-nodejs-static-site",
      "transform": {
        "prefix": "this-site-has-its-urls-on-a-different-path"
      }
    },
  }
}
```

Alternatively, you may find it cleaner and easier to write this in YAML:

### `example.spec.yaml`
```YAML
source_nest: build

subspecs:
  site-a:
    source: "git://example.com/my-nodejs-static-site"
    transform:
      prefix: join-this-onto-all-url-paths

  site-b:
    source: "git://example.com/other-nodejs-static-site"
    transform:
      prefix: this-site-has-its-urls-on-a-different-path
```

When ran with `interbuilder run example.spec.json` (the spec file
format is inferred by the file extension), this would do
the following, in parallel:
  * Clone two git repositories*
  * Detect they are NodeJS packages
  * Install NodeJS packages*
  * Run each packages' build command
  * Collect the build output files, and manipulate their file
    trees.
  * Combine the two file trees together, forming a new static
    site.

&ast;Tasks which download use a mutex lock so they will run in series.

## CLI Usage

For information on CLI usage, see
[CLI documentation](./docs/cli.md)
or run `interbuilder [command] help`. 

Interbuilder is primarily intended to be used as a CLI tool.
Build specs, as demonstrated above, are able to be ran with the
CLI binary:
```bash
interbuilder run myspec.spec.yaml
```

When the Interbuilder CLI outputs a set of files, it can
optionally encode them into a single line-delimited JSON file.
This allows the CLI to work with named pipes:

```bash
mkfifo pipe
interbuilder run spec.json pipe &
interbuilder assets --input pipe output.json
```
Asset outputs can be filtered. For example, to define two
outputs, one which takes assets with a file extension of `.html`
and another output which takes all pictures from a path with a
path prefix of `/static/`, one could use the following:
```bash
interbuilder assets --input - \
  filter:extension=html html-assets.json \
  filter:prefix=/static/,mime=picture/ static-picture-assets.json
```

## Go modules

In addition to being a CLI tool, Interbuilder can also be used as
a Go concurrency module. For more information, see [the module
documentation](./docs/module.md), or run `go doc
github.com/GilchristTech/interbuilder`. 

There are three modules, the core module in the root of this
repository, the `behaviors` module which contains spec file
processing and features, and the `cmd` module for the CLI.

## Repository layout

This repository contains the core Interbuilder package in the
repository's root, the behaviors package in the `behaviors`
directory, and the command-line package in the `cmd` directory. 

The Go module in this repository's root directory is the core
Interbuilder module. This defines the concurrency structure for
Interbuilder inside Specs, Tasks, and Assets, among other things.

The `behaviors/` directory contains more implementation-specific
functionality, including the processing of JSON structures in
spec files. For example, if the JSON property `"source":
"git://example.com/repo.git"` defines a SpecBuilder to interpret
that data into a the Spec's Props and enqueue a Task to download
the repository and infer what further tasks to run, based on its
contents.

The `cmd/` directory defines the main function entrypoint for the
command-line interface of Interbuilder. It handles argument
parsing, and constructing pipelines based on user-specified
arguments.

The `docs/` directory contains Interbuilder documentation,
written in Markdown. Currently, there is no build process for
generating a website based off these. This documentation does not
cover individual data structures, functions, or methods defined
in Go, and for that, `go doc` is more appropriate or.

Finally, the `examples/` directory contains examples
demonstrating Interbuilder usage.
