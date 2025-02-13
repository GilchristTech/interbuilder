package interbuilder

import (
  "fmt"
  "net/url"
)


type AssetFrame struct {
  History *HistoryEntry
  Spec *Spec
  Assets map[string]AssetFrameEntry
}


func (af *AssetFrame) AddKey (key string) error {
  if _, has_key := af.Assets[key]; has_key {
    return nil
  }

  af.Assets[key] = AssetFrameEntry {}
  return nil
}

func (af *AssetFrame) HasKey (key string) bool {
  _, has_key := af.Assets[key]
  return has_key
}


type AssetFrameEntry struct {
  Url      *url.URL
  LastTask *Task
  asset    *Asset
}


func (ae *AssetFrameEntry) GetAsset () *Asset {
  return ae.asset
}


func (tk *Task) AwaitAssetFrames () (map[string]*AssetFrame, error) {
  var sp *Spec = tk.Spec

  if sp == nil {
    return nil, fmt.Errorf("Task %s cannot await AssetFrame, its Spec is nil", tk.Name)
  }

  sp.asset_frames_cond.L.Lock()
  defer sp.asset_frames_cond.L.Unlock()

  if sp.asset_frames_expect == 0 {
    return nil, nil
  }

  for sp.asset_frames_have != sp.asset_frames_expect {
    if sp.asset_frames_have > sp.asset_frames_expect {
      return nil, fmt.Errorf(
        "Spec %s has more asset frames (%d) than it expects (%d).",
        sp.Name, sp.asset_frames_have, sp.asset_frames_expect,
      )
    }
    sp.asset_frames_cond.Wait()
  }

  var asset_frames = sp.asset_frames
  return asset_frames, nil
}


func (tk *Task) AwaitAssetFrameName (name string) (*AssetFrame, error) {
  var sp *Spec = tk.Spec

  if sp == nil {
    return nil, fmt.Errorf("Task %s cannot await AssetFrame named %s, its Spec is nil", tk.Name, sp.Name)
  }

  if sp.asset_frames_expect == 0 {
    return nil, fmt.Errorf("Task %s cannot await AssetFrame named %s, Spec does not expect any asset frames", tk.Name, name)
  }

  sp.asset_frames_cond.L.Lock()
  defer sp.asset_frames_cond.L.Unlock()

  // If the AssetFrame is already there, just return it
  //
  if asset_frame := sp.asset_frames[name]; asset_frame != nil {
    return asset_frame, nil
  }

  // Wait for AssetFrames until this one is defined
  //
  for {
    if asset_frame := sp.asset_frames[name]; asset_frame != nil {
      return asset_frame, nil
    }
    sp.asset_frames_cond.Wait()
  }
}
