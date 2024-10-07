package main

import (
  . "gilchrist.tech/interbuilder"
  "gilchrist.tech/interbuilder/behaviors"

  "github.com/spf13/cobra"

  "fmt"
  "os"
)


func MakeDefaultRootSpec () *Spec {
  root := NewSpec("root", nil)

  // Prop preprocessing layer
  //
  root.AddSpecResolver(behaviors.ResolveSourceURLType)
  root.AddSpecResolver(behaviors.ResolveSourceDir)
  root.AddSpecResolver(behaviors.ResolveTransform)

  // Source code inference layer
  //
  root.AddSpecResolver(behaviors.ResolveTaskInferSource) // TODO: rename to match TaskAssetsInfer?
  root.AddSpecResolver(behaviors.ResolveTaskSourceGitClone)
  root.AddSpecResolver(behaviors.ResolveTasksNodeJS)

  // Asset content inference
  //
  assets_infer      := & behaviors.TaskResolverAssetsInferRoot
  assets_infer_html := & behaviors.TaskResolverAssetsInferHtml
  assets_infer_css  := & behaviors.TaskResolverAssetsInferCss
  assets_infer.AddTaskResolver(assets_infer_html)
  assets_infer.AddTaskResolver(assets_infer_css)
  root.AddTaskResolver(assets_infer)

  root.AddTaskResolver(& behaviors.TaskResolverApplyPathTransformationsToHtmlContent)
  root.AddTaskResolver(& behaviors.TaskResolverApplyPathTransformationsToCssContent)

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


func main () {
  if err := cmd_root.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}
