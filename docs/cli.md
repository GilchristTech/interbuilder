# Interbuilder CLI

Interbuilder is primarily intended as a command-line tool for
running [build spec files](./specs.md), spawning multiple system
processes in parallel and pooling their output files (assets)
concurrently. Additionally, Interbuilder can also spin up simple
asset pipelines. These two actions are divided into two
subcommands:

- `interbuilder run`: Run a build specification file

- `interbuilder assets`: Run simple asset pipelines

## Compilation

Most actions related to compilation and testing are defined in
the `Makefile` in the repository's root directory. This Makefile
also tracks dependencies as a target, and therefore dependencies
are installed automatically, but dependencies can be manually
installed by running `make deps`.


To compile the CLI, run any of the following:
```bash
git clone https://github.com/GilchristTech/interbuilder
cd interbuilder/
make
```
This will produce an executable called `interbuilder`. It can be
installed by putting the binary somewhere in your `PATH`.
Alternatively, `make cli` exists as more specific build target.
The CLI can also be compiled without the use of GNU Make with the
following:
```bash
go tidy
go build -o interbuilder ./cmd
```

## Asset files (`.assets.json`)

Interbuilder uses JSON for communication of Assets between
multiple instances of itself. The command-line interface can
write assets to JSON, read or write them from a pipe. This is
done with line-delimited JSON objects, making streaming
of Assets into and from the Interbuilder CLI possible.

When the Interbuilder CLI outputs sets of files, it can
optionally encode them into a single JSON file, which allows
the CLI to work with Unix/Linux pipe files:

```bash
mkfifo pipe
interbuilder run spec.json pipe &
interbuilder assets --input pipe output.json
```

## Controlling asset outputs with positional arguments

Positional arguments in the Interbuilder CLI specify pipeline
outputs for the pipeline's root spec, and modifications to that
output. These arguments can be filesystem destinations, filter
expressions, and format expressions. Any expressions
written are applied to the first filesystem output specified
after them. In other words, each asset output argument can be
preceded by any number of expressions, and these expressions can
change which assets are sent to that destination and how they are
encoded.

By default, an output will receive all assets from the root spec
in the form of JSON, with content encoded in plain or base64
encoded strings, along with asset URLs and MIME types. The format
can be controlled with a positional argument starting with
`format:`, followed by a comma-separated list of tags which
control what to include in the formatting. This default behavior
can be manually-specified in a format expression like so:

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
