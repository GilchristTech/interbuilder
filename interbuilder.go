package interbuilder

import (
  "fmt"
  "sync"
  "net/url"
  "strings"
  "time"
  "reflect"
  "runtime"
  "io"
)


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


func (s *Spec) Println (a ...any) (n int, err error) {
  if quiet, _, _ := s.InheritPropBool("quiet"); quiet {
    return 0, nil
  }

  return fmt.Println(a...)
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
      return fmt.Errorf(
        "[%s] Error: repeating (circular) task entry in task list: %s\n",
        s.Name, t.ResolverId,
      )
    }

    // Run the task
    //
    s.Printf("[%s] task: %s (%s)\n", s.Name, task.Name, task.ResolverId)
    if err := task.Run(s); err != nil {
      if task.ResolverId != "" {
        return fmt.Errorf(
          "[%s/%s] Error in task (%s): %w\n",
          s.Name, task.Name, task.ResolverId, err,
        )
      } else {
        return fmt.Errorf(
          "[%s/%s] Error in task: %w\n",
          s.Name, task.Name, err,
        )
      }
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


func specFormat (w io.Writer, s *Spec, level int) {
  var tab     string = "  "
  var align_0 string = strings.Repeat(tab, level)
  var align_1 string = align_0 + tab
  var align_2 string = align_1 + tab

  fmt.Fprint(w, align_0, s.Url, "\n")

  // Properties
  //
  if len(s.Props) > 0 {
    fmt.Sprint(w, align_1, "Properties:\n")
    for key, value := range s.Props {
      fmt.Fprintf(w, "%s%s  \t%T  \t%v\n", align_2, key, value, value)
    }
  }

  // Tasks
  //
  task_pointers := make(map[*Task]bool)
  heading_printed := false

  for task := s.Tasks ; task != nil ; task = task.Next {
    if heading_printed == false {
      fmt.Fprint(w, align_1, "Tasks:\n")
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
      fmt.Fprintf(w, "%s%s %s (%s)  WARNING: circular task list\n", align_2, bullet, task.Name, task.ResolverId)
      break
    }
    task_pointers[task] = true

    fmt.Fprintf(w, "%s%s %s (%s)\n", align_2, bullet, task.Name, task.ResolverId)
  }

  // Subspecs
  //
  if len(s.Subspecs) > 0 {
    fmt.Fprint(w, align_1, "Subspecs:\n")
    for _, subspec := range s.Subspecs {
      specFormat(w, subspec, level+2)
    }
  }
}


func SprintSpec (s *Spec) string {
  var builder strings.Builder
  specFormat(&builder, s, 0)
  return builder.String()
}


func PrintSpec (s *Spec) (n int, err error){
  var builder strings.Builder
  specFormat(&builder, s, 0)
  return fmt.Println(builder.String())
}
