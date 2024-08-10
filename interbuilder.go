package interbuilder

import (
  "log"
  "fmt"
  "sync"
  "net/url"
  "strings"
  "time"
  "reflect"
  "runtime"
)


type SpecProps map[string]any


/*
  A Spec represents a node in a tree of concurrent user-defined
  operations which pass their output to their parents. They can
  be built using a JSON-like structure, and pass parsing rules
  (SpecResolvers) of that structure to their children, with
  parental rules being executed before child rules. Also, they
  have a namespacing and metadata-matching system (TaskResolvers)
  for determining which tasks to run, with child task resolvers
  taking precedence over those in the Spec parental chain.
*/
type Spec struct {
  Name      string
  Url       *url.URL
  History   HistoryEntry

  Parent    *Spec
  Root      *Spec
  Subspecs  map[string]*Spec

  OutputChannels  [] *chan *Asset
  OutputGroups    [] *sync.WaitGroup

  Input           chan *Asset
  InputGroup      sync.WaitGroup

  PathTransformations []*PathTransformation

  SpecResolvers   []SpecResolver
  Props           SpecProps

  TaskResolvers   *TaskResolver

  Tasks              *Task
  CurrentTask        *Task
  tasks_enqueue_end  *Task
  tasks_push_queue   *Task
  tasks_push_end     *Task
  task_queue_lock    sync.Mutex
}


type HistoryEntry struct {
  Url     *url.URL
  Parents []*HistoryEntry
  Time    time.Time
}


type SpecResolver func (*Spec) error

func NewSpec (name string, spec_url *url.URL) *Spec {
  if spec_url == nil {
    spec_url = &url.URL { Scheme: "ib", Host: name }
  }

  spec := Spec {
    Name:                name,
    Url:                 spec_url,
    History:             HistoryEntry { Url: spec_url  },
    Subspecs:            make( map[string]*Spec        ),
    OutputChannels:      make( [] *chan *Asset,       0),
    OutputGroups:        make( [] *sync.WaitGroup,    0),
    Input:               make( chan *Asset             ),
    PathTransformations: make( []*PathTransformation, 0),
    SpecResolvers:       make( [] SpecResolver,       0),
    Props:               make( SpecProps               ),
  }

  spec.Root = &spec

  return &spec
}


func (s *Spec) MakeUrl (paths ...string) *url.URL {
  return s.Url.JoinPath(paths...)
}


func (s *Spec) Resolve () error {
  return s.ResolveOther(s)
}


func (s *Spec) ResolveOther (o *Spec) error {
  if s.Parent != nil {
    if err := s.Parent.ResolveOther(o); err != nil {
      return err
    }
  }

  for _, resolver := range s.SpecResolvers {
    // Resolve this spec's 
    if err := resolver(o); err != nil {
      resolver_name := runtime.FuncForPC(reflect.ValueOf(resolver).Pointer()).Name()

      if o == s {
        return fmt.Errorf(
          "Resolver error in Spec %s in resolver %s: %w",
          s.Name, resolver_name, err,
        )
      }

      return fmt.Errorf(
        "Resolver error in Spec %s (resolving via Spec %s) in resolver %s: %w",
        o.Name, s.Name, resolver_name, err,
      )
    }
  }

  return nil
}


func (s *Spec) InheritProp (key string) (val any, found bool) {
  if val, found = s.Props[key] ; found {
    return val, found
  }

  if s.Parent == nil {
    return nil, false
  }

  return s.Parent.InheritProp(key)
}


func (s *Spec) InheritPropString (key string) (value string, ok, found bool) {
  value_any, found := s.InheritProp(key)
  value,     ok     = value_any.(string)
  return value, ok, found
}


func (s *Spec) GetProp (key string) (value any, found bool) {
  if value, found := s.Props[key] ; found {
    return value, found
  }
  return nil, false
}


func (s *Spec) GetPropBool (key string) (value bool, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(bool)
  return value, ok, found
}


func (s *Spec) InheritPropBool (key string) (value bool, ok, found bool) {
  value_any, found := s.InheritProp(key)
  value,     ok     = value_any.(bool)
  return value, ok, found
}


func (s *Spec) GetPropString (key string) (value string, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(string)
  return value, ok, found
}


func (s *Spec) GetPropUrl (key string) (value *url.URL, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(*url.URL)
  return value, ok, found
}


func (s *Spec) RequireProp (key string) (value any, err error) {
  value, found := s.Props[key]

  if !found {
    return nil, fmt.Errorf(
      "Spec %s requires spec prop %s to exist",
      s.Name, key,
    )
  }

  return value, nil
}


func (s *Spec) RequirePropString (key string) (string, error) {
  value_any, err := s.RequireProp(key)
  if err != nil {
    return "", err
  }

  value, ok := value_any.(string)
  if !ok {
    return "", fmt.Errorf(
      "Spec %s requires spec prop %s to be a string, got %T",
      s.Name, key, s.Props[key],
    )
  }

  return value, nil
}


func (s *Spec) GetPropJson (key string) (value map[string]any, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(map[string]any)
  return value, ok, found
}


func (s *Spec) AddSpecResolver (r SpecResolver) {
  s.SpecResolvers = append(s.SpecResolvers, r)
}


func (s *Spec) AddSubspec (a *Spec) *Spec {
  s.Subspecs[a.Name] = a
  a.Parent = s
  a.Root = s.Root
  a.AddOutputSpec(s)

  return a
}


func (s *Spec) AddOutput (ch *chan *Asset, wg *sync.WaitGroup) {
  if ch != nil {
    s.OutputChannels = append(s.OutputChannels, ch)
  }

  if wg != nil {
    s.OutputGroups = append(s.OutputGroups, wg)
    wg.Add(1)
  }
}


func (s *Spec) AddOutputSpec (o *Spec) {
  s.AddOutput(&o.Input, &o.InputGroup)
}


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


func (s *Spec) EnqueueTaskFunc (name string, f TaskFunc) *Task {
  var task = Task {
    Spec: s,
    Name: name,
    Func: f,
  }

  return s.EnqueueTask(&task)
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
  Push a Task into the push queue. In the task execution loop,
  before items are executed, tasks in the push queue are flushed
  and inserted into the main task queue to be executed before
  other tasks. In this sense, the main task queue executes like a
  stack rather than a queue. Return the final inserted item.
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


func (s *Spec) EnqueueTaskName (name string) (*Task, error) {
  task, err := s.GetTask(name, s)
  if task == nil || err != nil {
    return nil, err
  }
  return s.EnqueueTask(task), nil
}


func (s *Spec) EnqueueUniqueTask (t *Task) (*Task, error) {
  if t.Name == "" {
    return nil, fmt.Errorf("EnqueueUniqueTask error: task's name is empty")
  }

  existing_task := s.GetTaskFromQueue(t.Name)
  if existing_task != nil {
    return existing_task, nil
  }

  return s.EnqueueTask(t), nil
}


func (s *Spec) EnqueueUniqueTaskName (name string) (*Task, error) {
  existing_task := s.GetTaskFromQueue(name)
  if existing_task != nil {
    return existing_task, nil
  }
  return s.EnqueueTaskName(name)
}


func (s *Spec) GetTaskFromQueue (name string) *Task {
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


func (s *Spec) Done () {
  for _, output_group := range s.OutputGroups {
    output_group.Done()
  }
}


func (s *Spec) Printf (format string, a ...any) (n int, err error) {
  if quiet, _, _ := s.InheritPropBool("quiet"); quiet {
    return 0, nil
  }

  return fmt.Printf(format, a...)
}


func (s *Spec) Run () error {
  // TODO: print message verbosity settings; these should not print during tests
  s.Printf("[%s] Running\n", s.Name)
  defer s.Printf("[%s] Exit\n", s.Name)
  defer s.Done()

  //
  // Run subspecs in parallel goroutines
  //
  for _, subspec := range s.Subspecs {
    go func () {
      // TODO: give subspecs a quit signal
      err := subspec.Run()
      if err != nil {
        // TODO: terminate this spec (this, being that of s.Run)
      }
    }()
  }

  // When all subspecs have finished running, close this spec's
  // input channel.
  //
  go func () {
    s.InputGroup.Wait()
    close(s.Input)
  }()

  //
  // Main task queue loop
  //
  s.task_queue_lock.Lock()
  s.flushTaskPushQueue()
  var task *Task = s.Tasks
  s.CurrentTask = task
  s.task_queue_lock.Unlock()

  for task != nil {
    // Assert a valid task queue, going forward
    //
    if t := s.Tasks.GetCircularTask(); t != nil {
      log.Fatalf(
        "[%s] Error: repeating (circular) task entry in task list: %s\n",
        s.Name, t.ResolverId,
      )
    }

    // Run the task
    //
    s.Printf("[%s] task: %s (%s)\n", s.Name, task.Name, task.ResolverId)
    if err := task.Run(s); err != nil {
      s.Printf(
        "[%s/%s] Error in task %s (%s):\n%s\n",
        s.Name, task.Name,
        task.ResolverId,
        s.Name, err,
      )
      return err
    }

    // Flush the push queue and advance to the next task
    // TODO: if a quit signal is sent, skip to the deferred portion of the task queue.
    //
    s.task_queue_lock.Lock()
    s.flushTaskPushQueue()
    task          = task.Next
    s.CurrentTask = task
    s.task_queue_lock.Unlock()
  }

  // Emit any remaining assets
  //
  for asset := range s.Input {
    err := s.EmitAsset(asset)
    if err != nil { return err }
  }
  
  // For the above range to finish, s.Input must be closed. This
  // function runs a goroutine which waits for the subspecs to
  // finish executing before closing the input channel, which
  // means that for execution to get to this point,
  // s.InputGroup.Wait() has been called and finished.

  return nil
}


func PrintSpec (s *Spec, level int) {
  tab     := "  "
  align_0 := strings.Repeat(tab, level)
  align_1 := align_0 + tab
  align_2 := align_1 + tab

  fmt.Print(align_0, s.Url, "\n")

  // Properties
  //
  if len(s.Props) > 0 {
    fmt.Print(align_1, "Properties:\n")
    for key, value := range s.Props {
      fmt.Printf("%s%s  \t%T  \t%s\n", align_2, key, value, value)
    }
  }

  // Tasks
  //
  task_pointers := make(map[*Task]bool)
  heading_printed := false

  for task := s.Tasks ; task != nil ; task = task.Next {
    if heading_printed == false {
      fmt.Print(align_1, "Tasks:\n")
      heading_printed = true
    }
    bullet := "-"
    if task.Started {
      bullet = ">"
    }

    // Check for task uniqueness and terminate circular task lists
    //
    _, found := task_pointers[task]
    if found {
      fmt.Printf("%s%s %s (%s)  WARNING: circular task list\n", align_2, bullet, task.Name, task.ResolverId)
      break
    }
    task_pointers[task] = true

    fmt.Printf("%s%s %s (%s)\n", align_2, bullet, task.Name, task.ResolverId)
  }

  // Subspecs
  //
  if len(s.Subspecs) > 0 {
    fmt.Print(align_1, "Subspecs:\n")
    for _, subspec := range s.Subspecs {
      PrintSpec(subspec, level+2)
    }
  }
}
