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


func (t *Task) Append (a *Task) *Task {
  return t.End().Insert(a)
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


/*
  Searches this Spec, and recursively through its parents, for a
  TaskResolver with a specific ID. Returns nil if none is found
*/
func (s *Spec) GetTaskResolverById (id string) *TaskResolver {
  for tr := s.TaskResolvers ; tr != nil ; tr = tr.Next {
    if r := tr.GetTaskResolverById(id); r != nil {
      return r
    }
  }

  if s.Parent == nil {
    return nil
  }

  return s.Parent.GetTaskResolverById(id)
}


/*
  Append a TaskResolver to this Spec, taking priority over
  previously-added and parental resolvers
*/
func (s *Spec) AddTaskResolver (tr *TaskResolver) {
  var end *TaskResolver
  for end = tr ; end.Next != nil ; end = end.Next {}
  end.Next = s.TaskResolvers
  s.TaskResolvers = tr
}


func (s *Spec) GetTask (name string, spec *Spec) (*Task, error) {
  for resolver := s.TaskResolvers ; resolver != nil ; resolver = resolver.Next {
    task, err := resolver.GetTask(name, spec)
    if err != nil {
      return nil, fmt.Errorf("Error getting task in TaskResolver %s: %w", resolver.Id, err)
    }
    if task != nil {
      task.Spec = s
      return task, nil
    }
  }

  if s.Parent == nil {
    return nil, nil
  }

  task, err := s.Parent.GetTask(name, spec)
  if err != nil {
    return nil, err
  }

  if task != nil {
    task.Spec = s
  }
  return task, nil
}


func (s *Spec) NewTaskFunc (name string, f TaskFunc) *Task {
  return & Task {
    Spec: s,
    Name: name,
    Func: f,
  }
}


/*
  Insert a task into the task queue, before deferred tasks.
  Enqueued tasks are executed in first-in, first-out order, like
  a queue. Return the final inserted item.
*/
func (s *Spec) EnqueueTask (t *Task) *Task {
  s.task_queue_lock.Lock()
  defer s.task_queue_lock.Unlock()

  end := t.End()

  if s.Tasks == nil {
    s.Tasks = t
    s.tasks_enqueue_end = t
    return end
  }

  if s.tasks_enqueue_end == nil {
    end.Next = s.Tasks
    s.Tasks = t
    s.tasks_enqueue_end = end
    return end
  }

  s.tasks_enqueue_end = s.tasks_enqueue_end.insertRange(t, end)
  return end
}


/*
  EnqueueTaskFunc creates a new Task with the specified name and
  function (`f`), enqueues it for execution in the task queue,
  and returns it.
*/
func (s *Spec) EnqueueTaskFunc (name string, f TaskFunc) *Task {
  return s.EnqueueTask(s.NewTaskFunc(name, f))
}


/*
  Insert a task into the task queue, directly after the end of
  the enqueue end point. These tasks are executed in first-in,
  last-out order relative to other tasks in the queue, but if
  multiple tasks are inserted their order is maintained. Return
  the final inserted item.
*/
func (s *Spec) DeferTask (t *Task) *Task {
  s.task_queue_lock.Lock()
  defer s.task_queue_lock.Unlock()

  end := t.End()

  // If the spec tasks list is not yet defined, enqueued tasks
  // should still be executed before deferred tasks, so define
  // the tasks list, but do not define an end to the enqueue
  // point. This will cause enqueuing to insert into the top of
  // the task list.
  //
  if s.Tasks == nil {
    s.Tasks = t
    s.tasks_enqueue_end = nil
    return end
  }

  if s.tasks_enqueue_end == nil {
    s.tasks_enqueue_end = s.Tasks.End()
  }

  return s.tasks_enqueue_end.insertRange(t, end)
}


/*
  DeferTaskFunc creates a new Task with the specified name and
  function (`f`), defers it for execution in the task queue,
  and returns it.
*/
func (s *Spec) DeferTaskFunc (name string, f TaskFunc) *Task {
  return s.DeferTask(s.NewTaskFunc(name, f))
}



/*
  PushTask adds a Task to the push queue. The push queue is a
  temporary holding area for tasks that need to be executed
  immediately before other tasks in the main queue. When the task
  execution loop begins, and after each tasks, all tasks in the
  push queue are flushed into the main task queue to be executed
  next. This function returns the final inserted item in the push
  queue.
*/
func (s *Spec) PushTask (t *Task) *Task {
  end := t.End()

  if s.tasks_push_queue == nil || s.tasks_push_end == nil {
    s.tasks_push_queue = t
    s.tasks_push_end   = end
    return end
  }

  s.tasks_push_end = s.tasks_push_end.insertRange(t, end)
  return end
}


/*
  PushTaskFunc creates a new Task with the specified name and
  function (`f`), pushs it for execution in the task queue,
  and returns it.
*/
func (s *Spec) PushTaskFunc (name string, f TaskFunc) *Task {
  return s.PushTask(s.NewTaskFunc(name, f))
}


/*
  EnqueueTaskName retrieves a Task by its name and enqueues it
  for execution. If the task is found, it is added to the task
  queue and the last inserted Task is returned.  If the task
  cannot be found, it is returned as nil. If an error occurs, an
  error is returned.
*/
func (s *Spec) EnqueueTaskName (name string) (*Task, error) {
  task, err := s.GetTask(name, s)
  if task == nil || err != nil {
    return nil, err
  }
  return s.EnqueueTask(task), nil
}

 
/*
  EnqueueUniqueTask enqueues a Task only if there isn't already a
  task with the same name in the task queue. If a task with the
  same name already exists, it returns the existing task without
  modifying the task queue. Otherwise, it enqueues the provided
  task and returns the final enqueued Task.
*/
func (s *Spec) EnqueueUniqueTask (t *Task) (*Task, error) {
  if t.Name == "" {
    return nil, fmt.Errorf("EnqueueUniqueTask error: task's name is empty")
  }

  // TODO: check the push queue for matching tasks
  existing_task := s.GetTaskFromQueue(t.Name)
  if existing_task != nil {
    return existing_task, nil
  }

  return s.EnqueueTask(t), nil
}


/*
  EnqueueUniqueTaskName enqueues a Task by its name only if there isn't
  already a task with the same name in the task queue. If a task with
  the same name already exists, it returns the existing task without
  enqueuing a new one.
*/
func (s *Spec) EnqueueUniqueTaskName (name string) (*Task, error) {
  existing_task := s.GetTaskFromQueue(name)
  if existing_task != nil {
    return existing_task, nil
  }
  return s.EnqueueTaskName(name)
}


/*
  GetTaskFromQueue searches the task queue for a task with
  the specified name and returns it. If no such task is found, it
  returns nil.
*/
func (s *Spec) GetTaskFromQueue (name string) *Task {
  // TODO: check the push queue
  for task := s.Tasks ; task != nil ; task = task.Next {
    if task.Name == name {
      return task
    }
  }
  return nil
}


func (s *Spec) flushTaskPushQueue () *Task {
  start := s.tasks_push_queue
  end   := s.tasks_push_end

  s.tasks_push_queue = nil
  s.tasks_push_end   = nil

  if start == nil || end == nil {
    return nil
  }

  // If the current task has not been defined,
  // flush to the top of the task queue.
  //
  if s.CurrentTask == nil {
    if s.Tasks == nil {
      s.Tasks = start
      s.tasks_enqueue_end = end
      return end
    }

    end.Next = s.Tasks
    s.Tasks = start
    return end
  }

  return s.CurrentTask.insertRange(start, end)
}
