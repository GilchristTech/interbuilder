package interbuilder

import (
  "testing"
  "fmt"
  "strings"
  "os"
  "path/filepath"
  "sort"
)


func TestSpecGetTaskResolverById (t *testing.T) {
  // Spec and TaskResolver structure
  //
  var root    *Spec = NewSpec("root", nil)
  var subspec *Spec = root.AddSubspec(NewSpec("subspec", nil))

  var resolver_4b = & TaskResolver { Id: "task_name_4" }
  var resolver_4a = & TaskResolver { Id: "task_name_4" }
  var resolver_3  = & TaskResolver { Id: "task_name_3" }
  var resolver_2  = & TaskResolver { Id: "task_name_2", Next: resolver_4a }
  var resolver_1  = & TaskResolver { Id: "task_name_1", Next: resolver_2 }

  var resolver_0 = & TaskResolver {
    Id:       "task_name_0",
    Children: resolver_1,
    Next:     resolver_3,
  }

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
