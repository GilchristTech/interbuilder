package behaviors

import (
  "testing"
  . "gilchrist.tech/interbuilder"
  "strings"
)


func TestAssetsInferHtmlPathTransformations (t *testing.T) {
  var root = NewSpec("root", nil)
  var spec = root.AddSubspec(NewSpec("spec", nil))

  spec.Props["source_dir"] = t.TempDir()
  root.Props["source_dir"] = t.TempDir()

  root.AddTaskResolver(& TaskResolverAssetsInferRoot)
  TaskResolverAssetsInferRoot.AddTaskResolver(& TaskResolverAssetsInferHtml)
  root.AddTaskResolver(& TaskResolverApplyPathTransformationsToHtmlContent)

  // Path transformation
  //
  path_transformations, err := PathTransformationsFromAny("s`^/?`transformed/`")
  if err != nil { t.Fatal(err) }
  spec.PathTransformations = path_transformations

  // Produce assets
  //
  spec.EnqueueTaskFunc("produce-assets", func (s *Spec, tk *Task) error {
    err := s.WriteFile("file.txt", []byte("unmodified/path"), 0o660)
    if err != nil { return err }

    s.WriteFile("index.html", []byte(`
      <!DOCTYPE html>
      <html lang="en">
      <head>
        <meta charset="UTF-8">
        <title>Test page</title>
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
      </head>
      <body>
        <a href="page/">Relative link</a>
      </body>
      </html>
    `), 0o660)
    if err != nil { return err }

    asset, err := s.MakeFileKeyAsset("/")
    if err != nil { return err }
    return tk.EmitAsset(asset)
  })

  // Enqueue assets-infer task
  //
  infer_task, err := spec.EnqueueTaskName("assets-infer")
  if err != nil {
    t.Log(SprintSpec(root))
    t.Fatalf("Error while enqueuing task with name \"assets-infer\": %v", err)
  } else if infer_task == nil {
    t.Log(SprintSpec(root))
    t.Fatal("Expected assets-infer task to be enqueued, but it was nil")
  }

  // Root consumes test assets and assert them
  //
  var num_assets = 0
  root.EnqueueTaskFunc("consume-assert", func (s *Spec, tk *Task) error {
    if err := tk.PoolSpecInputAssets(); err != nil {
      return err
    }

    for _, asset := range tk.Assets {
      num_assets++

      key := strings.TrimLeft(asset.Url.Path, "/")
      switch key {
        default:
          t.Errorf("Unexpected Asset URL: %s", asset.Url)
        case "@emit/transformed/file.txt":
          data, err := asset.GetContentBytes()
          if err != nil {
            return err
          }
          if got, expect := string(data), "unmodified/path"; got != expect {
            t.Errorf("%s has content of \"%s\", expected \"%s\"", asset.Url, got, expect)
          }

        case "@emit/transformed/index.html":
          data, err := asset.GetContentBytes()
          if err != nil {
            return err
          }

          if expect := "transformed/page"; !strings.Contains(string(data), expect) {
            t.Errorf("Expected %s to have the string \"%s\", but it does not", asset.Url, expect)
          }
      }
    }

    return nil
  })

  var printed_spec = false

  if err := root.Run(); err != nil {
    if !printed_spec { t.Log(SprintSpec(root)); printed_spec = true }
    t.Error(err)
  }

  if got, expect := num_assets, 2; got != expect {
    if !printed_spec { t.Log(SprintSpec(root)); printed_spec = true }
    t.Errorf("Test finished with %d assets, expected %d", got, expect)
  }
}
