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
  Name       string
  Started    bool
  Func       TaskFunc
  Next       *Task
  History    HistoryEntry;
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

    for child := tr.Children ; child != nil ; child = child.Next {
      child_match, err := child.Match(name, s)
      if err != nil {
        return nil, err
      }
      if child_match != nil {
        return child_match, nil
      }
    }

    return tr, nil
  }

  tr_matches, err := tr.MatchFunc(name, s)
  if !tr_matches ||  err != nil {
    return nil, err
  }

  for child := tr.Children ; child != nil ; child = child.Next {
    matched_resolver, err := child.Match(name, s)
    if err != nil {
      return nil, err
    }
    if matched_resolver != nil {
      return matched_resolver, nil
    }
  }

  return tr, nil
}


func (tr *TaskResolver) NewTask() *Task {
  task           := tr.TaskPrototype  // shallow copy
  task.Name       = tr.Name
  task.ResolverId = tr.Id
  return &task
}


func (tr *TaskResolver) GetTask (name string, s *Spec) (*Task, error) {
  resolver, err := tr.Match(name, s)
  if resolver == nil || err != nil {
    return nil, err
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


func (t *Task) GetProp (key string) (value any, found bool) {
  if t.Spec == nil {
    return nil, false
  }

  if value, found := t.Spec.Props[key] ; found {
    return value, found
  }
  return nil, false
}


func (t *Task) GetPropString (key string) (value string, ok, found bool) {
  if t.Spec == nil {
    return "", false, false
  }

  value_any, found := t.Spec.Props[key]
  value,     ok     = value_any.(string)
  return value, ok, found
}


func (t *Task) RequireProp (name string) (value any, err error) {
  if t.Spec == nil {
    return nil, fmt.Errorf(
      "Task %s (%s) does not have Spec defined",
      t.Name, t.ResolverId,
    )
  }

  value, found := t.Spec.Props[name]

  if !found {
    return nil, fmt.Errorf(
      "Task %s/%s (%s) requires spec prop %s to exist",
      t.Spec.Name, t.Name, t.ResolverId, name,
    )
  }

  return value, nil
}


func (t *Task) RequireInheritProp (name string) (value any, err error) {
  if t.Spec == nil {
    return nil, fmt.Errorf(
      "Task %s (%s) does not have Spec defined",
      t.Name, t.ResolverId,
    )
  }

  value, found := t.Spec.InheritProp(name)

  if found == false {
    return nil, fmt.Errorf(
      "Task %s/%s (%s) requires inherited spec prop %s to exist",
      t.Spec.Name, t.Name, t.ResolverId, name,
    )
  }

  return value, nil
}



func (t *Task) RequireInheritPropString (key string) (string, error) {
  value_any, err := t.RequireInheritProp(key)
  if err != nil {
    return "", err
  }

  value, ok := value_any.(string)
  if ok == false {
    return "", fmt.Errorf(
      "Task %s/%s (%s) requires inherited spec prop %s to be a string, got %T",
      t.Spec.Name, t.Name, t.ResolverId, key, t.Spec.Props[key],
    )
  }

  return value, nil
}


func (t *Task) RequirePropString (name string) (value string, err error) {
  value_any, err := t.RequireProp(name)
  if err != nil {
    return "", err
  }

  value, ok := value_any.(string)
  if !ok {
    return "", fmt.Errorf(
      "Task %s/%s (%s) requires spec prop %s to be a string, got %T",
      t.Spec.Name, t.Name, t.ResolverId, name, t.Spec.Props[name],
    )
  }

  return value, nil
}


func (t *Task) RequirePropURL (name string) (value *url.URL, err error) {
  value_any, err := t.RequireProp(name)
  if err != nil {
    return nil, err
  }

  switch value := value_any.(type) {
  case string:
    return url.Parse(value)
  case *url.URL:
    return value, nil
  case url.URL:
    return &value, nil
  }

  return nil, fmt.Errorf(
    "Task %s/%s (%s) requires spec prop %s to be a URL string, url.URL, or *url.URL, got %T",
    t.Spec.Name, t.Name, t.ResolverId, name, t.Spec.Props[name],
  )
}
