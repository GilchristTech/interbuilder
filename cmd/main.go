package main

import (
  "log"
  . "gilchrist.tech/interbuilder"
  "gilchrist.tech/interbuilder/behaviors"

  "os"
  "encoding/json"
)


func MakeDefaultRootSpec () *Spec {
  root := NewSpec("root", nil)

  root.AddSpecResolver(behaviors.ResolveSourceURLType)
  root.AddSpecResolver(behaviors.ResolveSourceDir)

  root.AddSpecResolver(behaviors.ResolveTaskInferSource)
  root.AddSpecResolver(behaviors.ResolveTaskSourceGitClone)
  root.AddSpecResolver(behaviors.ResolveTasksNodeJS)

  root.AddSpecResolver(behaviors.ResolveSubspecs)

  root.EnqueueTaskFunc("root-consume", behaviors.TaskConsumeCopyFiles)
  return root
}


/*
  TODO: CLI options. Currently, the main entrypoint for the Interbuilder CLI simply runs a JSON file with a sample build spec tree. It git-clones two NodeJS-based static sites, installs dependencies, runs their build commands, and emits their build directories.
*/
func main () {
  root := MakeDefaultRootSpec()

  // Load spec configuration from file
  //
  specs_bytes, err := os.ReadFile("specs.json")
  if err != nil {
    log.Panic(err)
  }
  json.Unmarshal(specs_bytes, &root.Props)

  // Resolve
  //
  if err = root.Resolve() ; err != nil {
    log.Panic(err)
  }

  // Run tasks
  //
  if err = root.Run() ; err != nil {
    log.Panic(err)
  }

  PrintSpec(root, 0)
}
