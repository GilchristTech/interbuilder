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
  (SpecBuilders) of that structure to their children, with
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

  OutputSpecs     [] *Spec
  OutputChannels  [] *chan *Asset
  OutputGroups    [] *sync.WaitGroup

  // Input Asset handling
  //
  assets_input   []*Asset
  assets_chan    chan *Asset
  assets_lock    sync.Mutex
  assets_cond    *sync.Cond
  assets_done    bool
  InputGroup     sync.WaitGroup

  PathTransformations []*PathTransformation

  SpecBuilders    []SpecBuilder
  Props           SpecProps

  TaskResolvers   *TaskResolver

  Running bool

  Tasks              *Task
  CurrentTask        *Task
  tasks_enqueue_end  *Task
  tasks_push_queue   *Task
  tasks_push_end     *Task
  task_queue_lock    sync.Mutex

  // The AssetFrame to be built and outputted by this Spec
  AssetFrame              *AssetFrame
  asset_frame_lock        sync.Mutex

  // AssetFrame input fields

  asset_frames        map[string]*AssetFrame
  asset_frames_lock   sync.Mutex
  asset_frames_cond   *sync.Cond
  asset_frames_chan   chan *AssetFrame
  asset_frames_have   int
  asset_frames_expect int
}


type HistoryEntry struct {
  Url     *url.URL
  Parents []*HistoryEntry
  Time    time.Time
}


type SpecBuilder func (*Spec) error


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
    assets_chan:          make( chan *Asset,        1024),
    PathTransformations: make( []*PathTransformation, 0),
    SpecBuilders:        make( [] SpecBuilder,        0),
    Props:               make( SpecProps               ),
    asset_frames:        make( map[string]*AssetFrame  ),
    asset_frames_chan:   make( chan *AssetFrame        ),
  }

  spec.Root = &spec
  spec.asset_frames_cond = sync.NewCond(& spec.asset_frames_lock)
  spec.assets_cond = sync.NewCond(& spec.assets_lock)

  return &spec
}


func (s *Spec) MakeUrl (paths ...string) *url.URL {
  return s.Url.JoinPath(paths...)
}


func (s *Spec) Build () error {
  return s.BuildOther(s)
}


func (s *Spec) BuildOther (o *Spec) error {
  if s.Parent != nil {
    if err := s.Parent.BuildOther(o); err != nil {
      return err
    }
  }

  // Build from this spec's builder functions
  //
  for _, builder := range s.SpecBuilders {
    if err := builder(o); err != nil {
      builder_name := runtime.FuncForPC(reflect.ValueOf(builder).Pointer()).Name()

      if o == s {
        return fmt.Errorf(
          "Builder error in Spec %s in builder %s: %w",
          s.Name, builder_name, err,
        )
      }

      return fmt.Errorf(
        "Builder error in Spec %s (via builders in Spec %s) from builder %s: %w",
        o.Name, s.Name, builder_name, err,
      )
    }
  }

  return nil
}


func (s *Spec) AddSpecBuilder (b SpecBuilder) {
  s.SpecBuilders = append(s.SpecBuilders, b)
}


func (s *Spec) AddSubspec (a *Spec) *Spec {
  s.Subspecs[a.Name] = a
  a.Parent = s
  a.Root = s.Root
  a.AddOutputSpec(s)

  return a
}


func (sp *Spec) AddOutput (ch *chan *Asset, wg *sync.WaitGroup) {
  if ch != nil {
    sp.OutputChannels = append(sp.OutputChannels, ch)
  }

  if wg != nil {
    sp.OutputGroups = append(sp.OutputGroups, wg)
    wg.Add(1)
  }
}


func (sp *Spec) AddOutputSpec (out *Spec) {
  sp.AddOutput(&out.assets_chan, &out.InputGroup)
  sp.OutputSpecs = append(sp.OutputSpecs, out)
  out.asset_frame_lock.Lock()
  out.asset_frames_expect++
  out.asset_frame_lock.Unlock()
}


func (sp *Spec) Done () {
  for _, output_group := range sp.OutputGroups {
    output_group.Done()
  }

  sp.Running = false
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


func (sp *Spec) Run () error {
  // Only run the Spec if is not already running.
  //
  sp.task_queue_lock.Lock()
  if sp.Running {
    sp.task_queue_lock.Unlock()
    return fmt.Errorf("Spec with name \"%s\" is already running", sp.Name)
  }
  sp.Running = true
  sp.task_queue_lock.Unlock()

  sp.Printf("[%s] Running\n", sp.Name)
  defer sp.Printf("[%s] Exit\n", sp.Name)
  defer sp.Done()

  // Initialize Spec AssetFrame
  //
  sp.AssetFrame = & AssetFrame {
    Spec: sp,
    Assets: make(map[string]AssetFrameEntry),
  }

  var num_subspecs = len(sp.Subspecs)

  // Error and cancel channels. These are buffered with the
  // number of subspecs, because sending to an unbuffered channel
  // blocks, which can prevent the goroutines of subspecs from
  // exiting if they are trying to send an error.
  //
  var error_chan       = make(chan error, num_subspecs)
  var cancel_task_chan = make(chan bool,  num_subspecs)

  // AssetFrame input synchonization
  //
  if sp.asset_frames_expect > 0 {
    go sp.runAssetFrameBroadcast()
  }

  // Receive Assets from inputs
  //
  go func () {
    go sp.runAssetChanRecv()
  }()

  // Run subspecs in parallel goroutines
  //
  for _, subspec := range sp.Subspecs {
    go sp.runSubspec(subspec, error_chan, cancel_task_chan)
  }

  // When all subspecs have finished running, close this spec's
  // input channel.
  //
  go func () {
    sp.InputGroup.Wait()
    close(sp.assets_chan)
  }()

  //
  // Main task queue loop
  //
  sp.task_queue_lock.Lock()
  sp.flushTaskPushQueue()
  var task *Task = sp.Tasks
  sp.CurrentTask = task
  sp.task_queue_lock.Unlock()

  TASK_LOOP:
  for task != nil {
    select {
    default:
      // pass
    case <-cancel_task_chan:
      // TODO: instead of cancelling the task loop, perhaps this should skip to deferred tasks to allow cleanup tasks.
      break TASK_LOOP
    }

    // Check there's a valid task queue, going forward
    sp.task_queue_lock.Lock()

    if t := sp.Tasks.GetCircularTask(); t != nil {
      return fmt.Errorf(
        "Error in spec %s: repeating (circular) task entry in task list: %s",
        sp.Name, t.ResolverId,
      )
    }

    if task.Started {
      return fmt.Errorf("Tried to run task, but it was already started")
    }

    if (task.Func == nil) && (task.MapFunc == nil) {
      err := fmt.Errorf(
        "Error in spec %s: task \"%s\" doesn't have a Func or MapFunc defined",
        sp.Name, task.Name,
      )
      return err
    }

    // Run the Task Func
    //
    if task.ResolverId == "" {
      sp.Printf("[%s] task: %s\n", sp.Name, task.Name)
    } else {
      sp.Printf("[%s] task: %s (%s)\n", sp.Name, task.Name, task.ResolverId)
    }

    task.CancelChan = cancel_task_chan  // Pass by reference
    sp.task_queue_lock.Unlock()

    if err := task.Run(sp); err != nil {
      if task.ResolverId != "" {
        return fmt.Errorf(
          "Error in spec %s, in task %s (%s): %w\n",
          sp.Name, task.Name, task.ResolverId, err,
        )
      } else {
        return fmt.Errorf(
          "Error in spec %s, in task %s: %w\n",
          sp.Name, task.Name, err,
        )
      }
    }

    sp.task_queue_lock.Lock()
    task.CancelChan = nil
    task.Assets     = nil // Let un-emitted assets get freed

    // Flush the push queue and advance to the next task. Merge
    // the internal asset buffer into the next task.
    //
    // TODO: if a quit signal is sent, skip to the deferred portion of the task queue.
    //
    sp.flushTaskPushQueue()

    // If this is the last task, release the
    // Spec's AssetFrame to all outputs.
    //
    if sp.AssetFrame != nil && task.Next == nil {
      for _, out_spec := range sp.OutputSpecs {
        out_spec.asset_frames_chan <- sp.AssetFrame
      }
      sp.AssetFrame = nil
    }

    task           = task.Next
    sp.CurrentTask = task
    sp.task_queue_lock.Unlock()
  }

  // Consume remaining input assets and subspec errors:
  // there are no tasks left, but there may be input channel
  // assets remaining, or subspecs may have emitted errors. 
  //
  // The execution of this loop is implicitly tied to the
  // execution of subspecs; subspecs have their parents as output
  // WaitGroups, and their completion progresses the WaitGroup
  // this parent spec uses. When a subspec of this Spec finishes
  // (and calls Done()), this spec's WaitGroup is de-incremented.
  // Meanwhile, this Spec's Run method has spawned a goroutine to
  // close the input channel once the InputGroup WaitGroup is
  // Done, in turn causing the asset consumption in the loop
  // below to finish.

  CONSUME_INPUT_AND_ERRORS:
  for {
    select {
    case err := <-error_chan:
      sp.Println(err)
      return err

    case asset, ok := <-sp.assets_chan:
      if ok == false {
        // Subspecs may have finished executing, but they may
        // still be sending an error. Wait a tiny bit before
        // closing the error channel.
        //
        time.Sleep(5 * time.Millisecond)
        close(error_chan)
        break CONSUME_INPUT_AND_ERRORS
      }

      if err := sp.EmitAsset(asset); err != nil {
        return err
      }
    }
  }

  close(sp.asset_frames_chan)

  if err := <- error_chan; err != nil {
    return err
  }

  return nil
}


func (sp *Spec) runAssetFrameBroadcast () {
  for asset_frame := range sp.asset_frames_chan {
    sp.asset_frames_lock.Lock()
    sp.asset_frames[asset_frame.Spec.Name] = asset_frame
    sp.asset_frames_have++
    sp.asset_frames_cond.Broadcast()
    sp.asset_frames_lock.Unlock()
  }
}


func (sp *Spec) runAssetChanRecv () {
  for asset := range sp.assets_chan {
    sp.assets_cond.L.Lock()
    sp.assets_input = append(sp.assets_input, asset)
    sp.assets_cond.Broadcast()
    sp.assets_cond.L.Unlock()
  }

  // With the assets input channel closed, perform one more
  // broadcast to let all Asset awaits to get a nil Asset for
  // signaling closing.
  //
  sp.assets_cond.L.Lock()
  sp.assets_done = true
  sp.assets_cond.Broadcast()
  sp.assets_cond.L.Unlock()
}


func (sp *Spec) runSubspec(subspec *Spec, error_chan chan error, cancel_task_chan chan bool) error {
  if err := subspec.Run(); err != nil {
    var wrapped_error = fmt.Errorf(
        "Error in subspec \"%s\": %w", subspec.Name, err,
      )

    if error_chan != nil {
      error_chan <- wrapped_error
    }

    if cancel_task_chan != nil {
      cancel_task_chan <- true
    }

    return wrapped_error
  }
  return nil
}

func (sp *Spec) AwaitInputAssetNumber (number int) *Asset {
  sp.assets_cond.L.Lock()
  defer sp.assets_cond.L.Unlock()

  for number >= len(sp.assets_input) {
    if sp.assets_done {
      return nil
    }
    sp.assets_cond.Wait()
  }

  if number >= len(sp.assets_input) {
    return nil
  }

  return sp.assets_input[number]
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
    fmt.Fprintf(w, "%sProperties:\n", align_1)
    for key, value := range s.Props {
      fmt.Fprintf(w, "%s%s  \t%T  \t%v\n", align_2, key, value, value)
    }
  }

  if len(s.PathTransformations) > 0 {
    fmt.Fprintf(w, "%sPathTransformations:\n", align_1)
    for _, transformation := range s.PathTransformations {
      fmt.Fprintf(w, "%s%v\n", align_2, transformation)
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

    // Pick a bullet list to indicate the state of the Task
    //
    bullet := "-"

    if task.Errored {
      bullet = "!"
    } else if task.Started {
      bullet = ">"
    } else if task.MapFunc != nil {
      if task.num_assets_emitted > 0 {
        bullet = "|"
      } else {
        bullet = "~"
      }
    }

    // Check for task uniqueness and terminate circular task lists
    //
    _, found := task_pointers[task]
    if found {
      fmt.Fprintf(w, "%s%s %s (%s)  WARNING: circular task list\n", align_2, bullet, task.Name, task.ResolverId)
      break
    }
    task_pointers[task] = true

    fmt.Fprintf(w, "%s%s %s (%s)", align_2, bullet, task.Name, task.ResolverId)

    if num_assets := task.num_assets_emitted; num_assets > 0 {
      fmt.Fprintf(w, " [assets: %d]", num_assets)
    }

    if task.Mask != 0 {
      fmt.Fprintf(w, " [mask: %04O]", task.Mask)
    }

    fmt.Fprintln(w)
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
