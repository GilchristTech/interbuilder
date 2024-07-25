package interbuilder

import (
  "testing"
)


func TestSpecEnqueueTaskIsNotCircular (t *testing.T) {
  spec := NewSpec("test", nil)

  spec.EnqueueTask( & Task { Name: "Task1" } )
  spec.EnqueueTask( & Task { Name: "Task2" } )

  if spec.Tasks.GetCircularTask() != nil {
    t.Fatal("Task list is circular")
  }
}


func TestSpecDeferTaskIsNotCircular (t *testing.T) {
  /*
    Spec with only deferred tasks
  */
  spec_defer := NewSpec("defer-test", nil)

  spec_defer.DeferTask( & Task { Name: "Defer1" } )
  spec_defer.DeferTask( & Task { Name: "Defer2" } )

  if spec_defer.Tasks.GetCircularTask() != nil {
    t.Fatal("Task list is circular")
  }

  /*
    Spec with enqueued and deferred tasks
  */
  spec_enqueue_defer := NewSpec("defer-enqueue-test", nil)

  spec_enqueue_defer.EnqueueTask( & Task { Name: "Enqueue1" } )
  spec_enqueue_defer.DeferTask(   & Task { Name: "Defer1"   } )
  spec_enqueue_defer.EnqueueTask( & Task { Name: "Enqueue2" } )
  spec_enqueue_defer.DeferTask(   & Task { Name: "Defer2"   } )

  if spec_enqueue_defer.Tasks.GetCircularTask() != nil {
    t.Fatal("Task list is circular")
  }
}


func TestSpecEmptySingularRunFinishes (t *testing.T) {
  spec := NewSpec("single", nil)
  wrapTimeoutError(t, spec.Run)
}


func TestSpecEmptySingularRunEmitFinishes (t *testing.T) {
  spec := NewSpec("single", nil)

  spec.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  wrapTimeoutError(t, spec.Run)
}


func TestSpecChildRunEmitFinishes (t *testing.T) {
  root := NewSpec("root", nil)
  subspec := root.AddSubspec( NewSpec("subspec", nil ) )

  subspec.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  wrapTimeoutError(t, root.Run)
}


func TestSpecTreeRunEmitFinishes (t *testing.T) {
  root      := NewSpec("root", nil)
  subspec_a := root.AddSubspec( NewSpec("subspec_a", nil ) )
  subspec_b := root.AddSubspec( NewSpec("subspec_b", nil ) )

  subspec_a.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  subspec_b.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    return nil
  })

  wrapTimeoutError(t, root.Run)
}


func TestSpecChildRunEmitConsumesAssetFinishes (t *testing.T) {
  root    := NewSpec("root", nil)
  subspec := root.AddSubspec( NewSpec("subspec", nil ) )

  subspec.EnqueueTaskFunc("test-emit", func (s *Spec, t *Task) error {
    s.EmitAsset( & Asset {} )
    s.EmitAsset( & Asset {} )
    s.EmitAsset( & Asset {} )
    return nil
  })

  root.EnqueueTaskFunc("test-consume", func (s *Spec, task *Task) error {
    var asset_count int = 0
    for asset := range s.Input {
      if asset != nil {
        asset_count++
      }
    }

    if asset_count != 3 {
      t.Fatal("Did not consume exactly three assets")
    }

    return nil
  })

  wrapTimeoutError(t, root.Run)
}
