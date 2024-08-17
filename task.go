package interbuilder

import (
  "fmt"
  "os/exec"
  "os"
  "net/url"
  "strings"
)

type TaskFunc      func (*Spec, *Task) error
type TaskMatchFunc func (name string, spec *Spec) (bool, error)


type TaskResolver struct {
  Name          string
  Url           url.URL
  Id            string
  MatchFunc     TaskMatchFunc
  TaskPrototype Task

  Spec          *Spec
  History       HistoryEntry

  Next          *TaskResolver
  Children      *TaskResolver
}


type Task struct {
  Spec       *Spec
  ResolverId string
  Resolver   *TaskResolver
  Name       string
  Started    bool
  Func       TaskFunc
  Next       *Task
  History    HistoryEntry;
}


func (t *Task) Println (a ...any) (n int, err error) {
  var spec_name = "<nil>"

  if t.Spec != nil {
    if quiet, _, _ := t.Spec.InheritPropBool("quiet"); quiet {
      return 0, nil
    }
    spec_name = t.Spec.Name
  }

  var stdout_prefix = "[" + spec_name + "/" + t.Name + "] "
  var content string = fmt.Sprintln(a...)
  content = content[:len(content)-1]  // Trip newline
  content = stdout_prefix + strings.ReplaceAll(content, "\n", "\n"+stdout_prefix)
  return fmt.Println(content)
}


func (tr *TaskResolver) GetTaskResolverById (id string) *TaskResolver {
  if tr.Id == id {
    return tr
  }

  for child := tr.Children ; child != nil ; child = child.Next {
    if r := child.GetTaskResolverById(id); r != nil {
      return r
    }
  }

  return nil
}


/*
  Returns a pointer to this TaskResolver or a child, whichever has greater
  specificity in the resolver tree. If no matching resolver is found, return
  nil instead.
*/
func (tr *TaskResolver) Match (name string, s *Spec) (*TaskResolver, error) {
  if tr.MatchFunc == nil {
    if tr.Name != name {
      return nil, nil
    }

    child_match, err := tr.MatchChildren(name, s)
    if err != nil { return nil, err }
    if child_match != nil {
      return child_match, nil
    }

    return tr, nil
  }

  tr_matches, err := tr.MatchFunc(name, s)
  if !tr_matches ||  err != nil {
    return nil, err
  }

  child_match, err := tr.MatchChildren(name, s)
  if err != nil { return nil, err }
  if child_match != nil {
    return child_match, nil
  }

  return tr, nil
}


func (tr *TaskResolver) MatchChildren (name string, spec *Spec) (*TaskResolver, error) {
  for child := tr.Children ; child != nil ; child = child.Next {
    child_match, err := child.Match(name, spec)
    if err != nil {
      return nil, err
    }
    if child_match != nil {
      return child_match, nil
    }
  }

  return nil, nil
}


func (tr *TaskResolver) NewTask() *Task {
  var task Task = tr.TaskPrototype  // shallow copy

  task.Name       = tr.Name
  task.ResolverId = tr.Id
  task.Resolver   = tr

  return &task
}


func (tr *TaskResolver) GetTask (name string, s *Spec) (*Task, error) {
  resolver, err := tr.Match(name, s)
  if resolver == nil || err != nil {
    return nil, err
  }
  if resolver.TaskPrototype.Func == nil {
    return nil, fmt.Errorf("Task resolver has a null task function")
  }
  return resolver.NewTask(), nil
}


func (t *Task) Insert (i *Task) *Task {
  return t.insertRange(i, i.End())
}


func (t *Task) insertRange (i_start, i_end *Task) *Task {
  i_end.Next = t.Next
  t.Next = i_start
  return i_end
}


func (t *Task) End () *Task {
  for ;; t = t.Next {
    if t.Next == nil { return t }
  }
}


func (t *Task) GetCircularTask () *Task {
  task_pointers := make(map[*Task]bool)

  for ; t != nil ; t = t.Next {
    if _, found := task_pointers[t]; found {
      return t
    }

    task_pointers[t] = true
  }

  return nil
}


func (t *Task) Command (name string, args ...string) *exec.Cmd {
  cmd := exec.Command(name, args...)

  // TODO: get/inherit environment variables

  // Inherity working directory from source_dir prop
  //
  if t.Spec != nil {
    cmd.Dir, _, _ = t.Spec.InheritPropString("source_dir")
  }

  return cmd
}


func (t *Task) CommandRun (name string, args ...string) (*exec.Cmd, error) {
  cmd := t.Command(name, args...)

  spec_name := "<nil>"
  if t.Spec != nil {
    spec_name = t.Spec.Name
  }

  // Redirect output to prefixed wrapper of STDOUT/STDERR
  // TODO: break this out into its own method
  //
  stdout_prefix := "[" + spec_name + "/" + t.Name + "] "
  stderr_prefix := "{" + spec_name + "/" + t.Name + "} "

  stdout, err := cmd.StdoutPipe()
  if err != nil { return cmd, err }
  stderr, err := cmd.StderrPipe()
  if err != nil { return cmd, err }

  StreamPrefix(stdout, os.Stdout, stdout_prefix)
  StreamPrefix(stderr, os.Stderr, stderr_prefix)

  fmt.Print(stdout_prefix, "$ ", name, " ", strings.Join(args, " "), "\n")
  return cmd, cmd.Run()
}


func (t *Task) Run (s *Spec) error {
  if t.Func == nil {
    return fmt.Errorf("Error: Task.Func is nil")
  }

  t.Started = true
  return t.Func(s, t)
}
