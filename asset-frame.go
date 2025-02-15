package interbuilder

import (
  "fmt"
  "net/url"
  "sync"
)


type AssetFrame struct {
  History *HistoryEntry
  Spec *Spec
  assets map[string]*AssetFrameEntry
  lock sync.Mutex
}


type AssetFrameEntry struct {
  Url       *url.URL
  asset     *Asset
  asset_err error
  lock      sync.RWMutex
  cond      *sync.Cond
}


func (af *AssetFrame) AddKey (key string) error {
  af.lock.Lock()
  defer af.lock.Unlock()

  if _, has_key := af.assets[key]; has_key {
    return nil
  }

  var entry = & AssetFrameEntry {}
  af.assets[key] = entry
  entry.cond = sync.NewCond(&entry.lock)
  return nil
}

func (af *AssetFrame) HasKey (key string) bool {
  _, has_key := af.assets[key]
  return has_key
}


func (ae *AssetFrameEntry) AwaitAsset () (*Asset, error) {
  ae.cond.L.Lock()
  defer ae.cond.L.Unlock()

  for {
    if ae.asset != nil || ae.asset_err != nil {
      return ae.asset, ae.asset_err
    }
    ae.cond.Wait()
  }

  return ae.asset, nil
}


func (ae *AssetFrameEntry) SetAsset () error {
  ae.cond.L.Lock()
  defer ae.cond.L.Unlock()

  if ae.asset != nil || ae.asset_err != nil {
    return fmt.Errorf("Cannot SetAsset in AssetFrame, Asset is already set.")
  }

  ae.cond.Broadcast()
  return nil
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
