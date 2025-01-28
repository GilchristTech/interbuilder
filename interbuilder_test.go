package interbuilder

import (
  "testing"
  "fmt"
  "strings"
  "io"
)


func TestSpecEnqueueTaskIsNotCircular (t *testing.T) {
  spec := NewSpec("test", nil)
  spec.Props["quiet"] = true

  if _, err := spec.EnqueueTask( & Task { Name: "Task1" } ); err != nil {
    t.Fatal(err)
  }

  if _, err := spec.EnqueueTask( & Task { Name: "Task2" } ); err != nil {
    t.Fatal(err)
  }

  if spec.Tasks.GetCircularTask() != nil {
    t.Fatal("Task list is circular")
  }
}


func TestSpecDeferTaskIsNotCircular (t *testing.T) {
  /*
    Spec with only deferred tasks
  */
  spec_defer := NewSpec("defer-test", nil)
  spec_defer.Props["quiet"] = true

  spec_defer.DeferTask( & Task { Name: "Defer1" } )
  spec_defer.DeferTask( & Task { Name: "Defer2" } )

  if spec_defer.Tasks.GetCircularTask() != nil {
    t.Fatal("Task list is circular")
  }

  /*
    Spec with enqueued and deferred tasks
  */
  spec_enqueue_defer := NewSpec("defer-enqueue-test", nil)

  spec_enqueue_defer.EnqueueTask( & Task { Name: "Enqueue1" } )
  spec_enqueue_defer.DeferTask(   & Task { Name: "Defer1"   } )
  spec_enqueue_defer.EnqueueTask( & Task { Name: "Enqueue2" } )
  spec_enqueue_defer.DeferTask(   & Task { Name: "Defer2"   } )

  if spec_enqueue_defer.Tasks.GetCircularTask() != nil {
    t.Fatal("Task list is circular")
  }
}


func TestSpecRunDetectsCircularTask (t *testing.T) {
  var spec = NewSpec("test", nil)
  spec.Props["quiet"] = true

  var task_func = func (s *Spec, tk *Task) error {
    return nil
  }

  if task_1, err := spec.EnqueueTaskFunc("circular-1", task_func); err != nil {
    t.Fatal(err)
  } else if task_2, err := spec.EnqueueTaskFunc("circular-2", task_func); err != nil {
    t.Fatal(err)
  } else {
    // manually create a circular Task link, never do this
    task_2.Next = task_1
  }

  if err := spec.Run(); err == nil {
    t.Fatal("Expected Spec to error when encountering a circular task list", err)
  }
}


func TestSpecEmptySingularRunFinishes (t *testing.T) {
  spec := NewSpec("single", nil)
  spec.Props["quiet"] = true
  TestWrapTimeoutError(t, spec.Run)
}


func TestSpecEmptySingularRunEmitFinishes (t *testing.T) {
  spec := NewSpec("single", nil)
  spec.Props["quiet"] = true

  spec.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  TestWrapTimeoutError(t, spec.Run)
}


func TestSpecChildRunEmitFinishes (t *testing.T) {
  root := NewSpec("root", nil)
  root.Props["quiet"] = true
  subspec := root.AddSubspec( NewSpec("subspec", nil ) )

  subspec.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  TestWrapTimeoutError(t, root.Run)
}


func TestSpecTreeRunEmitFinishes (t *testing.T) {
  root      := NewSpec("root", nil)
  subspec_a := root.AddSubspec( NewSpec("subspec_a", nil ) )
  subspec_b := root.AddSubspec( NewSpec("subspec_b", nil ) )

  root.Props["quiet"] = true

  subspec_a.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  subspec_b.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  TestWrapTimeoutError(t, root.Run)
}


func TestSpecChildRunEmitConsumesAssetFinishes (t *testing.T) {
  root    := NewSpec("root", nil)
  subspec := root.AddSubspec( NewSpec("subspec", nil ) )

  root.Props["quiet"] = true

  subspec.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    if e := s.EmitAsset( & Asset { Url: s.MakeUrl("a") } ); e != nil { return e }
    if e := s.EmitAsset( & Asset { Url: s.MakeUrl("b") } ); e != nil { return e }
    if e := s.EmitAsset( & Asset { Url: s.MakeUrl("c") } ); e != nil { return e }
    return nil
  })

  root.EnqueueTaskFunc("test-consume", func (s *Spec, task *Task) error {
    var asset_count int = 0
    for asset := range s.Input {
      if asset != nil {
        asset_count++
      }
    }

    if asset_count != 3 {
      t.Fatal("Did not consume exactly three assets")
    }

    return nil
  })

  TestWrapTimeoutError(t, root.Run)
}


func TestSpecTaskCancelledBySubspecError (t *testing.T) {
  root    := NewSpec("root", nil)
  subspec := root.AddSubspec(NewSpec("subspec", nil))

  root.Props["quiet"] = true

  root.EnqueueTaskFunc("cancellable-consume", func (s *Spec, tk *Task) error {
    for { select {
      case <-tk.CancelChan:
        return nil
      case asset_chunk, ok := <-s.Input:
        if ok {
          t.Fatalf("Spec received unexpected asset chunk: %v", asset_chunk)
        }
    }}
    return nil
  })

  subspec.EnqueueTaskFunc("error", func (s *Spec, tk *Task) error {
    return fmt.Errorf("Expected error")
  })

  TestWrapTimeout(t, func () {
    if err := subspec.Run(); err == nil {
      t.Fatal("Expected specs to error, but no error was returned")
    } else {
      error_str := fmt.Sprintf("%v", err)
      if ! strings.Contains(error_str, "Expected error") {
        t.Fatalf("Spec exited with an error, but it was not the expected error: %v", err)
      }
    }
  })
}


func TestSpecChainTransformAssetPaths (t *testing.T) {
  root    := NewSpec("root", nil)
  level_3 :=    root.AddSubspec( NewSpec("level_3", nil ) )
  level_2 := level_3.AddSubspec( NewSpec("level_2", nil ) )
  level_1 := level_2.AddSubspec( NewSpec("level_1", nil ) )

  root.Props["quiet"] = true

  var produce_assets_started  bool
  var produce_assets_finished bool

  for spec_i, s := range []*Spec { level_1, level_2, level_3 } {
    // Define path transformations
    //
    var path_transformation_prop = make(map[string]any)
    path_transformation_prop["prefix"] = s.Name
    path_transformation, err := PathTransformationsFromAny(path_transformation_prop)
    if err != nil { t.Fatal(err) }
    s.PathTransformations = path_transformation

    s.EnqueueTaskFunc("chain-assets", func (s *Spec, task *Task) error {
      var num_assets = 0

      // The lowest-level spec in the chain should
      // produce assets instead of consuming them.
      //
      if spec_i == 0 {
        produce_assets_started = true
        for x := range 3 {
          asset_url := s.MakeUrl(fmt.Sprintf("a%d", x))
          task.Println(asset_url)
          if err := s.EmitAsset( & Asset { Url: asset_url }); err != nil {
            t.Fatal(err)
          }
          num_assets++
        }
        produce_assets_finished = true
      } else {
        // Consume assets if this is not the
        // lowest-level spec in the chain
        //
        for a := range s.Input {
          task.Println(a.Url)
          s.EmitAsset(a)
          num_assets++
        }
      }

      if num_assets != 3 {
        t.Fatal(fmt.Sprintf("Task %s in spec %s emitted %d assets, but expected 3", task.Name, s.Name, num_assets))
      }

      return nil
    })
  }

  root.EnqueueTaskFunc("pool-test-assets", func (s *Spec, task *Task) error {
    var expected_urls = []string {
      "ib://level_3/@emit/level_3/level_2/level_1/a0",
      "ib://level_3/@emit/level_3/level_2/level_1/a1",
      "ib://level_3/@emit/level_3/level_2/level_1/a2",
    }

    var expected_urls_found [3]bool

    for asset := range s.Input {
      task.Println(asset.Url)
      
      var url_matches bool

      for i, expected_url := range expected_urls {
        if asset.Url.String() == expected_url {
          if expected_urls_found[i] == true {
            t.Fatal("Root spec encountered an expected URL twice", asset.Url)
          }
          expected_urls_found[i] = true
          url_matches = true
          break
        }
      }

      if url_matches == false {
        t.Fatal("Root spec encountered unexpected URL:", asset.Url)
      }
    }

    // Check that each URL has been found
    //
    for i, expected_url := range expected_urls {
      if ! expected_urls_found[i] {
        t.Fatal("Root spec did not encounter all expected URLs, missing", expected_url)
      }
    }

    return nil
  })

  TestWrapTimeoutError(t, root.Run)

  if produce_assets_started == false {
    t.Fatal("Task produce-assets not started")
  }

  if produce_assets_finished == false {
    t.Fatal("Task produce-assets not finished")
  }
}


func TestSprintSpec (t *testing.T) {
  var root    *Spec = NewSpec("root", nil)
  var subspec *Spec = root.AddSubspec(NewSpec("subspec", nil))

  root.Props["root_prop"] = 1
  root.EnqueueTaskFunc("root_task", func (s *Spec, task *Task) error { return nil })

  subspec.Props["subspec_prop"] = true

  // Progress through the SprintSpec output, checking for
  // expected, ordered string occurence.
  //
  var specs_string string = SprintSpec(root)

  var expected_strings = [] string {
    "ib://root",
    "root_prop", "int", "1",
    "root_task",
    "\n",
    "ib://subspec",
    "subspec_prop", "bool", "true",
  }

  var specs_scan = specs_string

  for _, expected := range expected_strings {
    var index = strings.Index(specs_scan, expected)
    if index == -1 {
      t.Errorf("Did not find expected substring: %s", expected)
    }
    specs_scan = specs_scan[index + len(expected) :]
  }
}


func TestTaskPassAssetsToSpec (t *testing.T) {
  var root    *Spec = NewSpec("root", nil)
  var subspec *Spec = root.AddSubspec(NewSpec("subspec", nil))

  subspec.EnqueueTaskFunc("produce", func (s *Spec, tk *Task) error {
    for i := range 3 {
      asset := s.MakeAsset( fmt.Sprintf("%d", i) ) 
      asset.SetContentBytes(
        []byte( fmt.Sprintf("content %d", i) ),
      )
      tk.EmitAsset(asset)
      tk.Println(asset.Url)
    }
    return nil
  })

  subspec.EnqueueTaskFunc("mutate", func (s *Spec, tk *Task) error {
    for _, asset := range tk.Assets {
      // Read and modify the content of the asset
      content, err := asset.GetContentBytes()
      if err != nil { return nil }
      asset.SetContentBytes( []byte("modified " + string(content)) )

      tk.Println(asset.Url)
    }
    return nil
  })

  root.EnqueueTaskFunc("consume-assert", func (s *Spec, tk *Task) error {
    for { select {
    case <-tk.CancelChan:
      return nil

    case asset_chunk, ok := <-s.Input:
      if !ok {
        return nil
      }

      assets, err := asset_chunk.Flatten()
      if err != nil { return err }

      for _, asset := range assets {
        content, err := asset.GetContentBytes()
        if err != nil {
          t.Errorf("Error reading asset %s: %v", asset.Url, err)
        } else {
          var content string = string(content)
          var expect  string = fmt.Sprintf(
            "modified content %s",
            strings.TrimLeft(asset.Url.Path, "/"),
          )

          if content != expect {
            t.Errorf(
              "Asset %s content is \"%s\", expected \"%s\"",
              asset.Url, content, expect,
            )
          }
        }
      }
    }}

    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }
}


/*
  Test a pipeline which emits Assets between tasks, uses a
  content data reader function, applies a PathTransformation
  to strings inside the string content of the assets, and write
  the assets to disk.
*/
func TestTaskMapFuncPathTransformPipeline (t *testing.T) {
  root := NewSpec("root", nil)
  var output_dir = t.TempDir()
  root.Props["source_dir"] = output_dir
  root.Props["quiet"] = true

  // Path transformation
  //
  path_transformations, err := PathTransformationsFromAny("s`REPLACE-THIS`THIS-REPLACED`")
  if err != nil { t.Fatal(err) }
  root.PathTransformations = path_transformations

  // Produce an asset and pass it
  //
  root.EnqueueTaskFunc("produce", func (s *Spec, tk *Task) error {
    // Produce a .txt asset, on which to apply PathTransformations
    //
    err := s.WriteFile("asset.txt", []byte(`
      REPLACE-THIS
      dont-replace-this
    `), 0o660)
    if err != nil {
      return fmt.Errorf("Could not write asset file to produce: %w", err)
    }

    eventually_mutated_asset, err := s.MakeFileKeyAsset("asset.txt")
    if err != nil { return err }

    if err := tk.EmitAsset(eventually_mutated_asset); err != nil {
      return err
    }

    // Produce a control asset with no modifications to apply
    //
    err = s.WriteFile("control.bin", []byte(`
      ...binary data with incidental valid path transformation...
      REPLACE_THIS
      ...more binary data...
      dont-replace-this
      ...the last data...
    `), 0o660)

    if err != nil {
      return fmt.Errorf("Could not write asset file to produce: %w", err)
    }

    control_asset, err := s.MakeFileKeyAsset("control.bin")
    if err != nil { return err }

    if err := tk.EmitAsset(control_asset); err != nil {
      return err
    }

    return nil
  })

  // Assign text data handlers
  //
  root.EnqueueTaskMapFunc("type-txt-loader", func (a *Asset) (*Asset, error) {
    if ! strings.HasPrefix(a.Mimetype, "text") {
      return a, nil
    }

    err := a.SetContentDataReadFunc(func (s *Asset, r io.Reader) (any, error) {
      bytes, err := io.ReadAll(r)
      if err != nil { return "", err }
      return string(bytes), nil
    })
    if err != nil { return nil, err }

    err = a.SetContentDataWriteFunc(
      func (a *Asset, w io.Writer, data any) (int, error) {
        return w.Write([]byte(data.(string)))
      },
    )
    if err != nil { return nil, err }

    return a, nil
  })

  // Transform paths in text based on path transformations
  //
  root.EnqueueTaskMapFunc("text-transform", func (a *Asset) (*Asset, error) {
    // Only map over text
    //
    if ! strings.HasPrefix(a.Mimetype, "text") {
      return a, nil
    }

    content_any, err := a.GetContentData()
    if err != nil {
      return nil, fmt.Errorf(
        "Cannot get content data in text-transform on asset %s: %w", a.Url, err,
      )
    }

    var content string
    var ok      bool

    if content, ok = content_any.(string); ok == false {
      return nil, fmt.Errorf("ContentData is not a string")
    }

    var lines    []string = strings.Split(content, "\n")
    var modified bool

    for line_i, line := range lines {
      for _, transformation := range a.Spec.PathTransformations {
        new_line := transformation.TransformPath(line)
        modified = modified || (new_line != line)
        lines[line_i] = new_line
      }
    }

    if modified {
      a.SetContentData(strings.Join(lines, "\n"))
    }

    return a, nil
  })

  // Consume assets
  //
  var num_consumed = 0
  root.EnqueueTaskFunc("consume", func (s *Spec, tk *Task) error {
    for _, asset := range tk.Assets {
      num_consumed++

      content, err := asset.GetContentBytes()
      if err != nil { return err }

      key := strings.TrimLeft(asset.Url.Path, "/")
      switch key {
      default:
        t.Errorf("Unexpected asset key: %s", key)
      case "asset.txt":
        if ! strings.Contains(string(content), "THIS-REPLACED") {
          t.Fatalf("asset.txt does not content \"THIS-REPLACED\"")
        }
        if ! strings.Contains(string(content), "dont-replace-this") {
          t.Fatalf("control.bin does not contain \"dont-replace-this\"")
        }
        t.Log("asset:")
        t.Log(string(content))
      case "control.bin":
        if ! strings.Contains(string(content), "REPLACE_THIS") {
          t.Fatalf("control.bin does not contain \"REPLACE_THIS\"")
        }
        if ! strings.Contains(string(content), "dont-replace-this") {
          t.Fatalf("control.bin does not contain \"dont-replace-this\"")
        }
      }
    }
    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }

  if got, expect := num_consumed, 2; got != expect {
    t.Errorf("Consume task expected %d assets, got %d", expect, got)
  }
}
