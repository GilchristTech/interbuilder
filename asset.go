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
  "mime"
)


// Asset type masks
//
const (
  /* Asset type mask bit ranges */
  ASSET_FIELDS          uint64 = 0b_1_11111
  ASSET_FIELDS_QUANTITY uint64 = 0b_1_00000
  ASSET_FIELDS_ACCESS   uint64 = 0b_0_11111

  ASSET_TYPE_UNDEFINED  uint64 = 0b_0_00000

  /* Asset quantity bit: (whether asset is singular or pluralistic) */
  ASSET_QUANTITY_SINGLE uint64 = 0b_0_00000
  ASSET_QUANTITY_MULTI  uint64 = 0b_1_00000

  /* Singular asset types */
  ASSET_SINGLE_READER   uint64 = 0b_0_00001
  ASSET_SINGLE_WRITER   uint64 = 0b_0_00010

  /* Mult-asset types */
  ASSET_MULTI_ARRAY     uint64 = 0b_1_00001
  ASSET_MULTI_FUNC      uint64 = 0b_1_00010
  ASSET_MULTI_GENERATOR uint64 = 0b_1_00100
)


type Asset struct {
  Url       *url.URL
  History   *HistoryEntry
  Spec      *Spec

  Mimetype  string

  // Content
  //
  ContentModified bool
  ContentBytes    []byte

  // IO handling
  //
  FileSource string

  // Asset types: An asset struct can represent a singular asset, an array of
  // assets, or a lazy asset generator.
  //
  TypeMask         uint64

  get_reader_func  func (*Asset) (io.Reader, error)
  get_writer_func  func (*Asset) (io.Writer, error)

  asset_array      []*Asset
  asset_array_func func (*Asset) ([]*Asset, error)

  // Asset Generator: One asset may function like a generator for
  // other assets. In order to act like a generator, an Asset can
  // store pointers to generator functions which
  //
  generator_start func (a *Asset) (next func () (*Asset, error), err error)
  generator_next  func () (*Asset, error)
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
  if a.Url == nil {
    return fmt.Errorf("Cannot emit asset with a nil URL")
  }

  var modified       bool = false
  var url_path     string = a.Url.Path
  var url_prefix   string = ""
  var suffix_path  string = ""

  if strings.HasPrefix(url_path, "/@emit") {
    url_prefix  = url_path[:len("/@emit")]
    suffix_path = url_path[len("/@emit"):]
  } else if strings.HasPrefix(url_path, "@emit") {
    url_prefix  = url_path[:len("@emit")]
    suffix_path = url_path[len("@emit"):]
  } else {
    // TODO: detect leading and trailing slashes
    url_prefix = "@emit/"
    suffix_path = url_path
    modified = true
  }

  var suffix_path_original = suffix_path

  // Apply path transformations
  //
  for _, transformation := range s.PathTransformations {
    suffix_path = transformation.TransformPath(suffix_path)

    if !modified && (suffix_path != suffix_path_original) {
      modified = true
    }
  }

  // If the asset was modified, make a shallow copy
  //
  if modified {
    copied     := *a
    copied.Url  = s.MakeUrl(url_prefix + suffix_path)
    a           = & copied
  }

  for _, output := range s.OutputChannels {
    (*output) <- a
  }

  return nil
}


func (s *Spec) EmitFileKey (file_path string, key_parts ...string) error {
  var key = append([]string {"@emit"}, key_parts...)
  asset, err := s.MakeFileKeyAsset(file_path, key...)
  if err != nil { return fmt.Errorf("Error emitting file file with key %s: %w", key, err) }
  return s.EmitAsset(asset)
}


/*
  Relative to this Spec's `source_dir` path, look for a file at
  `source_path`, and create a filesystem asset with a URL key at
  `key_parts`.
*/
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

  var asset_url *url.URL = s.MakeUrl(key)

  var history = HistoryEntry {
    Url:     asset_url,
    Parents: [] *HistoryEntry { &s.History },
    Time:    time.Now(),
  }


  // TODO: specify means of singular access
  var type_mask uint64 = ASSET_TYPE_UNDEFINED

  var asset = Asset {
    Url:          asset_url,
    History:      & history,
    Spec:         s,
    Mimetype:     mimetype,
    TypeMask:     type_mask,
    FileSource:   file_path,
  }

  if is_dir {
    // This asset is a directory. Populate it with pluralistic callback functions
    asset.Mimetype = "inode/directory"

    type_mask = ASSET_MULTI_FUNC | ASSET_MULTI_GENERATOR
    asset.TypeMask = type_mask

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
        asset, err := s.MakeFileKeyAsset(file_path, asset.Url.Path, key)

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

      asset, err :=  s.MakeFileKeyAsset(file_path, key)
      if err != nil { return nil, err }

      return asset, nil
    }
  } else {
    // This asset is a file. Populate it with callbacks for IO handling

    asset.TypeMask |= ASSET_SINGLE_READER | ASSET_SINGLE_WRITER
    asset.Mimetype  = mime.TypeByExtension(filepath.Ext(file_path))

    asset.get_reader_func = func (a *Asset) (io.Reader, error) {
      return os.Open(a.FileSource)
    }

    asset.get_writer_func = func (a *Asset) (io.Writer, error) {
      return os.Create(a.FileSource)
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


func (a *Asset) GetReader () (io.Reader, error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Cannot get reader, asset is not singular")
  }

  if a.TypeMask & ASSET_SINGLE_READER == 0 {
    return nil, fmt.Errorf("Cannot get reader, asset does not have a reader type")
  }

  if a.get_reader_func == nil {
    return nil, fmt.Errorf("Cannot get reader, asset does not have a reader-getter function defined")
  }

  return a.get_reader_func(a)
}


func (a *Asset) GetWriter () (io.Writer, error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Cannot get writer, asset is not singular")
  }

  if a.TypeMask & ASSET_SINGLE_READER == 0 {
    return nil, fmt.Errorf("Cannot get writer, asset does not have a writer type")
  }

  if a.get_writer_func == nil {
    return nil, fmt.Errorf("Cannot get writer, asset does not have a writer-getter function defined")
  }

  return a.get_writer_func(a)
}


func (a *Asset) GetContentBytes () ([]byte, error) {
  if a.ContentBytes != nil {
    return a.ContentBytes, nil
  }

  reader, err := a.GetReader()
  if err != nil { return nil, err }

  bytes, err := io.ReadAll(reader)
  if err != nil { return nil, err }

  a.ContentBytes = bytes
  return bytes, nil
}


func (a *Asset) SetContentBytes (content []byte) error {
  if ! a.IsSingle() {
    return fmt.Errorf("Asset is not singular")
  }

  a.ContentBytes = content
  a.ContentModified = true
  return nil
}
