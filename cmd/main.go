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
  root.AddSpecBuilder(behaviors.BuildSourceURLType)
  root.AddSpecBuilder(behaviors.BuildSourceDir)
  root.AddSpecBuilder(behaviors.BuildTransform)

  // Source code inference layer
  //
  root.AddSpecBuilder(behaviors.BuildTaskInferSource) // TODO: rename to match TaskAssetsInfer?
  root.AddSpecBuilder(behaviors.BuildTaskSourceGitClone)
  root.AddSpecBuilder(behaviors.BuildTasksNodeJS)

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

  root.DeferTaskFunc("root-consume", behaviors.TaskConsumeLinkFiles)

  // Subspec layer
  //
  root.AddSpecBuilder(behaviors.ResolveSubspecs)

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
