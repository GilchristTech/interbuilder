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
  "bytes"
  "io/fs"
  "mime"
)


// Asset type masks
//
const (
  /* Asset type mask bit ranges */
  ASSET_FIELDS          uint64 = 0b_111_111
  ASSET_FIELDS_QUANTITY uint64 = 0b_100_000
  ASSET_FIELDS_ACCESS   uint64 = 0b_011_111

  ASSET_TYPE_UNDEFINED  uint64 = 0b_000_000

  /* Asset quantity bit: (whether asset is singular or pluralistic) */
  ASSET_QUANTITY_SINGLE uint64 = 0b_000_000
  ASSET_QUANTITY_MULTI  uint64 = 0b_100_000

  /* Singular asset types: how its data is read/written */
  ASSET_SINGLE_BYTE_R   uint64 = 0b_000_001 // Byte reader
  ASSET_SINGLE_BYTE_W   uint64 = 0b_000_010 // Byte writer
  ASSET_SINGLE_DATA_R   uint64 = 0b_000_100 // Data reader
  ASSET_SINGLE_DATA_W   uint64 = 0b_001_000 // Data writer

  /* Mult-asset types: how it is expanded into other assets */
  ASSET_MULTI_ARRAY     uint64 = 0b_100_001
  ASSET_MULTI_FUNC      uint64 = 0b_100_010
  ASSET_MULTI_GENERATOR uint64 = 0b_100_100
)


type Asset struct {
  Url       *url.URL
  History   *HistoryEntry
  Spec      *Spec

  Mimetype  string

  //
  // Content:
  // Assets track content in two ways: a byte buffer
  // (Asset.ContentBytes) and a general-purpose field
  // (Asset.ContentData). From these functions, higher-level
  // methods either return cached content or find a way to read
  // the data.
  //
  ContentBytes             []byte
  ContentModified          bool

  ContentData              any
  ContentDataModified      bool
  ContentDataSet           bool

  content_data_read_func   func (*Asset, io.Reader) (any, error)
  content_data_write_func  func (*Asset, io.Writer, any) (int, error)

  // Because Assets track two separate forms of the same data
  // (byte content and untyped data content),
  // and because those can diverge after mutation, tracking
  // whether these two areas in memory represent the same state
  // of content is done with the byte_data_parity flag.
  // When content is read from bytes into data (or vice versa),
  // the content between them has parity.
  //
  has_byte_data_parity bool

  // IO handling
  //
  FileSource string
  FileDest   string

  // Asset types: An asset struct can represent a singular asset, an array of
  // assets, or a lazy asset generator.
  //
  TypeMask         uint64

  content_bytes_get_reader_func func (*Asset) (io.Reader, error)
  content_bytes_get_writer_func func (*Asset) (io.Writer, error)

  asset_array      []*Asset
  asset_array_func func (*Asset) ([]*Asset, error)

  // Asset Generator: One asset may function like a generator for
  // other assets. In order to act like a generator, an Asset can
  // store pointers to generator functions which
  //
  generator_start func (a *Asset) (next func () (*Asset, error), err error)
  generator_next  func () (*Asset, error)
}


/*
  Creates a new history entry, meant to represent a departure
  from this asset in its history tree. This asset's history entry
  is the first of the parents, and more can be supplied through
  variadic arguments.
*/
func (a *Asset) ExtendHistory (add_parents ...*HistoryEntry) *HistoryEntry {
  var parents []*HistoryEntry

  if len(add_parents) == 0 {
    parents = []*HistoryEntry { a.History }
  } else {
    parents = make([]*HistoryEntry, 0, 1+len(add_parents))
    parents = append(parents, a.History)
    parents = append(parents, add_parents...)
  }

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


func (a *Asset) SetAssetArray (assets []*Asset) error {
  if a.TypeMask & ASSET_FIELDS_ACCESS != 0 {
    return fmt.Errorf("Cannot set asset array, type mask has existing access bits set, type mask: %O", a.TypeMask)
  }

  a.TypeMask |= ASSET_MULTI_ARRAY
  a.asset_array = assets
  return nil
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


/*
  WriteFile writes data to a file using os.WriteFile, except the a
  file key local to the spec's source dir is resolved into a file
  path.
*/
func (s *Spec) WriteFile (key string, data []byte, perm fs.FileMode) error {
  file_path, err := s.GetKeyPath(key)
  if err != nil { return err }

  dir_path, _ := filepath.Split(file_path)
  
  if err := os.MkdirAll(dir_path, os.ModePerm); err != nil {
    return err
  }

  return os.WriteFile(file_path, data, perm)
}


func (s *Spec) EmitAsset (a *Asset) error {

  if a.Url == nil {
    return fmt.Errorf("Cannot emit asset with a nil URL")
  }

  var modified       bool = false
  var url_path     string = a.Url.Path
  var url_prefix   string = ""
  var suffix_path  string = ""

  if strings.HasPrefix(url_path, "/@emit/") {
    url_prefix  = url_path[:len("/@emit/")]
    suffix_path = url_path[len("/@emit/"):]
  } else if strings.HasPrefix(url_path, "@emit/") {
    url_prefix  = url_path[:len("@emit/")]
    suffix_path = url_path[len("@emit/"):]
  } else {
    // TODO: detect leading and trailing slashes
    url_prefix = "/@emit/"
    suffix_path = url_path
    modified = true
  }

  suffix_path = strings.TrimLeft(suffix_path, "/")
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

  s.OutputAsset(a)

  return nil
}


func (s *Spec) OutputAsset (a *Asset) {
  for _, output := range s.OutputChannels {
    (*output) <- a
  }
}


func (s *Spec) EmitFileKey (file_path string, key_parts ...string) error {
  var key string

  if len(key_parts) == 0 {
    // TODO: assert source_path is a relative path
    // TODO: assert that the path separator is a forward-slash
    key = file_path
  } else {
    key = path.Join(key_parts...)
  }

  asset, err := s.MakeFileKeyAsset(file_path, key)
  if err != nil { return fmt.Errorf("Error emitting file file with key %s: %w", key, err) }
  return s.EmitAsset(asset)
}


func (s *Spec) MakeAsset (key ...string) *Asset {
  var asset_url *url.URL = s.MakeUrl(key...)

  var history = HistoryEntry {
    Url:     asset_url,
    Parents: [] *HistoryEntry { &s.History },
    Time:    time.Now(),
  }

  var asset = Asset {
    Url:     asset_url,
    Spec:    s,
    History: &history,
  }

  return &asset
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
    // TODO: assert source_path is a relative path
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

  var new_asset = Asset {
    Url:          asset_url,
    History:      & history,
    Spec:         s,
    Mimetype:     mimetype,
    TypeMask:     type_mask,
    FileSource:   file_path,
    FileDest:     file_path,
  }

  if is_dir {
    // This asset is a directory. Populate it with pluralistic callback functions
    new_asset.Mimetype = "inode/directory"

    type_mask = ASSET_MULTI_FUNC | ASSET_MULTI_GENERATOR
    new_asset.TypeMask = type_mask

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

    new_asset.asset_array_func = func (base_asset *Asset) ([]*Asset, error) {
      var assets = make([]*Asset, 0, len(keys))

      for _, key := range keys {
        var file_path string = filepath.Join(file_path, key)
        asset, err := s.MakeFileKeyAsset(file_path, base_asset.Url.Path, key)

        if err != nil { return nil, err }
        assets = append(assets, asset)
      }

      return assets, nil
    }
    
    var generator_index int = 0
    new_asset.generator_next = func () (*Asset, error) {
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

    new_asset.Mimetype  = mime.TypeByExtension(filepath.Ext(file_path))

    err := new_asset.SetContentBytesGetReaderFunc(func (a *Asset) (io.Reader, error) {
      return os.Open(a.FileSource)
    })
    if err != nil { return nil, err }

    new_asset.SetContentBytesWriterFunc(func (a *Asset) (io.Writer, error) {
      if a.FileDest == "" {
        return nil, fmt.Errorf("FileDest in asset %s not defined", a.Url)
      }

      var directory, _ = path.Split(a.FileDest)

      err = os.MkdirAll(directory, os.ModePerm)
      if err != nil { return nil, err }

      return os.Create(a.FileDest)
    })
    if err != nil { return nil, err }
  }

  return &new_asset, nil
}


func (s *Spec) AnnexAsset (a *Asset) (*Asset) {
  // Create a shallow copy of the asset and update the URL
  //
  var annexed   Asset = *a
  var new_url url.URL = *a.Url

  new_url.Host = s.Url.Host
  annexed.Url  = & new_url

  // Calculate file write path
  //
  source_dir, _ := s.RequireInheritPropString("source_dir")
  var key string = annexed.Url.Path

  if strings.HasPrefix(key, "@emit") {
    key = key[ len("@emit") : ]
  } else if strings.HasPrefix(key, "/@emit") {
    key = key[ len("/@emit") : ]
  }

  annexed.FileDest = filepath.Join(source_dir, key)

  var history_parents = make([]*HistoryEntry, 2, 2)
  history_parents[0] = a.History
  history_parents[1] = & s.History

  annexed.History = & HistoryEntry {
    Url:     annexed.Url,
    Parents: history_parents,
    Time:    time.Now(),
  }

  return &annexed
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


func (a *Asset) Flatten () ([]*Asset, error) {
  var err error

  root_assets, err := a.Expand()
  if err != nil { return nil, err }

  flattened_assets := make([]*Asset, 0, len(root_assets))

  for _, root_asset := range root_assets {
    if root_asset.IsSingle() {
      flattened_assets = append(flattened_assets, root_asset)
      continue
    }

    assets, err := root_asset.Flatten()
    if err != nil { return nil, err }
    flattened_assets = append(flattened_assets, assets...)
  }

  return flattened_assets, nil
}


func (a *Asset) SetContentBytesGetReaderFunc (f func (*Asset) (io.Reader, error)) error {
  if ! a.IsSingle() {
    return fmt.Errorf("Cannot set get-reader function, asset is not singular")
  }
  a.TypeMask |= ASSET_SINGLE_BYTE_R
  a.content_bytes_get_reader_func = f
  return nil
}


func (a *Asset) SetContentBytesWriterFunc (f func (*Asset) (io.Writer, error)) error {
  if ! a.IsSingle() {
    return fmt.Errorf("Cannot set get-writer function, asset is not singular")
  }

  a.TypeMask |= ASSET_SINGLE_BYTE_W
  a.content_bytes_get_writer_func = f
  return nil
}


func (a *Asset) ContentBytesGetReader () (io.Reader, error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Cannot get reader, asset is not singular")
  }

  if a.TypeMask & ASSET_SINGLE_BYTE_R == 0 {
    return nil, fmt.Errorf("Cannot get reader, asset does not have a reader type")
  }

  if a.content_bytes_get_reader_func == nil {
    return nil, fmt.Errorf("Cannot get reader, asset does not have a reader-getter function defined")
  }

  return a.content_bytes_get_reader_func(a)
}


func (a *Asset) ContentBytesGetWriter () (io.Writer, error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Cannot get writer, asset is not singular")
  }

  if a.TypeMask & ASSET_SINGLE_BYTE_R == 0 {
    return nil, fmt.Errorf("Cannot get writer, asset does not have a writer type")
  }

  if a.content_bytes_get_writer_func == nil {
    return nil, fmt.Errorf("Cannot get writer, asset does not have a writer-getter function defined")
  }

  return a.content_bytes_get_writer_func(a)
}


func (a *Asset) writeContentDataToContentBytes () ([]byte, error) {
  // Read ContentData into ContentBytes, and set the parity
  // flag.

  writer := bytes.NewBuffer([]byte{})
  if _, err := a.WriteContentDataTo(writer); err != nil {
    return nil, fmt.Errorf("Error writing asset content data to asset content bytes: %w", err)
  }
  a.ContentBytes = writer.Bytes()

  a.has_byte_data_parity = true
  return a.ContentBytes, nil
}


func (a *Asset) GetContentBytes () ([]byte, error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Asset is not singular")
  }

  if a.ContentData != nil {
    if a.ContentBytes == nil {
      return a.writeContentDataToContentBytes()
    }

    // Content data and bytes are defined. If they're in the same
    // state, we can return the bytes data.
    //
    if a.has_byte_data_parity {
      return a.ContentBytes, nil
    }

    // ContentData and ContentBytes are both defined, but do not
    // have parity. If only one of them is modified, try to use
    // that one.
    //
    if a.ContentDataModified && a.ContentModified {
      return nil, fmt.Errorf("Asset has divergent content and data modifications")
    } else if a.ContentDataModified {
      return a.ContentBytes, nil
    } else if a.ContentModified {
      return a.writeContentDataToContentBytes()
    }

    // Nothing is modified. Do not return and continue trying to
    // read bytes.
  }

  if a.ContentBytes != nil {
    return a.ContentBytes, nil
  }

  if a.content_bytes_get_reader_func == nil {
    return nil, nil
  }

  reader, err := a.ContentBytesGetReader()
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


func (a *Asset) GetContentData () (any, error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Cannot get data, asset is not singular")
  }

  if a.ContentData != nil {
    return a.ContentData, nil
  }

  if a.TypeMask & ASSET_SINGLE_DATA_R != ASSET_SINGLE_DATA_R {
    return nil, fmt.Errorf(
      "Cannot get data, asset has no data loader type bit set",
    )
  }

  if a.content_data_read_func == nil {
    return nil, fmt.Errorf(
      "Cannot get data, asset has no data loader set.",
    )
  }

  if a.ContentBytes != nil {
    reader    := bytes.NewReader(a.ContentBytes)
    data, err := a.content_data_read_func(a, reader)
    if err != nil {
      return nil, fmt.Errorf(
        "Error while reading data from content buffer: %w", err,
      )
    }
    a.ContentData = data
    return data, nil
  } else {
    reader, err := a.ContentBytesGetReader()
    if err != nil {
      return nil, fmt.Errorf(
        "Cannot get data, error getting reader: %w", err,
      )
    }

    data, err := a.content_data_read_func(a, reader)
    if err != nil {
      return nil, fmt.Errorf(
        "Error while reading data from content reader: %w", err,
      )
    }
    a.ContentData = data
    return data, nil
  }
}


func (a *Asset) SetContentData (data any) error {
  if ! a.IsSingle() {
    return fmt.Errorf("Asset is not singular")
  }

  a.ContentData     = data
  a.ContentModified = true
  return nil
}


func (a *Asset) SetContentDataReadFunc (f func (a *Asset, r io.Reader) (any, error)) error {
  if ! a.IsSingle() {
    return fmt.Errorf("Asset is not singular")
  }

  a.TypeMask |= ASSET_SINGLE_DATA_R
  a.content_data_read_func = f
  
  // Because setting the function that reads new ContentBytes can
  // result in different ContentBytes which would have occured
  // following a pas read of content data, not clearing the cache
  // could result in a drift from mutations in the two instances
  // of representative data. Reads of further data should reread,
  // which would be caused by clearning the data cache.
  //
  a.ClearContentDataCache()

  return nil
}


func (a *Asset) SetContentDataWriteFunc (f func (*Asset, io.Writer, any) (int, error)) error {
  if ! a.IsSingle() {
    return fmt.Errorf("Asset is not singular")
  }

  a.TypeMask |= ASSET_SINGLE_DATA_W
  a.content_data_write_func = f
  return nil
}


func (a *Asset) GetContentDataWriteFunc () (func (*Asset, io.Writer, any)(int, error), error) {
  if ! a.IsSingle() {
    return nil, fmt.Errorf("Asset is not singular")
  }

  if a.content_data_write_func == nil {
    return nil, fmt.Errorf("Cannot get data write function, content data function not set")
  }

  return a.content_data_write_func, nil
}


/*
  ReadDataTo reads data from the parameter using the Asset's data
  reader. This does read or write from any internal data cache
  (but it's possible the reader function does).
*/
func (a *Asset) WriteContentDataTo (to io.Writer) (int, error) {
  if ! a.IsSingle() {
    return 0, fmt.Errorf("Cannot write, asset is not singular")
  }

  writeData, err := a.GetContentDataWriteFunc()
  if err != nil {
    return 0, fmt.Errorf("Cannot get data writer function: %w", err)
  }
  return writeData(a, to, a.ContentData)
}


func (a *Asset) ClearContentCache () {
  a.ClearContentByteCache()
  a.ClearContentDataCache()
}


func (a *Asset) ClearContentByteCache () {
  a.ContentBytes         = nil
  a.ContentModified      = false
  a.has_byte_data_parity = false
}


func (a *Asset) ClearContentDataCache () {
  a.ContentData          = nil
  a.ContentDataModified  = false
  a.has_byte_data_parity = false
}
