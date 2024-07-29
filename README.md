# Interbuilder: Declarative Build Pipelining

Interbuilder is a declarative workflow and build pipeline tool,
intended for use with static web assets and the programs that
generate them.

This software's initial version is still in development.

## Compilation

To compile the CLI, run one of the following:
```bash
make
make build
make cli
```

## Module Concepts

### Build Specifications (Specs)
  Interbuilder organizes data pipelines into a tree of Specs
  running in parallel. Each Spec runs a serial list of Tasks, and
  Tasks can tell the Spec to emit Assets as output, usually to
  the Spec's parent.

### Spec Properties (Props)
### Tasks

### Assets
  An asset represents one or more things which gets passed
  through the pipeline. Usually, these represent files or sets of
  files. An asset can be singular and readable, or pluralistic
  and expandable into more assets.

### Interbuilder URLs (ib://)
  Interbuilder uses URLs with the `ib://` scheme to denote
  different resources internally.

### Spec Resolution
### Task Resolution and Handlers
