# Interbuilder Documentation

Interbuilder is a declarative workflow and build pipeline tool,
intended for use with static web assets and the programs that
generate them.

Interbuilder's initial version is still in development, and
fundamental reworks of any part of it may occur frequently.

[GitHub Repository](https://github.com/GilchristTech/interbuilder)

## Documentation sections

Because Interbuilder consists of a command-line program,
expression syntax, schemas in formats like JSON, Go modules with
specific data and concurrency structures, and possibly other
things, this documentation attempts to organize such concepts
into sections where they are most relevant. 

- **[Command-line Interface](./cli.md)**
  The primary intended use of Interbuilder is through its CLI and
  Spec files. The CLI documentation covers CLI compilation,
  usage, and arguments that do not pertain directly to Specs.

- **[Build Specifications (Specs)](./specs.md)**
  Interbuilder provides means to define pipelines in JSON and
  YAML formats through `.spec.json` and `.spec.yaml` files. When
  building a pipeline, it will read the structured data using a
  modular system of behaviors, defined in the `behaviors` module.

  The Build Specifications documentation covers the default ways
  JSON structures are translated into the Interbuilder runtime,
  called behaviors.

- **[Go Modules](./module.md)**
  Interbuilder can also be used as a Go concurrency pipeline
  module. There are three modules, the core module, `behaviors`
  for modular Spec parsing, and `cmd` for the CLI. The Go Modules
  documentation contains broad information about the technical
  design of Interbuilder's internals.
