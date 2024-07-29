package interbuilder

import (
  "net/url"
  "io"
  "time"
  "fmt"
  "os"
  "path"
  "path/filepath"
  "strings"
  "io/fs"
)


// Asset type masks
//
const (
  /* Asset type mask bit ranges */
  ASSET_FIELDS_QUANTITY   = 0b00_00000_1
  ASSET_FIELDS_ACCESS     = 0b00_11111_0
  ASSET_FIELDS_TRANSFER   = 0b11_00000_0

  ASSET_TYPE_UNDEFINED    = 0b00_00000_0

  /* Asset quantity bit: (whether asset is singular or pluralistic) */
  ASSET_QUANTITY_SINGLE   = 0b00_00000_0
  ASSET_QUANTITY_MULTI    = 0b00_00000_1

  /* Singular asset types */
  ASSET_SINGLE_READER     = 0b00_00001_0
  ASSET_SINGLE_WRITER     = 0b00_00010_0
  ASSET_SINGLE_BYTES      = 0b00_00100_0
  ASSET_SINGLE_STRING     = 0b00_01000_0
  ASSET_SINGLE_DATA       = 0b00_10000_0

  /* Mult-asset types */
  ASSET_MULTI_ARRAY       = 0b00_00001_1
  ASSET_MULTI_FUNC        = 0b00_00010_1
  ASSET_MULTI_GENERATOR   = 0b00_00100_1

  /* Asset transfer modes */
  ASSET_TRANSFER_COPY     = 0b00_00000_0
  ASSET_TRANSFER_NONE     = 0b01_00000_0
  ASSET_TRANSFER_MOVE     = 0b10_00000_0
  ASSET_TRANSFER_LINK     = 0b11_00000_0
)


var ASSET_MULTI_TYPES = []int {
  ASSET_MULTI_ARRAY,
  ASSET_MULTI_FUNC,
  ASSET_MULTI_GENERATOR,
}


type Asset struct {
  Url       *url.URL
  History   *HistoryEntry
  Spec      *Spec

  Mimetype  string
  Data      any

  // IO handling
  //
  FilePath        string
  ReadFilePath    string
  WriteFilePath   string
  Size            int
  was_read        bool
  is_directory    bool

  content         any
  content_bytes   *[]byte
  content_string  string
  reader          io.Reader

  // Asset types: An asset struct can represent a singular asset, an array of
  // assets, or a lazy asset generator.
  //
  TypeMask         int
  asset_array      []*Asset
  asset_array_func func (*Asset) ([]*Asset, error)

  // Asset Generator: One asset may function like a generator for
  // other assets. In order to act like a generator, an Asset can
  // store pointers to generator functions which
  //
  generator_start   func (a *Asset) (next func () (*Asset, error), err error)
  generator_next    func () (*Asset, error)
}


func (a *Asset) ExtendHistory (add_parents ...*HistoryEntry) *HistoryEntry {
  parents := make([]*HistoryEntry, 0, 1+len(add_parents))
  parents  = append(parents, a.History)
  parents  = append(parents, add_parents...)

  return & HistoryEntry {
    Url:     a.Url,
    Parents: parents,
    Time:    time.Now(),
  }
}


func (a *Asset) GenerateAssets () (next_func func () (*Asset, error), err error) {
  var nextFunc func() (*Asset, error)

  if a.generator_start != nil {
    nextFunc, err = a.generator_start(a)
    if err != nil { return nil, err }
  } else {
    nextFunc = a.generator_next
  }

  if nextFunc == nil {
    return nil, fmt.Errorf("No generator next function defined")
  }

  return nextFunc, nil
}


func (a *Asset) GenerateAssetsArray () ([]*Asset, error) {
  var assets = make([]*Asset, 0)

  nextFunc, err := a.GenerateAssets()
  if err != nil { return nil, err }

  for {
    asset, err := nextFunc()
    if asset == nil || err != nil {
      return assets, err
    }

    assets = append(assets, asset)
  }
}


type HistoryEntry struct {
  Url     *url.URL
  Parents []*HistoryEntry
  Time    time.Time
}


/*
  For a given filesystem path, relative to the source_dir
  property, return whether that path exists; as well as any error
  in determing this.
*/
func (s *Spec) PathExists (local_path string) (bool, error) {
  spec_source, err := s.RequirePropString("source_dir")
  if err != nil { return false, err }

  abs_path, err := filepath.Abs(path.Join(spec_source, local_path))
  if err != nil { return false, err }

  _, err = os.Stat(abs_path)
  if err != nil {
    if os.IsNotExist(err) {
      return false, nil
    }
    return false, err
  }

  return true, nil
}


/*
  Given a filesystem path inside the Spec's source_dir, return
  the relative path to the source_dir. Errors if the path is not
  within the Spec's source_dir.
*/
func (s *Spec) GetPathKey (p string) (string, error) {
  spec_source, err := s.RequirePropString("source_dir")
  if err != nil { return "", err }
  return filepath.Rel(spec_source, p)
}


/*
  Convert a Spec Asset key into a filesystem path.
*/
func (s *Spec) GetKeyPath (k string) (string, error) {
  spec_source, err := s.RequirePropString("source_dir")
  if err != nil { return "", err }

  if os.PathSeparator != '/' {
    k = strings.ReplaceAll(k, "/", string(os.PathSeparator))
  }

  return filepath.Join(spec_source, k), nil
}


func (s *Spec) EmitAsset (a *Asset) error {
  for _, output := range s.OutputChannels {
    (*output) <- a
  }
  return nil
}


func (s *Spec) EmitFileKey (file_path string, key_parts ...string) error {
  asset, err := s.MakeFileKeyAsset(file_path, key_parts...)
  if err != nil { return err }
  return s.EmitAsset(asset)
}


func (s *Spec) MakeFileKeyAsset (source_path string, key_parts ...string) (*Asset, error) {
  source_dir, err := s.RequirePropString("source_dir")
  if err != nil { return nil, err }

  var key string

  if len(key_parts) == 0 {
    // TODO: assert that the path separator is a forward-slash
    key = source_path
  } else {
    key = path.Join(key_parts...)
  }

  var file_path string = source_path

  if !strings.HasPrefix(file_path, source_dir) {
    file_path = filepath.Join(source_dir, source_path)
  }

  var mimetype string = ""

  file_info, err := os.Stat(file_path)
  if err != nil { return nil, err }

  // TODO: check for symbolic links
  var is_dir bool = file_info.IsDir() 

  var asset_url *url.URL = s.MakeUrl("@emit", key)

  var history = HistoryEntry {
    Url:     asset_url,
    Parents: [] *HistoryEntry { &s.History },
    Time:    time.Now(),
  }

  var asset = Asset {
    Url:        asset_url,
    History:    & history,
    Spec:       s,
    Mimetype:   mimetype,
    Size:       -1,
    TypeMask:   ASSET_TYPE_UNDEFINED, // TODO: specify means of singular access

    is_directory: is_dir,

    FilePath:   file_path,
  }

  if is_dir {
    asset.Mimetype = "inode/directory"
    asset.TypeMask = ASSET_MULTI_FUNC | ASSET_MULTI_GENERATOR

    var keys = make([]string, 0)
    var walk_err error = nil

    filepath.WalkDir(file_path, func (rooted_path string, entry fs.DirEntry, err error) error {
      walk_err = err
      if entry.IsDir() {
        return nil
      }

      keys = append(keys, rooted_path[ len(file_path) : ])
      return nil
    })

    if walk_err != nil {
      return nil, walk_err
    }

    asset.asset_array_func = func (a *Asset) ([]*Asset, error) {
      var assets = make([]*Asset, 0, len(keys))

      for _, key := range keys {
        var file_path string = filepath.Join(file_path, key)
        asset, err := s.MakeFileKeyAsset(file_path, key)
        if err != nil { return nil, err }
        assets = append(assets, asset)
      }

      return assets, nil
    }
    
    var generator_index int = 0
    asset.generator_next = func () (*Asset, error) {
      if generator_index >= len(keys) {
        return nil, nil
      }

      var key string = keys[generator_index]
      generator_index++

      var file_path string = filepath.Join(file_path, key)

      return s.MakeFileKeyAsset(file_path, key)
    }
  }

  return &asset, nil
}


func (a *Asset) IsSingle () bool {
  return a.TypeMask & ASSET_FIELDS_QUANTITY == ASSET_QUANTITY_SINGLE
}


func (a *Asset) IsMulti () bool {
  return a.TypeMask & ASSET_FIELDS_QUANTITY == ASSET_QUANTITY_MULTI
}


func (a *Asset) Expand () ([]*Asset, error) {
  if a.IsSingle() {
    return []*Asset { a }, nil
  }

  var access = a.TypeMask & ASSET_FIELDS_ACCESS

  if access & ASSET_MULTI_ARRAY != 0 {
    return a.asset_array, nil
  }

  if access & ASSET_MULTI_FUNC != 0 {
    return a.asset_array_func(a)
  }

  if access & ASSET_MULTI_GENERATOR != 0 {
    return a.GenerateAssetsArray()
  }

  return nil, fmt.Errorf("Unsupported asset type mask 0x%X", a.TypeMask)
}
