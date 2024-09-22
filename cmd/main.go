package main

import (
  . "gilchrist.tech/interbuilder"
  "gilchrist.tech/interbuilder/behaviors"

  "github.com/spf13/cobra"

  "fmt"
  "os"
  "io"
  "encoding/json"
)


func MakeDefaultRootSpec () *Spec {
  root := NewSpec("root", nil)

  // Prop preprocessing layer
  //
  root.AddSpecResolver(behaviors.ResolveSourceURLType)
  root.AddSpecResolver(behaviors.ResolveSourceDir)

  // Source code inference layer
  //
  root.AddSpecResolver(behaviors.ResolveTaskInferSource) // TODO: rename to match TaskAssetsInfer?
  root.AddSpecResolver(behaviors.ResolveTaskSourceGitClone)
  root.AddSpecResolver(behaviors.ResolveTasksNodeJS)

  // Asset content inference
  //
  root.AddTaskResolver(& behaviors.TaskResolverAssetsInferRoot)
  behaviors.TaskResolverAssetsInferRoot.AddTaskResolver(& behaviors.TaskResolverAssetsInferHtml)
  root.AddTaskResolver(& behaviors.TaskResolverApplyPathTransformationsToHtmlContent)

  root.PushTaskFunc("root-consume", behaviors.TaskConsumeLinkFiles)

  // Subspec layer
  //
  root.AddSpecResolver(behaviors.ResolveSubspecs)

  return root
}


var cmd_root = & cobra.Command {
  Use: "interbuilder",
  Short: "Declarative Build Pipelining",
}


var print_spec   bool
var flag_outputs []string

var cmd_run = & cobra.Command {
  Use: "run [file]",
  Short: "Run from a build specification file",
  Args: cobra.ExactArgs(1),
  Run: func (cmd *cobra.Command, args []string) {
    var root       *Spec = MakeDefaultRootSpec()
    var spec_file string = args[0]

    // handle flag: --print-spec
    //
    if print_spec {
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
    for output_i, output_dest := range flag_outputs {
      var task_name = fmt.Sprintf("cli-output-%d", output_i)
      var writer io.Writer

      if output_dest == "-" {
        writer = os.Stdout
      } else {
        writer, err = os.Create(output_dest)
        if err != nil {
          fmt.Println(err)
          os.Exit(1)
        }
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

      if closer, ok := writer.(io.Closer); ok {
        if closer == os.Stdout {
          goto DONT_CLOSE
        }

        var task_name = fmt.Sprintf("cli-output-close-%d", output_i)
        root.DeferTaskFunc(task_name, func (*Spec, *Task) error {
          closer.Close()
          return nil
        })
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


func init () {
  cmd_root.AddCommand(cmd_run)

  cmd_run.Flags().BoolVar(
    &print_spec, "print-spec", false,
    "Print the build specification tree when execution is finished",
  )

  cmd_run.Flags().StringArrayVarP(
    &flag_outputs, "output", "o", []string{},
    "Specify an output",
  )
}

func main () {
  if err := cmd_root.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}
