package main

import (
  . "gilchrist.tech/interbuilder"
  "gilchrist.tech/interbuilder/behaviors"

  "github.com/spf13/cobra"

  "fmt"
  "os"
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

  // Asset processing layer
  //
  root.DeferTaskFunc("root-consume", behaviors.TaskConsumeLinkFiles)

  // Subspec layer
  //
  root.AddSpecResolver(behaviors.ResolveSubspecs)

  return root
}


var cmd_root = & cobra.Command {
  Use: "interbuilder",
  Short: "Declarative Build Pipelining",
}


var print_spec bool

var cmd_run = & cobra.Command {
  Use: "run [file]",
  Short: "Run from a build specification file",
  Args: cobra.ExactArgs(1),
  Run: func (cmd *cobra.Command, args []string) {
    var root       *Spec = MakeDefaultRootSpec()
    var spec_file string = args[0]

    if print_spec {
      defer PrintSpec(root)
    }

    // Load spec configuration from file
    //
    specs_bytes, err := os.ReadFile(spec_file)
    if err != nil {
      fmt.Println("Could not read spec file: %w\n", err)
      os.Exit(1)
    }

    if err := json.Unmarshal(specs_bytes, &root.Props); err != nil {
      fmt.Printf("Could not parse spec json file: %v\n", err)
      os.Exit(1)
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

  cmd_run.Flags().BoolVar(&print_spec, "print-spec", false, "Print the build specification tree when execution is finished")
}

func main () {
  if err := cmd_root.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}
