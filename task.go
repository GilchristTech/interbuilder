package interbuilder

import (
  "fmt"
  "os/exec"
  "os"
  "strings"
)


/*
  The Task Mask constants are bitmasks which define how tasks
  work with Assets, whether they produce or consume, and what
  kind of modifications they make to Assets. When used, they
  allow for greater safety in Task queues, and can allow Tasks to
  be skipped when emitting Assets.
*/
const (
  TASK_FIELDS              uint64 = 0b_001_111_111_001  // All bits used by Task masks
  TASK_FIELDS_ASSETS       uint64 = 0b_000_111_111_000  // Bits in Tasks masks for Asset behaviors
  TASK_FIELDS_TASKS        uint64 = 0b_001_000_000_000  // Bits in Tasks masks for Task behaviors

  TASK_MASK_DEFINED        uint64 = 0b_000_000_000_001  // This bit distinguishes a Task
                                                        // mask with no permissions from
                                                        // one with undefined permissions,
                                                        // allowing masks with a zero
                                                        // value to act like a null value,
                                                        // rather than restrictive
                                                        // permission set.

  TASK_ASSETS_EMIT         uint64 = 0b_000_001_000_001  // Task emits new Assets
  TASK_ASSETS_GENERATE     uint64 = 0b_000_001_001_001  // Task creates new assets
  TASK_ASSETS_CONSUME      uint64 = TASK_ASSETS_FROM_ALL

  TASK_ASSETS_FROM_ALL     uint64 = 0b_000_110_000_001  // Task relies on Assets from anywhere
  TASK_ASSETS_FROM_SPECS   uint64 = 0b_000_010_000_001  // Task relies on Assets from input Specs
  TASK_ASSETS_FROM_TASKS   uint64 = 0b_000_100_000_001  // Task relies on Assets from previous Tasks

  TASK_ASSETS_FILTER       uint64 = 0b_000_001_010_001  // Task may not emit all Assets it consumes
  TASK_ASSETS_MUTATE       uint64 = 0b_000_001_100_001  // Task changes the content of existing assets

  TASK_ASSETS_FILTER_ALL   uint64 = TASK_ASSETS_FILTER | TASK_ASSETS_FROM_ALL
  TASK_ASSETS_FILTER_SPEC  uint64 = TASK_ASSETS_FILTER | TASK_ASSETS_FROM_SPECS
  TASK_ASSETS_FILTER_TASK  uint64 = TASK_ASSETS_FILTER | TASK_ASSETS_FROM_TASKS

  TASK_ASSETS_MUTATE_ALL   uint64 = TASK_ASSETS_MUTATE | TASK_ASSETS_FROM_ALL
  TASK_ASSETS_MUTATE_SPEC  uint64 = TASK_ASSETS_MUTATE | TASK_ASSETS_FROM_SPECS
  TASK_ASSETS_MUTATE_TASK  uint64 = TASK_ASSETS_MUTATE | TASK_ASSETS_FROM_TASKS

  TASK_TASKS_QUEUE         uint64 = 0b_001_000_000_001  // Task modifies the Task queue
) 


func TaskMaskContains (accept_mask, test_mask uint64) bool {
  if test_mask == 0 {
    return accept_mask == 0 || (accept_mask & TASK_FIELDS == TASK_FIELDS)
  }
  return (accept_mask == 0) || (accept_mask & test_mask == test_mask)
}


func TaskMaskValid (accept_mask, test_mask uint64) bool {
  if test_mask == 0 {
    return (accept_mask ^ TASK_FIELDS) & TASK_FIELDS == 0
  }
  return (accept_mask ^ test_mask) & test_mask == 0
}


type TaskFunc      func (*Spec, *Task) error
type TaskMapFunc   func (*Asset) (*Asset, error)
type TaskMatchFunc func (name string, spec *Spec) (bool, error)


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
  Errored    bool
  Next       *Task
  History    HistoryEntry

  // The Task Mask optionally specifies whether this task emits
  // or consumes assets, and other more specific safety
  // constraints.
  //
  Mask uint64

  Assets      []*Asset

  num_assets_received  int
  num_assets_emitted   int
  num_assets_generated int

  spec_asset_number int

  // Func task callback functions only run when this task is
  // reached in the Task queue. It can access its internal Assets
  // array in a current, complete state, and can modify the
  // future elements of the Task queue in a thread-safe way. Func
  // Task callbacks are the serial execution mechanism of a
  // Spec's Task queue.
  //
  Func TaskFunc
  
  // MapFunc task callback functions are ran over every Asset
  // emitted to this Task, and can be executed as part of the
  // emitting algorithm before a task is reached within the Task
  // queue.
  //
  MapFunc TaskMapFunc

  // TODO: deprecate, replace with methods
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
    either a single Asset or acting like a promise to expand into
    more Assets. Tasks have flags to specify how they handle
    these differing Asset quantities.
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

  // IgnoreAssets indicates that this Task does not read or
  // modify assets (this does not preclude the Task creating
  // them). This allows the Asset emitting to skip this Task. An
  // example of where this is useful, is in a Task which closes
  // an IO resource, because the Asset can be emitted, and
  // possibly freed from memory without requiring this task to be
  // executed.
  // TODO: deprecate, as this feature is redundant with the Task.Mask consume flag
  //
  IgnoreAssets bool
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
  tk.Errored = false
  if err := tk.Func(s, tk); err != nil {
    tk.Errored = true
    return err
  }
  return nil
}


/*
  Insert a task into the task queue, before deferred tasks.
  Enqueued tasks are executed in first-in, first-out order, like
  a queue.

  This method can execute during Spec execution, and does not
  lock the task queue.
*/
func (sp *Spec) enqueueTaskUnsafe (tk *Task) error {
  tk.Spec = sp

  // Find the end of the added tasks while
  // updating their Spec values.
  //
  var end = tk
  for next := tk.Next; next != nil; end, next = next, next.Next {
    if next.Spec == sp {
      continue
    } else if next.Spec != nil {
      return fmt.Errorf(
        "Cannot add this Task to Spec with name '%s', it already has a Spec defined with name '%s'",
        sp.Name, next.Spec.Name,
      )
    } else {
      next.Spec = sp
    }
  }

  // If the Task queue is uninitialized, use this Task as the
  // start of the task queue.
  //
  if sp.Tasks == nil {
    sp.Tasks = tk
    sp.tasks_enqueue_end = tk
    return nil
  }

  // If the end of the Task enqueue is undefined, initialize it.
  //
  if sp.tasks_enqueue_end == nil {
    end.Next = sp.Tasks
    sp.Tasks = tk
    sp.tasks_enqueue_end = end
    return nil
  }

  sp.tasks_enqueue_end = sp.tasks_enqueue_end.insertRange(tk, end)
  return nil
}


/*
  Insert a task into the task queue, before deferred tasks.
  Enqueued tasks are executed in first-in, first-out order, like
  a queue.

  This method is meant to construct a Task queue prior to running
  the Spec. Because of this, it returns an error if the Spec is
  running. In order to modify the Task queue during execution,
  use Task.EnqueueTask.
*/
func (sp *Spec) EnqueueTask (tk *Task) error {
  sp.task_queue_lock.Lock()
  defer sp.task_queue_lock.Unlock()

  if sp.Running {
    return fmt.Errorf("Spec \"%s\" cannot enqueue tasks while it is running", sp.Name)
  }

  return sp.enqueueTaskUnsafe(tk)
}


/*
  EnqueueTaskFunc creates a new Task with the specified name and
  function (`fn`), enqueues it for execution in the task queue.
*/
func (sp *Spec) EnqueueTaskFunc (name string, fn TaskFunc) error {
  return sp.EnqueueTask(& Task {
    Name: name,
    Func: fn,
  })
}


/*
  EnqueueTaskMapFunc creates a new Task with the specified name
  and asset map function (`fn`), enqueues it for execution in the
  task queue.
*/
func (sp *Spec) EnqueueTaskMapFunc (name string, fn TaskMapFunc) error {
  return sp.EnqueueTask(& Task {
    Name: name,
    MapFunc: fn,
  })
}


/*
  Insert a task into the task queue, directly after the end of
  the enqueue end point. These tasks are executed in first-in,
  last-out order relative to other tasks in the queue, but if
  multiple tasks are inserted their order is maintained.

  This method can execute during Spec execution, and does not
  lock the task queue.
*/
func (sp *Spec) deferTaskUnsafe (tk *Task) error {
  tk.Spec = sp

  // Find the end of the added tasks while
  // updating their Spec values.
  //
  var end = tk
  for next := tk.Next; next != nil; end, next = next, next.Next {
    if next.Spec == sp {
      continue
    } else if next.Spec != nil {
      return fmt.Errorf("Cannot add this Task to Spec with name \"%s\", it already has a Spec defined with name \"%s\"", sp.Name, next.Spec.Name)
    } else {
      next.Spec = sp
    }
  }

  // If the spec tasks list is not yet defined, enqueued tasks
  // should still be executed before deferred tasks, so define
  // the tasks list, but do not define an end to the enqueue
  // point. This will cause enqueuing to insert into the top of
  // the task list.
  //
  if sp.Tasks == nil {
    sp.Tasks = tk
    sp.tasks_enqueue_end = nil
    return nil
  }

  if sp.tasks_enqueue_end == nil {
    sp.tasks_enqueue_end = sp.Tasks.End()
  }

  sp.tasks_enqueue_end.insertRange(tk, end)
  return nil
}


/*
  Insert a task into the task queue, directly after the end of
  the enqueue end point. These tasks are executed in first-in,
  last-out order relative to other tasks in the queue, but if
  multiple tasks are inserted their order is maintained.

  This method is meant to construct a Task queue prior to running
  the Spec. Because of this, it returns an error if the Spec is
  running. In order to modify the Task queue during execution,
  use Task.DeferTask.
*/
func (sp *Spec) DeferTask (tk *Task) error {
  sp.task_queue_lock.Lock()
  defer sp.task_queue_lock.Unlock()

  if sp.Running {
    return fmt.Errorf("Spec \"%s\" cannot defer tasks while it is running", sp.Name)
  }

  return sp.deferTaskUnsafe(tk)
}


/*
  DeferTaskFunc creates a new Task with the specified name and
  function (`fn`), defers it for execution in the task queue.
*/
func (sp *Spec) DeferTaskFunc (name string, fn TaskFunc) error {
  return sp.DeferTask(& Task { Name: name, Func: fn })
}


/*
  DeferTaskMapFunc creates a new Task with the specified name
  and asset map function (`fn`), defers it for execution in the
  task queue, and returns it.
*/
func (sp *Spec) DeferTaskMapFunc (name string, fn TaskMapFunc) error {
  return sp.DeferTask(& Task { Name: name, MapFunc: fn })
}


/*
  PushTask adds a Task to the push queue. The push queue is a
  temporary holding area for tasks that need to be executed
  immediately before other tasks in the main queue. When the task
  execution loop begins, and after each tasks, all tasks in the
  push queue are flushed into the main task queue to be executed
  next.

  This method can execute during Spec execution, and does not
  lock the task queue.
*/
func (sp *Spec) pushTaskUnsafe (tk *Task) error {
  tk.Spec = sp

  // Find the end of the added tasks while
  // updating their Spec values.
  //
  var end = tk
  for next := tk.Next; next != nil; end, next = next, next.Next {
    if next.Spec == sp {
      continue
    } else if next.Spec != nil {
      return fmt.Errorf("Cannot add this Task to Spec with name \"%s\", it already has a Spec defined with name \"%s\"", sp.Name, next.Spec.Name)
    } else {
      next.Spec = sp
    }
  }

  if sp.tasks_push_queue == nil || sp.tasks_push_end == nil {
    sp.tasks_push_queue = tk
    sp.tasks_push_end   = end
    return nil
  }

  sp.tasks_push_end = sp.tasks_push_end.insertRange(tk, end)
  return nil
}


/*
  PushTask adds a Task to the push queue. The push queue is a
  temporary holding area for tasks that need to be executed
  immediately before other tasks in the main queue. When the task
  execution loop begins, and after each tasks, all tasks in the
  push queue are flushed into the main task queue to be executed
  next.

  This method is meant to construct a Task queue prior to running
  the Spec. Because of this, it returns an error if the Spec is
  running. In order to modify the Task queue during execution,
  use Task.PushTask.
*/
func (sp *Spec) PushTask (tk *Task) error {
  sp.task_queue_lock.Lock()
  defer sp.task_queue_lock.Unlock()

  if sp.Running {
    return fmt.Errorf("Spec \"%s\" cannot push tasks while it is running", sp.Name)
  }

  return sp.pushTaskUnsafe(tk)
}


/*
  PushTaskFunc creates a new Task with the specified name and
  function (`f`), pushs it for execution in the task queue
*/
func (sp *Spec) PushTaskFunc (name string, fn TaskFunc) error {
  return sp.PushTask(& Task { Name: name, Func: fn })
}


/*
  EnqueueTaskName retrieves a Task by its name and enqueues it
  for execution. If the task is found, it is added to the task
  queue and the last inserted Task is returned.  If the task
  cannot be found, it is returned as nil. If an error occurs, an
  error is returned.
*/
func (sp *Spec) EnqueueTaskName (name string) (*Task, error) {
  task, err := sp.GetTask(name, sp)
  if task == nil || err != nil {
    return nil, err
  }
  return task, sp.EnqueueTask(task)
}

 
/*
  EnqueueUniqueTask enqueues a Task only if there isn't already a
  task with the same name in the task queue. If a task with the
  same name already exists, it returns the existing task without
  modifying the task queue. Otherwise, it enqueues the provided
  task and returns the final enqueued Task.
*/
func (sp *Spec) EnqueueUniqueTask (tk *Task) (*Task, error) {
  if tk.Name == "" {
    return nil, fmt.Errorf("EnqueueUniqueTask error: task's name is empty")
  }

  existing_task := sp.GetTaskFromQueue(tk.Name)
  if existing_task != nil {
    return existing_task, nil
  }

  return tk, sp.EnqueueTask(tk)
}


/*
  EnqueueUniqueTaskName enqueues a Task by its name only if there isn't
  already a task with the same name in the task queue. If a task with
  the same name already exists, it returns the existing task without
  enqueuing a new one.
*/
func (sp *Spec) EnqueueUniqueTaskName (name string) (*Task, error) {
  existing_task := sp.GetTaskFromQueue(name)
  if existing_task != nil {
    return existing_task, nil
  }
  return sp.EnqueueTaskName(name)
}


/*
  GetTaskFromQueue searches the task queue for a task with
  the specified name and returns it. If no such task is found, it
  returns nil.
*/
func (sp *Spec) GetTaskFromQueue (name string) *Task {
  // TODO: check the push queue for matching tasks
  for task := sp.Tasks ; task != nil ; task = task.Next {
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
  EmitAsset sends an Asset to subsequent assets. As long
  as there are Tasks with only MapFuncs, the asset will have
  those map function applied, and Tasks with normal Funcs will
  have the asset deposited into their asset array and block the
  passing until that Task is reached in the Spec's Task queue. If
  an Asset makes it all the way through the Task queue, it is
  emitted.
*/
func (tk *Task) EmitAsset (asset *Asset) error {
  var spec = tk.Spec


  // If the Task mask is defined but not set to emit, error. An undefined
  // (zero) mask is okay.
  //
  if TaskMaskContains(tk.Mask, TASK_ASSETS_EMIT) == false {
    return fmt.Errorf(
      "Task cannot emit asset, Task.Mask has a value of %04O",
      tk.Mask,
    )
  }

  // Update the AssetFrame with this Asset's URL path
  spec.asset_frame_lock.Lock()
  if spec.AssetFrame.HasKey(asset.Url.Path) == false {
    if TaskMaskContains(tk.Mask, TASK_ASSETS_GENERATE) == false {
      return fmt.Errorf(
        "Task cannot emit asset with new URL, Task.Mask states it cannot generate Assets (mask: %O)",
        tk.Mask,
      )
    }

    if err := spec.AssetFrame.AddKey(asset.Url.Path); err != nil {
      return err
    }

    tk.num_assets_generated++
  }
  spec.asset_frame_lock.Unlock()

  if asset.Spec == nil {
    asset.Spec = tk.Spec
  }
  var err error

  var next *Task = tk.Next

  // Don't emit to Tasks which cannot consume Assets due to their
  // Mask. Search the task chain for a Task which can accept
  // assets, or leave the `next` variable nil from trying.
  //
  for next = tk.Next; next != nil; next = next.Next {
    if (!next.IgnoreAssets                               &&(
        TaskMaskContains(next.Mask, TASK_TASKS_QUEUE)     ||
        TaskMaskContains(next.Mask, TASK_ASSETS_CONSUME)  ||
        TaskMaskContains(next.Mask, TASK_ASSETS_FILTER)   ||
        TaskMaskContains(next.Mask, TASK_ASSETS_MUTATE)  )){
      break
    }
  }

  // If this is the final task, the only place left for the asset
  // to go is being emitted by the Spec. Do so if it exists.
  //
  if next == nil {
    if tk.Spec != nil {
      if err := tk.Spec.EmitAsset(asset); err != nil {
        return fmt.Errorf("Error in task %s emitting asset: %w", tk.Name, err)
      }
    }
    tk.num_assets_emitted++
    return nil
  }

  // Handle pluralistic assets
  //
  if asset.IsMulti() {

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
      if assets, err := asset.Flatten(); err != nil {
        return err
      } else {
        for _, flattened_asset := range assets {
          if err := tk.EmitAsset(flattened_asset); err != nil {
            return err
          }
          tk.num_assets_emitted++
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
    tk.num_assets_emitted++
    return next.EmitAsset(asset)
  }

  // This asset matches in the next task.

  // If the next task has no MapFunc, deposit it into that task's
  // Asset buffer and exit.
  //
  if next.MapFunc == nil {
    tk.num_assets_emitted++
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
  if asset == nil {
    // Assert that the next task has the permission to filter
    // assets. If so, filter this Asset by returning early, or
    // return a permission error.
    //
    if asset.Spec == next.Spec {
      if TaskMaskContains(tk.Mask, TASK_ASSETS_FILTER_TASK) == false {
        return fmt.Errorf(
          "Task %s cannot filter assets from tasks, but its MapFunc returned nil (mask: %04O)",
          next.Name, next.Mask,
        )
      }
    } else {
      if TaskMaskContains(tk.Mask, TASK_ASSETS_FILTER_SPEC) == false {
        return fmt.Errorf(
          "Task %s cannot filter assets from specs, but its MapFunc returned nil (mask: %04O)",
          next.Name, next.Mask,
        )
      }
    }
    return nil
  }

  // With the new asset, if the next task has a Func, then it is
  // the destination, since the Func may mutate the asset via its
  // task buffer.
  //
  if next.Func != nil {
    next.AddAsset(asset)
    tk.num_assets_emitted++
    return nil
  }

  // There is a next task, we have a valid (map-function-applied)
  // asset for it, and this task has no Func to mutate the asset
  // further. Recurse, sending the asset as far as it can go in
  // the Task without requiring the task queue to synchronize up
  // until that point.
  //
  tk.num_assets_emitted++
  return next.EmitAsset(asset)
}


func (tk *Task) AwaitInputAssetNumber (number int) (*Asset, error) {
  if err := tk.AssertSpec(); err != nil {
    return nil, fmt.Errorf("Task %s cannot await Asset input: %w", tk.Name, err)
  }

  if TaskMaskContains(tk.Mask, TASK_ASSETS_FROM_SPECS) == false {
    return nil, fmt.Errorf(
      "Task %s cannot await Asset input: Task.Mask forbids receiving assets from specs (%04O)", tk.Name, tk.Mask,
    )
  }

  return tk.Spec.AwaitInputAssetNumber(number), nil
}


func (tk *Task) AwaitInputAssetNext () (*Asset, error) {
  if asset, err := tk.AwaitInputAssetNumber(tk.spec_asset_number); err != nil {
    return nil, err

  } else if asset != nil {
    tk.spec_asset_number++
    return asset, nil
  }

  return nil, nil
}


/*
  PoolSpecInputAssets reads the Spec input channel for asset
  chunks and inserts them into the Task's Asset array.
*/
func (tk *Task) PoolSpecInputAssets () error {
  // If the Task mask is defined but not set to emit, error. An undefined
  // (zero) mask is okay.
  //
  if TaskMaskContains(tk.Mask, TASK_ASSETS_CONSUME) == false {
    return fmt.Errorf("Task cannot pool assets, Task.Mask has a value of %04O, and is not set to consume assets", tk.Mask)
  }

  if tk.Spec == nil {
    return fmt.Errorf("Task Spec is nil")
  }

  for {
    asset_chunk, err := tk.AwaitInputAssetNext()
    if err != nil {
      return err
    }

    if asset_chunk == nil {
      break
    }

    if asset_chunk.IsSingle() || tk.AcceptMultiAssets {
      tk.Assets = append(tk.Assets, asset_chunk)
      continue
    }

    // This is a multi-asset, and this task does not accept
    // multi-assets.

    if ! tk.RejectFlattenMultiAssets {
      if assets, err := asset_chunk.Flatten(); err != nil {
        return fmt.Errorf(
          `Cannot pool assets, asset chunk with URL "%s" returned an error while flattening: %w"`,
          asset_chunk.Url, err,
        )
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
  // TODO: the first task after this which receives assets is not necessarily tk.Next. This should read ahead for valid tasks, enabling more shortcutting of Assets.
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
  // TODO: add error and check Asset permissions
  if a != nil {
    tk.num_assets_received++
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
  AssertSpec returns an error if this task's Spec is nil.
*/
func (tk *Task) AssertSpec () error {
  if tk.Spec != nil {
    return nil
  }

  return fmt.Errorf("Spec is nil")
}


/*
  AssertTaskQueuing returns an error if this task is unable to
  modify the task queue, either due to an undefined Spec or
  because of Task.Mask permission issues.
*/
func (tk *Task) AssertTaskQueuing () error {
  if err := tk.AssertSpec(); err != nil {
    return fmt.Errorf("Task with name '%s' cannot modify the task queue: %w", tk.Name, err)
  }

  var spec = tk.Spec

  if TaskMaskContains(tk.Mask, TASK_TASKS_QUEUE) == false {
    return fmt.Errorf(
      "Task with name '%s' in spec '%s' cannot modify task queue, Task.Mask has a value of %O",
      tk.Name, spec.Name, tk.Mask,
    )
  }

  return nil
}


/*
  AssertTaskIsQueueable returns an error if this task would have a
  permission issue from queuing another task. It does *not*
  assert that this task can queue tasks; for that, use
  AssertTaskQueuing.
*/
func (tk *Task) AssertTaskIsQueueable (task *Task) error {
  var accept_resolver_name = "undefined"
  var test_resolver_name   = "undefined"

  var accept_mask = tk.Mask
  if tk.Resolver != nil {
    accept_mask |= tk.Resolver.AcceptMask
    accept_resolver_name = tk.Resolver.Name
  }

  if accept_mask == 0 {
    return nil
  }

  var test_mask = task.Mask
  if task.Resolver != nil {
    test_mask |= task.Resolver.AcceptMask
    test_resolver_name = task.Resolver.Name
  }

  if TaskMaskValid(accept_mask, test_mask) == false {
    return fmt.Errorf(
      "Task '%s (%s)' cannot add a Task '%s (%s)', added Task's Mask (%04O) is not a subset of (%04O)",
      tk.Name, accept_resolver_name, task.Name, test_resolver_name, test_mask, accept_mask,
    )
  }

  return nil
}


func (tk *Task) DeferTask (task *Task) error {
  if err := tk.AssertTaskQueuing(); err != nil {
    return err
  }
  var spec = tk.Spec
  spec.task_queue_lock.Lock()
  defer spec.task_queue_lock.Unlock()
  return spec.deferTaskUnsafe(task)
}


func (tk *Task) DeferTaskFunc (name string, fn TaskFunc) error {
  return tk.DeferTask(& Task {
    Name: name,
    Func: fn,
  })
}


func (tk *Task) DeferTaskMapFunc (name string, fn TaskMapFunc) error {
  return tk.DeferTask(& Task {
    Name: name,
    MapFunc: fn,
  })
}


func (tk *Task) EnqueueTask (task *Task) error {
  if err := tk.AssertTaskQueuing(); err != nil {
    return err
  }
  var spec = tk.Spec
  spec.task_queue_lock.Lock()
  defer spec.task_queue_lock.Unlock()

  if err := tk.AssertTaskIsQueueable(task); err != nil {
    return err
  }

  return spec.enqueueTaskUnsafe(task)
}


func (tk *Task) EnqueueTaskFunc (name string, fn TaskFunc) error {
  var task = & Task {
    Name: name,
    Func: fn,
  }

  return tk.EnqueueTask(task)
}


func (tk *Task) EnqueueTaskMapFunc (name string, fn TaskMapFunc) error {
  if name == "" {
    return fmt.Errorf("EnqueueUniqueTask error: task's name is empty")
  }

  var task = & Task {
    Name: name,
    MapFunc: fn,
  }

  return tk.EnqueueTask(task)
}


func (tk *Task) EnqueueTaskName (name string) (*Task, error) {
  if tk.Spec == nil {
    return nil, fmt.Errorf("Task with name '%s' cannot modify the task queue, Spec is nil", tk.Name)
  }

  task, err := tk.Spec.GetTask(name, tk.Spec)
  if task == nil || err != nil {
    return nil, err
  }
  return task, tk.EnqueueTask(task)
}


func (tk *Task) EnqueueUniqueTask (task *Task) (*Task, error) {
  if tk.Spec == nil {
    return nil, fmt.Errorf("Task with name '%s' cannot modify the task queue, Spec is nil", tk.Name)
  }

  if task.Name == "" {
    return nil, fmt.Errorf("EnqueueUniqueTask error: task's name is empty")
  }

  existing_task := tk.Spec.GetTaskFromQueue(task.Name)
  if existing_task != nil {
    return existing_task, nil
  }

  return task, tk.EnqueueTask(tk)
}


func (tk *Task) EnqueueUniqueTaskName (name string) (*Task, error) {
  if tk.Spec == nil {
    return nil, fmt.Errorf("Task with name '%s' cannot modify the task queue, Spec is nil", tk.Name)
  }

  existing_task := tk.Spec.GetTaskFromQueue(name)
  if existing_task != nil {
    return existing_task, nil
  }
  return tk.EnqueueTaskName(name)
}



func (tk *Task) PushTask (task *Task) error {
  if err := tk.AssertTaskQueuing(); err != nil {
    return err
  }
  var spec = tk.Spec
  spec.task_queue_lock.Lock()
  defer spec.task_queue_lock.Unlock()
  return spec.pushTaskUnsafe(task)
}


func (tk *Task) PushTaskFunc (name string, fn TaskFunc) error {
  return tk.PushTask(& Task {
    Name: name,
    Func: fn,
  })
}
