/*
  This is a minimal "Hello World" example for the Interbuilder Go
  module. True to the nature of the famous program, it only seeks
  to print the text "Hello, world!", and does this by creating one
  Spec and one Task, which prints the text when the Spec runs.

  To run this example from within the examples directory, execute:
    go run hello-print.go

  Here is its expected output:
    [root] Running
    [root] task: print-hello
    [root/print-hello] Hello, world!
    [root] Exit

    ib://root
      Tasks:
        > print-hello ()
*/

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
