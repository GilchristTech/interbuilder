package interbuilder

import (
  "testing"
  "fmt"
  "strings"
)


func TestSpecEnqueueTaskIsNotCircular (t *testing.T) {
  spec := NewSpec("test", nil)
  spec.Props["quiet"] = true

  spec.EnqueueTask( & Task { Name: "Task1" } )
  spec.EnqueueTask( & Task { Name: "Task2" } )

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
