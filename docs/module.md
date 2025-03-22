# Interbuilder Go Module

In addition to being a CLI tool, Interbuilder can also be used as
a Go concurrency module. This document aims to mention some of
these concepts, but Interbuilder's Go API is currently being
actively changed. Some parts may be renamed or reworked, and this
documentation is not meant to be comprehensive at the moment.

Additionally, this document does not aim to represent individual
methods, functions, variables, or structs; for that, `go doc
github.com/GilchristTech/interbuilder` would provide far more
information. Instead, this document aims to broadly describe how
Interbuilder is organized under the hood, and is very incomplete.

## Example module usage

Here is a minimal Hello World program in Interbuilder.
```
go get "github.com/GilchristTech/interbuilder"
go tidy
```

`hello-print.go`:
```go
package main

import (
  "fmt"
  ib "github.com/GilchristTech/interbuilder"
)

func main () {
  var spec *ib.Spec = ib.NewSpec("root", nil)
spec.EnqueueTaskFunc("print-hello", func (sp *ib.Spec, tk *ib.Task) error {
    tk.Println("Hello, world!")
    return nil
  })

  if err := spec.Run(); err != nil {
    fmt.Printf("Error when running spec:\n%v\n", err)
  }

  fmt.Println()
  ib.PrintSpec(spec)
}
```

## Pipeline Concepts

For the user, an Interbuilder pipeline is meant to be defined in
a short JSON file. This is meant to unburden the user with
managing the pipeline and every particular of their build process
when working at a high level.

These all map to the data structures in the core Go module. This
page functions as an overview. For information on specific
structs or methods, use `go doc`.

### Build Specifications (Specs)
  Interbuilder organizes data pipelines into a tree of Specs
  running in parallel. Each Spec runs a serial list of Tasks, and
  Tasks can tell the Spec to emit Assets as output, usually to
  the Spec's parent.

### Spec Properties (Props)
  Each Spec contains a typically user-defined JSON-like data
  structure for holding metadata and hints or instructions of
  which tasks are to be executed.  Tasks, Asset callback
  functions, SpecBuilders, TaskResolvers, and Tasks read from
  these as a configuration data structure.

### Tasks
  While Specs are ran in parallel, within each Spec is a
  serially-ran queue of Tasks. Each task can change what comes
  later in the task queue.
  
## Assets
  An asset represents one or more things which gets passed
  through the pipeline. Usually, these represent files or sets of
  files. An asset can be singular and readable, or pluralistic
  and expandable into more assets.

### Prop Evaluation (SpecBuilders)

#### Task Resolution and Handlers (TaskResolvers)

### Path Transformations
  Each Spec has an optional array of path transformations, which
  apply a change to the URL path of each asset emitted. Tasks can
  read these path transformations

## Tests

To run tests for the Interbuilder modules, run:
```
make test
```

If developing, tests can be live-updated with `make test-watch`,
and test coverage can be viewed with `make test-coverage` or
`make test-coverage-browser`.
