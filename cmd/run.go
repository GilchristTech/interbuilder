package main

import (
  . "gilchrist.tech/interbuilder"

  "github.com/spf13/cobra"

  "fmt"
  "os"
  "io"
  "encoding/json"
)


var cmd_run = & cobra.Command {
  Use: "run [file]",
  Short: "Run from a build specification file",
  Args: cobra.ExactArgs(1),
  Run: func (cmd *cobra.Command, args []string) {
    var root       *Spec = MakeDefaultRootSpec()
    var spec_file string = args[0]

    // handle flag: --print-spec
    //
    if Flag_print_spec {
      defer func () {
        fmt.Println()
        PrintSpec(root)
      }()
    }

    // Load spec configuration from file
    //
    specs_bytes, err := os.ReadFile(spec_file)
    if err != nil {
      fmt.Printf("Could not read spec file: %v\n", err)
      os.Exit(1)
    }

    if err := json.Unmarshal(specs_bytes, &root.Props); err != nil {
      fmt.Printf("Could not parse spec json file: %v\n", err)
      os.Exit(1)
    }

    // Create tasks for outputs
    //
    for output_i, output_dest := range Flag_outputs {
      var task_name = fmt.Sprintf("cli-output-%d", output_i)

      var writer io.Writer
      var closer io.Closer
      var err    error

      writer, closer, err = outputStringToWriter(output_dest)

      if err != nil {
        fmt.Println(err)
        os.Exit(1)
      }

      root.EnqueueTaskMapFunc(task_name, func (a *Asset) (*Asset, error) {
        asset_json, err := AssetJsonMarshal(a)
        if err != nil {
          return nil, err
        }
        writer.Write(asset_json)
        writer.Write([]byte("\n"))
        return a, nil
      })

      // Defer a Task to close the file, if applicable
      //
      if closer != nil {
        if closer == os.Stdout {
          goto DONT_CLOSE
        }

        var task_name  = fmt.Sprintf("cli-output-close-%d", output_i)
        var close_task = root.DeferTaskFunc(task_name, func (*Spec, *Task) error {
          closer.Close()
          return nil
        })
        close_task.IgnoreAssets = true
      }
      DONT_CLOSE:
    }

    // Resolve
    //
    if err = root.Resolve() ; err != nil {
      fmt.Printf("Error while resolving build specs: %v\n", err)
      os.Exit(1)
    }

    // Run tasks
    //
    if err = root.Run() ; err != nil {
      fmt.Printf("Error while running build specs: %v\n", err)
      os.Exit(1)
    }
  },
}


