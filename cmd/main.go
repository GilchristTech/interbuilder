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

  // Asset processing layer
  //
  // root.AddSpecResolver(behaviors.ResolveTransformPathsAssetContentHtml)
  // root.AddSpecResolver(behaviors.ResolveTransform)
  root.DeferTaskFunc("root-consume", behaviors.TaskConsumeLinkFiles)

  // Subspec layer
  //
  root.AddSpecResolver(behaviors.ResolveSubspecs)

  return root
}


func mainRunSpecJsonFile (spec_file string) (status int, err error) {
  root := MakeDefaultRootSpec()

  // Load spec configuration from file
  //
  specs_bytes, err := os.ReadFile(spec_file)
  if err != nil {
    return 1, fmt.Errorf("Could not read spec file: %w", err)
  }

  if err := json.Unmarshal(specs_bytes, &root.Props); err != nil {
    return 1, fmt.Errorf("Could not parse spec json file: %w", err)
  }

  // Resolve
  //
  if err = root.Resolve() ; err != nil {
    return 1, fmt.Errorf("Error while resolving build specs: %w", err)
  }

  // Run tasks
  //
  if err = root.Run() ; err != nil {
    return 1, fmt.Errorf("Error while running build specs: %w", err)
  }

  return 0, nil
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
    status, err := mainRunSpecJsonFile(args[0])
    if err != nil {
      fmt.Println(err)
    }
    os.Exit(status)
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
