package interbuilder

import (
  "testing"
  "fmt"
  "strings"
  "os"
  "path/filepath"
  "sort"
)


func TestSpecTaskQueue (t *testing.T) {
  var root *Spec = NewSpec("root", nil)
  root.Props["quiet"] = true

  // Give all tasks the same function, which appends it's name
  // onto a list of tasks. The tasks will be inserted, and if
  // the alphanumeric order of their names is the same of their
  // execution, the test will pass.
  //
  // To evaluate this, the task being executed appends the task
  // name to an array, and the tasks will share the same task
  // function.
  //
  var task_log []string
  var task_func = func (sp *Spec, tk *Task) error {
    var task_name = tk.Name

    // Ensure Task queue and Specs are properly defined

    if sp == nil {
      t.Errorf("Task with name %s has a nil Spec argument", task_name)
    }

    if tk.Spec == nil {
      t.Errorf("Task with name %s has a nil Spec property", task_name)
    } else if sp != tk.Spec {
      t.Errorf("Task with name %s's Spec function argument and Spec struct property are not equal", task_name)
    }

    task_log = append(task_log, task_name)
    return nil
  }

  root.DeferTaskFunc("task_z", task_func)
  root.EnqueueTaskFunc("task_n", task_func)
  root.DeferTaskFunc("task_y", task_func)
  root.EnqueueTaskFunc("task_o", task_func)
  root.DeferTaskFunc("task_x", task_func)
  root.EnqueueTaskFunc("task_p", task_func)

  var task_a =   & Task { Name: "task_a", Func: task_func }
  task_a.Append( & Task { Name: "task_b", Func: task_func } )
  task_a.Append( & Task { Name: "task_c", Func: task_func } )
  root.PushTask(task_a)
  root.PushTaskFunc("task_d", task_func)

  // Add a task which modifies the task queue during execution

  root.PushTaskFunc("task_e", func (sp *Spec, tk *Task) error {
    if err := task_func(sp, tk); err != nil {
      return err
    }

    return tk.PushTaskFunc("task_f", task_func)
  })

  root.PushTaskFunc("task_g", task_func)

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }

  if got, expect := len(task_log), 13; got != expect {
    t.Errorf("Expected %d tasks in the task log, got %d", expect, got)
  }

  if sort.StringsAreSorted(task_log) == false {
    t.Errorf("Expected task_log to be in alphabetical order, got %v", task_log)
  }
}


func TestTaskCommand (t *testing.T) {
  var root       *Spec  = NewSpec("root", nil)
  var source_dir string = t.TempDir()
  root.Props["source_dir"] = source_dir
  root.Props["quiet"] = true

  root.EnqueueTaskFunc("create-file", func (s *Spec, tk *Task) error {
    // Assert an error is returned on a non-zero exit code
    //
    tk.Println("Running invalid commands")
    if _, err := tk.CommandRun("exit", "1"); err == nil {
      return fmt.Errorf("Command did not return an error")
    }
    if _, err := tk.CommandRun("invalid-command-thats-probably-not-in-PATH"); err == nil {
      return fmt.Errorf("Command did not return an error")
    }

    // Use a shell command to write a file
    //
    tk.Println("Creating file")
    if cmd, err := tk.CommandRun("sh", "-c", "echo file-content > file.txt"); err != nil {
      return fmt.Errorf("Error running command to create file: %w", err)
    } else if cmd == nil {
      return fmt.Errorf("Returned *exec.Cmd is nil")
    }

    // Assert the file exists within the spec's source directory,
    // implying the task was ran within the temporary spec
    // directory.
    //
    if _, err := os.Stat(filepath.Join(source_dir, "file.txt")); err != nil {
      return err
    }

    // Use a shell command to read the file content
    //
    var cat_output strings.Builder

    tk.Println("Reading file")
    cmd := tk.Command("cat", "file.txt")
    cmd.Stdout = &cat_output

    if err := cmd.Run(); err != nil {
      return fmt.Errorf("Error cat'ing file.txt: %w", err)
    }

    if got, expect := cat_output.String(), "file-content\n"; got != expect {
      return fmt.Errorf("cat output is %s, got %s", expect, got)
    }

    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }
}


func TestTaskEmitAsset (t *testing.T) {
  /*
    These strings encode definitions of simple task queues, each
    a test case. Each character represents another Task. All
    Funcs and MapFuncs are the same. Digits represent a task with
    one of the following configurations:
      0: No MapFunc, no Func
      1: No MapFunc,    Func
      2:    MapFunc, no Func
      3:    MapFunc,    Func
  */
  var test_cases = []struct {
    Tasks     string
    Expect    []byte
    WillError bool
  }{
    { Tasks:     "", Expect: []byte{0} }, // Test it works with nothing
    { Tasks:    "2", Expect: []byte{1} }, // Test a single map
    { Tasks: "2222", Expect: []byte{4} }, // Test cumulative map functions
    { Tasks:  "111", Expect: []byte{0, 0, 0, 0} },
    { Tasks:    "0", WillError: true },
    { Tasks:  "220", WillError: true },
  }

  var assetTaskMapFunc = func (a *Asset) (*Asset, error) {
    a.ContentBytes[len(a.ContentBytes)-1]++
    return a, nil
  }

  var assetTaskFunc = func (s *Spec, tk *Task) error {
    for _, asset_chunk := range tk.Assets {
      assets, err := asset_chunk.Flatten()
      if err != nil { return err }

      for _, asset := range assets {
        asset.ContentBytes = append(asset.ContentBytes, 0)
        tk.EmitAsset(asset)
      }
    }

    return nil
  }

  // Set up and run each test case
  //
  for test_case_i, test_case := range test_cases {
    t.Logf("Test case #%d: %v", test_case_i, test_case)

    var root *Spec = NewSpec(fmt.Sprintf("root-case-%d", test_case_i), nil)
    var spec *Spec = root.AddSubspec(NewSpec("spec", nil))

    root.Props["quiet"] = true

    root.EnqueueTaskFunc("root-consume", func (s *Spec, tk *Task) error {
      var num_assets = 0

      for {
        asset_chunk, err := tk.AwaitInputAssetNext()
        if err != nil {
          return err
        } else if asset_chunk == nil {
          break
        }

        assets, err := asset_chunk.Flatten()
        if err != nil { return err }

        for _, asset := range assets {
          num_assets++
          content_bytes, err := asset.GetContentBytes()
          if err != nil { t.Fatal(err) }

          if string(content_bytes) != string(test_case.Expect) {
            t.Fatalf(
              "Test case #%d expects an asset content of %v, got %v",
              test_case_i, test_case.Expect, content_bytes,
            )
          }
        }
      }

      expect_num_assets := 1
      if test_case.WillError {
        expect_num_assets = 0
      }

      if got := num_assets; got != expect_num_assets {
        t.Fatalf(
          "Test case #%d expected %d assets, got %d",
          test_case_i, expect_num_assets, got,
        )
      }
      return nil
    })

    spec.PushTaskFunc("produce", func (s *Spec, tk *Task) error {
      asset := s.MakeAsset("asset.txt")
      asset.SetContentBytes([]byte{ 0 })
      tk.EmitAsset(asset)
      return nil
    })

    // Decode the test case's task sequence into tasks in the task queue
    //
    for _, char := range test_case.Tasks {
      var task = & Task {}
      switch char {
      case '0':
        task.Name    = "empty"
      case '1':
        task.Name    = "func"
        task.Func    = assetTaskFunc
      case '2':
        task.Name    = "map"
        task.MapFunc = assetTaskMapFunc
      case '3':
        task.Name    = "map-func"
        task.Func    = assetTaskFunc
        task.MapFunc = assetTaskMapFunc
      }
      spec.PushTask(task)
    }

    // TODO: if this is ran with spec.Run() instead of root.Run(), it will timeout. This may be the sign of a problem.
    if err := root.Run(); err != nil {
      if test_case.WillError == false {
        t.Fatalf("Test case #%d errored: %v", test_case_i, err)
      }
    } else {
      if test_case.WillError {
        t.Fatalf("Test case #%d was expected to error, but did not", test_case_i)
      }
    }
  }
}


func TestTaskIgnoreAssets (t *testing.T) {
  var root = NewSpec("root", nil)

  var num_consumed_assets = 0

  root.EnqueueTaskFunc("produce-asset", func (s *Spec, tk *Task) error {
    return tk.EmitAsset(s.MakeAsset("asset"))
  })

  if err := root.EnqueueTask(& Task {
    Name: "ignore-assets",
    IgnoreAssets: true,
    Func: func (s *Spec, tk *Task) error {
      if len(tk.Assets) != 0 {
        t.Fatalf("IgnoreAssets task encountered an asset")
      }
      return nil
    },
  }); err != nil {
    t.Fatal(err)
  }

  root.EnqueueTaskFunc("consume-assets", func (s *Spec, tk *Task) error {
    num_consumed_assets = len(tk.Assets)
    if got, expect := num_consumed_assets, 1; got != expect {
      t.Fatalf("consume-assets expected %d assets, got %d", expect, got)
    }

    var asset = tk.Assets[0]
    var key = strings.TrimLeft(asset.Url.Path, "/")

    if got , expect := key, "asset"; got != expect {
      t.Fatalf("consume-assets got one asset; expected key to be %s, got %s",  expect, got)
    }

    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }

  if got, expect := num_consumed_assets, 1; got != expect {
    t.Fatalf("consume-assets expected %d assets, got %d", expect, got)
  }
}


func TestTaskPoolSpecInputAssetsWithNoInput (t *testing.T) {
  var spec = NewSpec("test-Task.PoolSpecInputAssets-no-input", nil)

  spec.EnqueueTaskFunc("pool-assets", func (sp *Spec, tk *Task) error {
    if err := tk.PoolSpecInputAssets(); err != nil {
      return err
    }
    return nil
  })

  if err := spec.Run(); err != nil {
    t.Error(err)
  }
}


func TestTaskEmitMultiAsset (t *testing.T) {
  var resolver_produce_asset_single = TaskResolver {
    Name: "produce-asset-singular",
    TaskPrototype: Task {
      AcceptMultiAssets: true,
      Func: func (s *Spec, tk *Task) error {
        if err := tk.ForwardAssets(); err != nil {
          return fmt.Errorf("Error forwarding assets: %w", err)
        }

        asset := s.MakeAsset("single")

        if err := tk.EmitAsset(asset); err != nil {
          return fmt.Errorf("Error emitting new single asset: %w", err)
        }

        return nil
      },
    },
  }

  var resolver_produce_asset_multi = TaskResolver {
    Name: "produce-asset-multi",
    TaskPrototype: Task {
      Func: func (s *Spec, tk *Task) error {
        if err := tk.ForwardAssets(); err != nil {
          return fmt.Errorf("Error forwarding assets: %w", err)
        }

        // Produce and emit a multi-asset with 10 assets.
        //
        asset := s.MakeAsset("multi")
        asset.SetAssetArray([]*Asset {
          s.MakeAsset("single"), s.MakeAsset("single"), s.MakeAsset("single"),
          s.MakeAsset("single"), s.MakeAsset("single"), s.MakeAsset("single"),
          s.MakeAsset("single"), s.MakeAsset("single"), s.MakeAsset("single"),
          s.MakeAsset("single"),
        })

        if err := tk.EmitAsset(asset); err != nil {
          return fmt.Errorf("Error emitting new single asset: %w", err)
        }

        return nil
      },
    },
  }

  // All tasks which map assets will use the
  // same mapping function
  //
  var task_map_func = func (a *Asset) (*Asset, error) {
    return a, nil
  }

  // Use TaskResolvers as task factories to test
  // different task sequences.
  //
  var resolver_map_multi = TaskResolver {
    Name: "map-accept-multi",
    TaskPrototype: Task {
      MapFunc:           task_map_func,
      AcceptMultiAssets: true,
    },
  }

  var resolver_map_flatten = TaskResolver {
    Name: "map-flatten",
    TaskPrototype: Task {
      MapFunc: task_map_func,
    },
  }

  var resolver_map_reject_multi = TaskResolver {
    Name: "map-reject-multi",
    TaskPrototype: Task {
      Name:                     "map-reject-multi",
      MapFunc:                  task_map_func,
      AcceptMultiAssets:        false,
      RejectFlattenMultiAssets: true,
    },
  }

  var test_cases = []struct {
    Tasks             []*TaskResolver;
    ExpectError       bool;
    NumSingularAssets int;
  }{

    { NumSingularAssets: 10,
      Tasks: []*TaskResolver { &resolver_produce_asset_multi },
    },

    { NumSingularAssets: 12,
      Tasks: []*TaskResolver {
        & resolver_produce_asset_single,
        & resolver_produce_asset_multi,
        & resolver_produce_asset_single,
      },
    },

    { ExpectError: true,
      Tasks: []*TaskResolver {
        & resolver_produce_asset_multi,
        & resolver_map_reject_multi,
      },
    },

    { NumSingularAssets: 11,
      Tasks: []*TaskResolver {
        & resolver_produce_asset_single,
        & resolver_produce_asset_multi,
        & resolver_map_flatten,
      },
    },

    { NumSingularAssets: 11,
      Tasks: []*TaskResolver {
        & resolver_produce_asset_single,
        & resolver_produce_asset_multi,
        & resolver_map_flatten,
        & resolver_map_reject_multi,
      },
    },

    { NumSingularAssets: 21,
      Tasks: []*TaskResolver {
        & resolver_produce_asset_single,
        & resolver_produce_asset_multi,
        & resolver_produce_asset_multi,
        & resolver_map_multi,
      },
    },
  }


  TEST_CASES:
  for test_case_i, test_case := range test_cases {
    fmt.Println("  ---  Initializing test case", test_case_i, " ---")
    var root = NewSpec(fmt.Sprintf("root_test-case-%d", test_case_i), nil)
    var spec = root.AddSubspec(
      NewSpec(fmt.Sprintf("subspec_test-case-%d", test_case_i),
      nil),
    )
    root.Props["quiet"] = true

    // Generate the task queue
    //
    for _, task_resolver := range test_case.Tasks {
      if err := spec.EnqueueTask(task_resolver.NewTask()); err != nil {
        t.Errorf("Error constructing task queue in test case %d: %v", test_case_i, err)
        continue TEST_CASES
      }
    }

    // Consume spec output and assert the test case's conditions
    //
    root.EnqueueTaskFunc("consume-assert", func (s *Spec, tk *Task) error {
      if err := tk.PoolSpecInputAssets(); err != nil {
        return err
      }

      if got, expect := len(tk.Assets), test_case.NumSingularAssets; got != expect {
        t.Errorf(
          "Test case %d expected %d singular assets total, got %d",
          test_case_i, expect, got,
        )
      }

      return nil
    })

    if err := root.Run(); (err != nil) != test_case.ExpectError {
      t.Log(SprintSpec(root))
      t.Log(err)
      if test_case.ExpectError {
        t.Errorf("Test case %d expected an error, but no error was returned", test_case_i)
      } else {
        t.Errorf("Test case %d exited with an error: %v", test_case_i, err)
      }
    }
  }
}


func TestTaskMaskEmit (t *testing.T) {
  // Create a task which cannot emit assets, and make sure it
  // errors when emitting an asset.
  //
  var task = Task {
    Name: "erroring-emitter-test-task",
    Mask: TASK_MASK_DEFINED,
  }

  if err := task.EmitAsset(& Asset {}); err == nil {
    t.Fatalf("Task expected to error (mask is %03O)", task.Mask)
  }

  // TODO: add Task to spec with a MapFunc, but mask set not to accept assets.

  var root = NewSpec("root", nil)
  root.Props["quiet"] = true

  root.EnqueueTask(& Task {
    Name: "only-emit",
    Mask: TASK_ASSETS_GENERATE,
    Func: func (sp *Spec, tk *Task) error {
      var asset = sp.MakeAsset("emit1.txt")
      return tk.EmitAsset(asset)
    },
  })

  root.EnqueueTask(& Task {
    Name: "ignore-assets",
    Mask: TASK_MASK_DEFINED,
    Func: func (sp *Spec, tk *Task) error {
      if length := len(tk.Assets); length > 0 {
        t.Errorf("Task set not to consume Assets had Assets in its buffer, with a length of %d Assets", length)
      }
      return nil
    },
  })

  root.EnqueueTask(& Task {
    Name: "emit-dont-consume",
    Mask: TASK_ASSETS_GENERATE,
    Func: func (sp *Spec, tk *Task) error {
      if err := tk.EmitAsset(sp.MakeAsset("emit2.txt")); err != nil {
        return fmt.Errorf("Error when emitting asset: %w", err)
      }

      if length := len(tk.Assets); length > 0 {
        t.Errorf("Task set not to consume Assets had Assets in its buffer, with a length of %d Assets", length)
      }

      if err := tk.PoolSpecInputAssets(); err == nil {
        t.Errorf("No error when pooling assets in a Task which should error when consuming assets.")
      }

      return nil
    },
  })

  root.EnqueueTask(& Task {
    Name: "consume-assets",
    Mask: TASK_ASSETS_CONSUME | TASK_ASSETS_FILTER_ALL,
    Func: func (sp *Spec, tk *Task) error {
      if err := tk.PoolSpecInputAssets(); err != nil {
        return err
      }

      if length, expect := len(tk.Assets), 2; length != expect {
        t.Errorf("Task expected to consume %d assets, got %d", expect, length)
      }

      return tk.ForwardAssets()
    },
  })

  if err := root.Run(); err != nil {
    t.Fatalf("Spec exitted with an error: %v", err)
  }
}
