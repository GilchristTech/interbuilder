package main

import (
  . "gilchrist.tech/interbuilder"
  "gilchrist.tech/interbuilder/behaviors"

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
  root.AddSpecResolver(behaviors.ResolveTaskInferSource)
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


func mainRunSpecJsonFile (args []string) (status int, err error) {
  if num_args, expected_num_args := len(args), 1; num_args != expected_num_args {
    return 1, fmt.Errorf("Expected %d args, got %d", expected_num_args, num_args)
  }

  spec_file := args[0]

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


func main () {
  args := os.Args[1:]

  status, err := mainRunSpecJsonFile(args)
  if err != nil {
    fmt.Println(err)
  }
  os.Exit(status)
}
