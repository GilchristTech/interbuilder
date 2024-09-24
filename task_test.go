package interbuilder

import (
  "testing"
  "fmt"
  "strings"
  "os"
  "path/filepath"
  "sort"
  "net/url"
)


func TestSpecGetTaskResolverById (t *testing.T) {
  // Spec and TaskResolver structure
  //
  var root    *Spec = NewSpec("root", nil)
  var subspec *Spec = root.AddSubspec(NewSpec("subspec", nil))

  root.Props["quiet"] = true

  var resolver_4b = & TaskResolver { Id: "task_name_4" }
  var resolver_4a = & TaskResolver { Id: "task_name_4" }
  var resolver_3  = & TaskResolver { Id: "task_name_3" }
  var resolver_2  = & TaskResolver { Id: "task_name_2" }
  var resolver_1  = & TaskResolver { Id: "task_name_1", Next: resolver_2 }

  var resolver_0 = & TaskResolver {
    Id:       "task_name_0",
    Next:     resolver_3,
  }

  resolver_0.AddTaskResolver(resolver_1)
  resolver_2.AddTaskResolver(resolver_4a)

  root.AddTaskResolver(resolver_0)
  subspec.AddTaskResolver(resolver_4b)

  assertGetTaskResolverById := func (s *Spec, name string, expect *TaskResolver) {
    if got := s.GetTaskResolverById(name); got == nil {
      t.Errorf("Expected a task resolver with ID %s, got <nil>", expect.Id)
    } else if got != expect {
      t.Errorf("Expected task resolver with ID %s, got a resolver with ID %s", expect.Id, got.Id)
    }
  }

  // Test getting a top-level task resolver
  //
  assertGetTaskResolverById(root,    "task_name_0", resolver_0)
  assertGetTaskResolverById(subspec, "task_name_0", resolver_0)

  // Test getting a top-level sibling task resolver
  //
  assertGetTaskResolverById(root,    "task_name_3", resolver_3)
  assertGetTaskResolverById(subspec, "task_name_3", resolver_3)

  // Test nonexistant task resolver
  //
  if got := subspec.GetTaskResolverById("does-not-exist"); got != nil {
    t.Errorf("Expected a <nil> task resolver, instead got a resolver with ID %s", got.Id)
  }

  // Test getting a child task resolver
  //
  assertGetTaskResolverById(root,    "task_name_1", resolver_1)
  assertGetTaskResolverById(subspec, "task_name_1", resolver_1)

  // Test getting a child-sibling task resolver
  //
  assertGetTaskResolverById(root,    "task_name_2", resolver_2)
  assertGetTaskResolverById(subspec, "task_name_2", resolver_2)

  // Task subspec taking precident over parent
  //
  assertGetTaskResolverById(root,    "task_name_4", resolver_4a)
  assertGetTaskResolverById(subspec, "task_name_4", resolver_4b)
}


func TestSpecGetTask (t *testing.T) {
  var root    *Spec = NewSpec("root", nil)
  var subspec *Spec = root.AddSubspec(NewSpec("subspec", nil))

  var task_root_match_attempts int = 0

  var tr_error = & TaskResolver {
    Name: "task-error",
    Id:   "task-error",
    MatchFunc: func (name string, spec *Spec) (bool, error) {
      if name == "task-error" {
        return false, fmt.Errorf("When this resolver matches, it is supposed to error")
      }
      return false, nil
    },
  }

  var tr_root_child = & TaskResolver {
    Name: "task", 
    Id:   "task",
    Next: tr_error,
    TaskPrototype: Task {
      Name: "task",
      Func: func (*Spec, *Task) error { return nil },
    },
  }

  var tr_root = & TaskResolver {
    Name:     "task",
    Id:       "task-root",
    Children: tr_root_child,
    MatchFunc: func (name string, spec *Spec) (bool, error) {
      task_root_match_attempts++
      return strings.HasPrefix(name, "task"), nil
    },
  }

  var tr_override = & TaskResolver {
    Name: "task",
    Id:   "task-root-override",
    TaskPrototype: Task {
      Name: "task",
      Func: func (*Spec, *Task) error { return nil },
    },
  }

  root.AddTaskResolver(tr_root)
  subspec.AddTaskResolver(tr_override)

  // Assert getting a non-existant task
  //
  if task, err := subspec.GetTask("doesnt-exist", subspec); err != nil {
    t.Error(err)
  } else if task != nil {
    t.Errorf("Expected task to be nil, got a task with name %s", task.Name)
  }

  // Assert getting a task, first matching through a parent, then
  // going through children
  //
  if task, err := root.GetTask("task", root); err != nil {
    t.Error(err)
  } else if task == nil {
    t.Errorf("Expected to get a task with the name \"task\", instead got nil")
  } else if got := task.Resolver.Id; got != "task" {
    t.Errorf("Expected task resolver to be task-root, got %s", got)
  }

  // Assert getting a task which takes priority over a parent
  //
  if task, err := subspec.GetTask("task", root); err != nil {
    t.Error(err)
  } else if task == nil {
    t.Errorf("Expected to get a task with the name \"task\", instead got nil")
  } else if got := task.Resolver.Id; got != "task-root-override" {
    t.Errorf("Expected task resolver to be task-root-override, got %s", got)
  }

  // Assert the number of times the root spec's root task
  // resolver's match function was ran
  //
  if got, expect := task_root_match_attempts, 2; got != expect {
    t.Errorf("Expected task_root's match function to be ran %d times, got %d", expect, got)
  }

  // Assert that matching errors are propagated
  //
  error_task, err := subspec.GetTask("task-error", subspec)
  if err == nil {
    t.Error("Getting the error task did not return an error")
  }
  if error_task != nil {
    t.Errorf("Getting the error resolver returned a task with the name %s", error_task.Name)
  }
}


func TestSpecTaskQueue (t *testing.T) {
  // TODO: add a task which modifies the task queue, instead of the order of tasks being mostly evaluated prior to task execution.

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
  var task_func = func (s *Spec, tk *Task) error {
    task_log = append(task_log, tk.Name)
    return nil
  }

  root.DeferTaskFunc("task_z", task_func)
  root.EnqueueTaskFunc("task_n", task_func)
  root.DeferTaskFunc("task_y", task_func)
  root.EnqueueTaskFunc("task_o", task_func)
  root.DeferTaskFunc("task_x", task_func)
  root.EnqueueTaskFunc("task_p", task_func)

  var task_a = root.NewTaskFunc("task_a", task_func)
  task_a.Append( root.NewTaskFunc("task_b", task_func) )
  task_a.Append( root.NewTaskFunc("task_c", task_func) )
  root.PushTask(task_a)
  root.PushTaskFunc("task_d", task_func)

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }

  if got, expect := len(task_log), 10; got != expect {
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

      for { select {
      case <- tk.CancelChan:
        return nil

      case asset_chunk, ok := <- s.Input:
        if !ok {
          return nil
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
      }}

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
      var task *Task = spec.NewTask("")
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

  ignore_task := root.EnqueueTaskFunc("ignore-assets", func (s *Spec, tk *Task) error {
    if len(tk.Assets) != 0 {
      t.Fatalf("IgnoreAssets task encountered an asset")
    }
    return nil
  })
  ignore_task.IgnoreAssets = true

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


func TestTaskEmitMultiAsset (t *testing.T) {
  var resolver_produce_asset_single = TaskResolver {
    Name: "produce-asset-singular",
    TaskPrototype: Task {
      AcceptMultiAssets: true,
      Func: func (s *Spec, tk *Task) error {
        tk.PoolSpecInputAssets()

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

        // Produce and emit a multi-asset with ten assets.
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


  for test_case_i, test_case := range test_cases {
    var root = NewSpec("root", nil)
    var spec = root.AddSubspec(NewSpec("spec", nil))

    // Generate the task queue
    //
    for _, task_resolver := range test_case.Tasks {
      spec.EnqueueTask(task_resolver.NewTask())
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


func TestTaskResolverMatchWithAsset (t *testing.T) {
  var task_resolver_txt_specific = & TaskResolver {
    Id: "task-resolver-specific",
    Name: "test-resolver",
    TaskPrototype: Task {
      MatchMimePrefix: "text/plain",
      MatchFunc: func (tk *Task, a *Asset) (bool, error) {
        if a.Url == nil {
          return false, fmt.Errorf("Asset does not have a defined URL")
        }
        var url_key string = a.Url.Path
        return strings.Contains(url_key, "specific"), nil
      },
    },
  }

  var task_resolver_txt = & TaskResolver {
    Id: "test-resolver-root",
    Name: "test-resolver",
    Children: task_resolver_txt_specific,
    TaskPrototype: Task {
      MatchMimePrefix: "text/plain",
    },
  }

  base_url, _ := url.Parse("ib://test")

  var asset_file_txt = & Asset {
    Url:      base_url.JoinPath("file.txt"),
    Mimetype: "text/plain",
  }

  var asset_specific_file_txt = & Asset {
    Url:      base_url.JoinPath("specific-file.txt"),
    Mimetype: "text/plain",
  }

  if resolver_match, err := task_resolver_txt.MatchWithAsset(asset_file_txt); err != nil {
    t.Fatal(err)
  } else if got := resolver_match; got == nil {
    t.Fatalf("Task resolver is nil")
  } else if expect := task_resolver_txt; got != expect {
    t.Fatalf("Expected task resolver with ID %s, got %s", expect.Id, got.Id)
  }

  if resolver_match, err := task_resolver_txt.MatchWithAsset(asset_specific_file_txt); err != nil {
    t.Fatal(err)
  } else if got := resolver_match; got == nil {
    t.Fatalf("Task resolver is nil")
  } else if expect := task_resolver_txt_specific; got != expect {
    t.Fatalf("Expected task resolver with ID %s, got %s", expect.Id, got.Id)
  }
}
