package interbuilder

import (
  "testing"
  "fmt"
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

    } else if length, expect := len(asset_frame.Assets), 9; length != expect {
      t.Errorf("Expected AssetFrame to have %d asset(1), got %d", expect, length)
    }

    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }
}
