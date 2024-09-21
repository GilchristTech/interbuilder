package interbuilder

import (
  "fmt"
  "os/exec"
  "os"
  "net/url"
  "strings"
)

type TaskFunc      func (*Spec, *Task) error
type TaskMapFunc   func (*Asset) (*Asset, error)
type TaskMatchFunc func (name string, spec *Spec) (bool, error)


/*
  TaskResolvers are a hierarchical system of matching conditions
  for Tasks, and act as the factories to build them. When a Spec
  is told to search for a Task, it will navigate its own
  TaskResolver tree, and if a match isn't found, it will look
  among its parents. 
*/
type TaskResolver struct {
  Name          string
  Url           url.URL
  Id            string
  TaskPrototype Task  // TODO: consider renaming. TaskTemplate, perhaps? Also consider making private.

  Spec          *Spec
  History       HistoryEntry

  Next          *TaskResolver
  Children      *TaskResolver

  MatchBlocks   bool
  MatchFunc     TaskMatchFunc
}


/*
  Tasks are the operational units of Interbuilder. Specs maintain
  a queue of these tasks, which run system commands and
  manipulate Assets. Tasks act as a singly-linked list of callback
  functions (a Spec's Task queue), which also move Assets forward
  in the list. There are two types of callback functions, which
  are fields in the Task struct: Func and MapFunc.
*/
type Task struct {
  Spec       *Spec
  ResolverId string
  Resolver   *TaskResolver
  Name       string
  Started    bool
  Next       *Task
  History    HistoryEntry

  Assets     []*Asset

  // Func task callback functions only run when this task is
  // reached in the Task queue. It can access its internal Assets
  // array in a current, complete state, and can modify the
  // future elements of the Task queue in a thread-safe way. Func
  // Task callbacks are the serial execution mechanism of a
  // Spec's Task queue.
  //
  Func       TaskFunc
  
  // MapFunc task callback functions are ran over every Asset
  // emitted to this Task, and can be executed as part of the
  // emitting algorithm before a task is reached within the Task
  // queue.
  //
  MapFunc    TaskMapFunc

  CancelChan chan bool

  /*
    Asset matching: used in conjunction with a MapFunc, the
    matching operands below are used to evaluate whether a given
    asset can be received by the MapFunc, or passed to the next
    Task.
  */
  MatchFunc       func (*Task, *Asset) (bool, error)
  MatchMimePrefix string

  /*
    Asset quantity handling: Assets are capable of representing
    either a single Asset, or act like a promise to expand into
    more Assets. Because tasks can filter what 
  */

  // When receiving a multi-asset, by default, if a Task does not
  // accept them, it should flatten the asset. However, a Task
  // can turn this behavior off with the RejectFlattenMultiAssets.
  //
  RejectFlattenMultiAssets bool

  // AcceptMultiAssets allows a Task to receive multi-assets the
  // same way it would receive singular assets.
  //
  AcceptMultiAssets bool
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

    if tr.MatchBlocks == false {
      child_match, err := tr.MatchChildren(name, s)
      if err != nil { return nil, err }
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

  if tr.MatchBlocks == false {
    child_match, err := tr.MatchChildren(name, s)
    if err != nil { return nil, err }
    if child_match != nil {
      return child_match, nil
    }
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
  if resolver.TaskPrototype.Func == nil && resolver.TaskPrototype.MapFunc == nil {
    return nil, fmt.Errorf("Task resolver has a nil Func and MapFunc")
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


func (tk *Task) Run (s *Spec) error {
  if tk.MapFunc == nil && tk.Func == nil {
    return fmt.Errorf("Both Task.Func and Task.MapFunc are nil")
  }

  if tk.Func == nil {
    return nil
  }

  tk.Started = true
  return tk.Func(s, tk)
}


/*
  Searches this Spec, and recursively through its parents, for a
  TaskResolver with a specific ID. Returns nil if none is found.
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
  previously-added and parental resolvers.
*/
func (s *Spec) AddTaskResolver (tr *TaskResolver) {
  var end *TaskResolver
  for end = tr ; end.Next != nil ; end = end.Next {}
  end.Next = s.TaskResolvers
  s.TaskResolvers = tr
}


/*
  Append a TaskResolver to this TaskResolver as a child, taking
  priority over previously-added and sub-resolvers.
*/
func (tr *TaskResolver) AddTaskResolver (add *TaskResolver) {
  // Search for the last sibling
  //
  var last_sibling *TaskResolver = add
  for ; last_sibling.Next != nil ; last_sibling = last_sibling.Next {}

  last_sibling.Next = tr.Children

  if add != last_sibling {
    add.Next = last_sibling
  }

  tr.Children = add
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


func (s *Spec) NewTask (name string) *Task {
  return & Task  {
    Spec: s,
    Name: name,
  }
}


func (s *Spec) NewTaskFunc (name string, f TaskFunc) *Task {
  return & Task {
    Spec: s,
    Name: name,
    Func: f,
  }
}


func (s *Spec) NewTaskMapFunc (name string, f TaskMapFunc) *Task {
  return & Task {
    Spec: s,
    Name: name,
    MapFunc: f,
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

  t.Spec = s

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
  EnqueueTaskMapFunc creates a new Task with the specified name
  and asset map function (`f`), enqueues it for execution in the
  task queue, and returns it.
*/
func (s *Spec) EnqueueTaskMapFunc (name string, f TaskMapFunc) *Task {
  return s.EnqueueTask(s.NewTaskMapFunc(name, f))
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
  t.Spec = s

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
  t.Spec = s

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


/*
  PassAsset sends an Asset to subsequent assets. As long
  as there are Tasks with only MapFuncs, the asset will have
  those map function applied, and Tasks with normal Funcs will
  have the asset deposited into their asset array and block the
  passing until that Task is reached in the Spec's Task queue. If
  an Asset makes it all the way through the Task queue, it is
  emitted.
*/
func (tk *Task) EmitAsset (a *Asset) error {
  var asset *Asset = a
  var err   error

  // If this is the final task, the only place left for the asset
  // to go is being emitted by the Spec. Do so if it exists.
  //
  if tk.Next == nil {
    if tk.Spec != nil {
      if err := tk.Spec.EmitAsset(a); err != nil {
        return fmt.Errorf("Error in task %s emitting asset: %w", tk.Name, err)
      }
    }
    return nil
  }

  // There is a task after this one.
  var next *Task = tk.Next

  // Handle pluralistic assets
  //
  if a.IsMulti() {

    // This pluralistic asset can be handled the same way as a
    // singular asset. Exit any special multi-asset handling.
    //
    if next.AcceptMultiAssets {
      goto EXIT_IS_MULTI
    }

    // The next asset does not accept multi-assets, but it may
    // allow for multi-assets to be flattened. If so, flatten;
    // otherwise, error because a way of handling this
    // multi-asset was not found.
    //
    if next.RejectFlattenMultiAssets == false {
      if assets, err := a.Flatten(); err != nil {
        return err
      } else {
        for _, asset := range assets {
          if err := tk.EmitAsset(asset); err != nil {
            return err
          }
        }
      }
      return nil
    }

    return fmt.Errorf("Cannot pass from task %s to %s, asset is not singular and the task cannot receive multi assets", tk.Name, next.Name)
  }
  EXIT_IS_MULTI:


  // Check the next Task's asset filters. If this asset does not
  // match the next task, then let the next task pass this asset
  // without handling it.
  //
  if matches, err := next.MatchAsset(asset); err != nil {
    return err
  } else if matches == false {
    return next.EmitAsset(asset)
  }

  // This asset matches in the next task.

  // If the next task has no MapFunc, deposit it into that task's
  // Asset buffer and exit.
  //
  if next.MapFunc == nil {
    next.AddAsset(asset)
    return nil
  }

  // This asset matches the next task, and that task has a
  // MapFunc. Apply the map function and replace the asset with a
  // new reference.
  //
  asset, err = next.MapFunc(asset)
  if err != nil {
    return fmt.Errorf("Error in task %s MapFunc: %w", next.Name, err)
  }
  if asset == nil { return nil }

  // With the new asset, if the next task has a Func, then it is
  // the destination, since the Func may mutate the asset via its
  // task buffer.
  //
  if next.Func != nil {
    next.AddAsset(asset)
    return nil
  }

  // There is a next task, we have a valid (map-function-applied)
  // asset for it, and this task has no Func to mutate the asset
  // further. Recurse, sending the asset as far as it can go in
  // the Task without requiring the task queue to synchronize up
  // until that point.
  //
  return next.EmitAsset(asset)
}


/*
  PoolSpecInputAssets reads the Spec input channel for asset
  chunks and inserts them into the Task's Asset array. Note:
  because this blocks until all input is received, it can be less
  efficient than using a range over the Input channel.
*/
func (tk *Task) PoolSpecInputAssets () error {
  if tk.Spec == nil {
    return fmt.Errorf("Task Spec is nil")
  }

  for asset_chunk := range tk.Spec.Input {
    if asset_chunk.IsSingle() || tk.AcceptMultiAssets {
      tk.Assets = append(tk.Assets, asset_chunk)
      continue
    }

    // This is a multi-asset, and this task does not accept
    // multi-assets.

    if ! tk.RejectFlattenMultiAssets {
      if assets, err := asset_chunk.Flatten(); err != nil {
        return err
      } else {
        tk.Assets = append(tk.Assets, assets...)
      }
      continue
    }

    return fmt.Errorf("This task does not have a way of receiving a multi-asset")
  }

  return nil
}


/*
  ForwardAssets emits all assets from this Task's internal Assets
  array into the next task or spec, returning an error if one
  occurs.
*/
func (tk *Task) ForwardAssets () error {
  if tk.Spec == nil {
    return fmt.Errorf("Task Spec undefined")
  }

  // If a multi-asset can be used, create one with the Assets
  // slice and emit it. This is likely more efficient than
  // iterating Assets and emitting them individually.
  //
  if tk.Next == nil || tk.Next.AcceptMultiAssets {
    asset := tk.Spec.MakeAsset("")
    asset.SetAssetArray(tk.Assets)
    return tk.EmitAsset(asset)
  }

  // There is a next task, it does not accept multi-assets.
  // Emit all assets.
  //
  for _, asset := range tk.Assets {
    if err := tk.EmitAsset(asset); err != nil {
      return fmt.Errorf("Error while forwarding an asset: %w", err)
    }
  }

  return nil
}


/*
  AddAsset adds an asset to the Task's internal asset buffer.
  Returns the asset. This does not perform any validation of the
  Asset or Task.
*/
func (tk *Task) AddAsset (a *Asset) *Asset {
  if a != nil {
    tk.Assets = append(tk.Assets, a)
  }
  return a
}


/*
  MatchAsset compares an asset with multiple matching operands,
  defined inside the Task. If any defined matching operands do
  not evaluate to true, the function returns false. If all
  defined matching operands are true, or if none are defined,
  this function returns true.
*/
func (tk *Task) MatchAsset (a *Asset) (bool, error) {
  if tk.MatchMimePrefix != "" && !strings.HasPrefix(a.Mimetype, tk.MatchMimePrefix) {
    return false, nil
  }

  if tk.MatchFunc != nil {
    if is_match, err := tk.MatchFunc(tk, a); err != nil {
      return false, err
    } else if is_match == false {
      return false, nil
    }
  }

  return true, nil
}


/*
  Match a TaskResolver using an asset, comparing it with the this
  resolver's task prototype, and those of this resolver's
  children. Find the deepest-ish matching TaskResolver.
*/
func (tr *TaskResolver) MatchWithAsset (a *Asset) (*TaskResolver, error) {
  // Check this resolver's TaskPrototype for a match.
  // Guard against a non-match without checking children.
  //
  if this_matches, err := tr.TaskPrototype.MatchAsset(a); err != nil {
    return nil, err
  } else if this_matches == false {
    return nil, nil
  }

  // This resolver, tr, matches.
  // Check children for matches, which take precedence
  //
  if tr.MatchBlocks == false {
    child_match, err := tr.MatchChildrenWithAsset(a)
    if err != nil {
      return nil, nil
    } else if child_match != nil {
      return child_match, err
    }
  }

  // No children match, but this resolver does.
  // Return this resolver.
  //
  return tr, nil
}


func (tr *TaskResolver) MatchChildrenWithAsset (a *Asset) (*TaskResolver, error) {
  for child := tr.Children; child != nil; child = child.Next {
    if child_match, err := child.MatchWithAsset(a); err != nil {
      return nil, err
    } else if child_match != nil {
      return child_match, nil
    }
  }
  return nil, nil
}
