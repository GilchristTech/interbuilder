package interbuilder

import (
  "testing"
  "fmt"
  "strings"
)


func TestAutomaticAssetFrames (t *testing.T) {
  var root = NewSpec("root", nil)
  var produce_spec = NewSpec("produce", nil)

  root.Props["quiet"] = true
  root.AddSubspec(produce_spec)

  var produce_func = func (sp *Spec, tk *Task) error {
    for n := 0; n < 3; n++ {
      var asset = sp.MakeAsset(fmt.Sprintf("%s/%d", tk.Name, n))
      asset.SetContentBytes([]byte("Hello, world!"))

      if err := tk.EmitAsset(asset); err != nil {
        return err
      }
    }

    return nil
  }

  produce_spec.EnqueueTaskFunc("produce-asset-a", produce_func)
  produce_spec.EnqueueTaskFunc("produce-asset-b", produce_func)
  produce_spec.EnqueueTaskFunc("produce-asset-c", produce_func)

  root.EnqueueTaskFunc("await-asset-frame", func (sp *Spec, tk *Task) error {
    asset_frames, err := tk.AwaitAssetFrames()
    if err != nil {
      return err
    }

    if asset_frame, ok := asset_frames["produce"]; !ok {
     return fmt.Errorf("Asset frame not found: produce")

    } else if length, expect := len(asset_frame.assets), 9; length != expect {
      // TODO: the above tests asset_frame.assets, which is private. This should be testing a public method or value.
      t.Errorf("Expected AssetFrame to have %d asset(1), got %d", expect, length)
    }

    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }
}


func TestAutomaticAssetFramesMapFuncFilter (t *testing.T) {
  var root = NewSpec("root", nil)
  var produce_spec = NewSpec("produce", nil)

  root.Props["quiet"] = true
  root.AddSubspec(produce_spec)

  var produce_func = func (sp *Spec, tk *Task) error {
    for n := 0; n < 3; n++ {
      var asset = sp.MakeAsset(fmt.Sprintf("%s/%d", tk.Name, n+1))

      if err := tk.EmitAsset(asset); err != nil {
        return err
      }
    }

    return nil
  }

  var filterTaskMapFunc = func (contains string) TaskMapFunc {
    return func (a *Asset) (*Asset, error) {
      if strings.Contains(a.Url.Path, contains) {
        return nil, nil
      }
      return a, nil
    }
  }

  produce_spec.EnqueueTaskFunc("produce-asset-a", produce_func)
  produce_spec.EnqueueTask(& Task {
    Name:    "filter-a",
    Mask:    TASK_ASSETS_FILTER_TASK,
    MapFunc: filterTaskMapFunc("a/3"),
  })

  produce_spec.EnqueueTaskFunc("produce-asset-b", produce_func)
  produce_spec.EnqueueTask(& Task {
    Name:    "filter-b",
    Mask:    TASK_ASSETS_FILTER_TASK,
    MapFunc: filterTaskMapFunc("b/2"),
  })

  produce_spec.EnqueueTaskFunc("produce-asset-c", produce_func)
  produce_spec.EnqueueTask(& Task {
    Name:    "filter-c",
    Mask:    TASK_ASSETS_FILTER_TASK,
    MapFunc: filterTaskMapFunc("c/1"),
  })

  root.EnqueueTaskFunc("await-asset-frame", func (sp *Spec, tk *Task) error {
    asset_frames, err := tk.AwaitAssetFrames()
    if err != nil {
      return err
    }

    if asset_frame, ok := asset_frames["produce"]; !ok {
     return fmt.Errorf("Asset frame not found: produce")

    } else if length, expect := len(asset_frame.assets), 6; length != expect {
      // TODO: the above tests asset_frame.assets, which is private. This should be testing a public method or value.
      t.Errorf("Expected AssetFrame to have %d asset(1), got %d", expect, length)

    } else {
      // Get asset inputs and assert that their paths are correct.

      tk.PoolSpecInputAssets()
      for _, asset := range tk.Assets {
        switch path := strings.TrimLeft(asset.Url.Path, "/"); path {
        case  "@emit/produce-asset-a/1", "@emit/produce-asset-a/2",
              "@emit/produce-asset-b/1", "@emit/produce-asset-b/3",
              "@emit/produce-asset-c/2", "@emit/produce-asset-c/3":
          key := path[5:]
          if _, in_frame := asset_frame.assets[key]; in_frame {
            t.Errorf("Expected Asset received but not found in AssetFrame: %s", path)
          }

        default:
          t.Errorf("Unexpected Asset path: %s", path)
        }
      }
    }

    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }
}
