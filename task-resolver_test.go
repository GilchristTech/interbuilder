package interbuilder

import (
  "testing"
  "fmt"
  "strings"
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




func TestTaskResolverAcceptMask (t *testing.T) {
  var root_resolver = & TaskResolver {
    Id: "accepts-nothing",
    AcceptMask: TASK_MASK_DEFINED,
  }

  fmt.Println("...")
  if err := root_resolver.AddTaskResolver(& TaskResolver {
    Id: "undefined-mask",
  }); err == nil {
    t.Fatal("TaskResolver with undefined (all permissions) Task Mask was added to Task Resolver with a minimal AcceptanceMask and an error was not produced.")
  }

  if err := root_resolver.AddTaskResolver(& TaskResolver {
    Id: "no-permissions",
    TaskPrototype: Task {
      Mask: TASK_MASK_DEFINED,
    },
  }); err != nil {
    t.Fatal(err)
  }
}
