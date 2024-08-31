package interbuilder

import (
  "testing"
  "net/url"
  "strconv"
  "path"
  "path/filepath"
  "os"
  "fmt"
  "io"
  "time"
  "bytes"
)


func TestAssetExtendHistory (t *testing.T) {
  var asset_url, _     = url.Parse("ib://test/@emit/asset.txt")
  // var updated_url, _ = url.Parse("ib://test/@emit/new-asset.txt")
  var history_b_url, _ = url.Parse("ib://spec/")

  var history_a = HistoryEntry { Url: asset_url,     Time: time.Now() }
  var history_b = HistoryEntry { Url: history_b_url, Time: time.Now() }

  var asset = & Asset {
    Url:     asset_url,
    History: & history_a,
  }

  // Test case: unmodified Asset.ExtendHistory with no arguments
  //
  var expanded_a *HistoryEntry = asset.ExtendHistory()

  if len(expanded_a.Parents) != 1 || expanded_a.Parents[0] != &history_a {
    t.Fatal("Expanded history is not exclusively a pointer to the previous history entry")
  }

  if ! expanded_a.Time.After(history_a.Time) {
    t.Error("Expanded history time is not greater than parent's time")
  }

  if got, expected := expanded_a.Url.String(), asset_url.String(); got != expected {
    t.Errorf("Expanded URL is not the asset URL, expected %s, got %s", expected, got)
  }

  // Test case: unmodified Asset.ExtendHistory with a parent argument
  //
  var expanded_b *HistoryEntry = asset.ExtendHistory(&history_b)

  if length := len(expanded_b.Parents); length != 2 {
    t.Fatalf("Expanded history does not have two parents, got %d", length)
  }

  for _, parent := range expanded_b.Parents {
    if ! expanded_b.Time.After(parent.Time) {
      t.Error("Expanded history time is not greater than a parent's time")
    }

    parent_url := parent.Url.String()

    expected_urls := make(map[string]bool)
    expected_urls[asset_url.String()] = true
    expected_urls[history_b_url.String()] = true

    if _, parent_url_found := expected_urls[parent_url]; !parent_url_found {
      var expected_urls_text string
      for expected_url, _ := range expected_urls {
        expected_urls_text += "\n  " + expected_url
      }
      t.Errorf("Unexpected parent URL: %s; expected URLs:%s", parent_url, expected_urls_text)
    }
  }

  // TODO: test cases where asset URL has been modified
}


func TestAssetExpandSingular (t *testing.T) {
  var testing_assets = []*Asset {
    & Asset {},
    & Asset { TypeMask: ASSET_QUANTITY_SINGLE },
  }

  for i, asset := range testing_assets {
    assets, err := asset.Expand()
    if err != nil {
      t.Fatalf("Error in asset #%d: %s", i, err)
    }

    if len(assets) != 1 {
      t.Fatalf("Expanded assets array in asset #%d does not have a length of 1", i)
    }
  }
}


func TestAssetExpandArray (t *testing.T) {
  var type_mask uint64 = ASSET_MULTI_ARRAY
  var base_url, _      = url.Parse("ib://testing/mask")
  var test_url         = base_url.JoinPath(strconv.FormatUint(type_mask, 2))

  var asset_array = make([]*Asset, 3)

  for i := 0 ; i < 3 ; i++ {
    asset_array[i] = & Asset {
      Url: base_url.JoinPath(strconv.Itoa(i)),
    }
  }

  var test_asset = & Asset {
    Url:         test_url,
    TypeMask:    type_mask,
    asset_array: asset_array,
  }
  
  assets, err := test_asset.Expand()
  if err != nil {
    t.Fatalf("Error in asset: %s", err)
  }

  if len(assets) != 3 {
    t.Fatalf("Expanded assets array in test asset does not have a length of 3, got %d", len(assets))
  }
}


func TestAssetExpandArrayFunc (t *testing.T) {
  var test_url, _ = url.Parse("ib://testing/mask")
  var type_mask   = ASSET_MULTI_FUNC
  var base_url    = test_url.JoinPath(strconv.FormatUint(type_mask, 2))

  var test_asset = & Asset {
    Url:      base_url,
    TypeMask: ASSET_MULTI_FUNC,

    asset_array_func: func (a *Asset) ([]*Asset, error) {
      var asset_array = make([]*Asset, 3)
      for i := 0 ; i < 3 ; i++ {
        asset_array[i] = & Asset {
          Url: base_url.JoinPath(strconv.Itoa(i)),
        }
      }
      return asset_array, nil
    },
  }

  assets, err := test_asset.Expand()

  if err != nil {
    t.Fatalf("Error in test asset: %s", err)
  }

  if len(assets) != 3 {
    t.Fatalf("Expanded assets array in test asset expected to have a length of 3, got %d", len(assets))
  }
}


func TestAssetExpandGenerator (t *testing.T) {
  // With generators having custom conditions for their
  // termination, wrap this in a timeout in case the generator
  // doesn't terminate.
  //
  TestWrapTimeout(t, func () {
    var test_url, _      = url.Parse("ib://testing/mask")
    var type_mask uint64 = ASSET_MULTI_GENERATOR
    var base_url         = test_url.JoinPath(strconv.FormatUint(type_mask, 2))

    var generator_start = func (a *Asset) (func()(*Asset, error), error) {
      var asset_array = make([]*Asset, 3)
      for i := 0 ; i < 3 ; i++ {
        asset_array[i] = & Asset {
          Url: base_url.JoinPath(strconv.Itoa(i)),
        }
      }

      var index int = 0

      return func () (*Asset, error) {
        // Terminate generator
        if index >= len(asset_array) {
          return nil, nil
        }

        var asset *Asset = asset_array[index]
        index++
        return asset, nil
      }, nil
    }

    var test_asset = & Asset {
      Url:             base_url,
      TypeMask:        type_mask,
      generator_start: generator_start,
    }

    assets, err := test_asset.Expand()

    if err != nil {
      t.Fatalf("Error in test asset: %s", err)
    }

    if len(assets) != 3 {
      t.Fatalf("Expanded assets array in test asset expected to have a length of 3, got %d", len(assets))
    }
  })
}


func TestAssetFlattenNestedMultiAssets (t *testing.T) {
  var spec *Spec = NewSpec("spec", nil)

  var asset_single_counter int
  var asset_multi_counter  int

  var makeNestedAssets   func (base_key string, level int) *Asset
  makeNestedAssets     = func (base_key string, level int) *Asset {
    if level <= 0 {
      asset := spec.MakeAsset(
        base_key, fmt.Sprintf("single_%d.%d", level, asset_single_counter),
      )
      asset_single_counter++
      return asset
    }

    base_key = path.Join(
        base_key, fmt.Sprintf("%d.%d", level, asset_multi_counter),
      )
    asset := spec.MakeAsset(base_key)
    asset_multi_counter++

    asset.SetAssetArray([]*Asset {
      makeNestedAssets(base_key, 0),
      makeNestedAssets(base_key, level-1),
      makeNestedAssets(base_key, level-1),
    })

    return asset
  }

  var root_asset = makeNestedAssets("", 5)

  flattened, err := root_asset.Flatten()

  if err != nil {
    t.Fatalf("Error flattening assets: %v", err)
  }

  var all_assets_singular bool = true

  for _, asset := range flattened {
    if ! asset.IsSingle() {
      t.Errorf("Asset is not singular: %s", asset.Url)
      all_assets_singular = false
    }
    if asset.IsMulti() {
      t.Errorf("Asset is pluralistic: %s", asset.Url)
      all_assets_singular = false
    }
  }

  if all_assets_singular == false {
    t.Error("Not all assets are singular")
  }

  if got, expect := len(flattened), 63; got != expect {
    t.Errorf("Flattening assets returned %d assets, got %d", expect, got)
  }
}


func TestSpecPathExists (t *testing.T) {
  var err error
  var source_dir string = t.TempDir()

  root := NewSpec("root", nil)
  root.Props["source_dir"] = source_dir

  // Make the file
  //
  var file_path = filepath.Join(source_dir, "exists.txt")
  err = os.WriteFile(file_path, []byte("Test file!"), 0o660)
  if err != nil { t.Fatal(err) }

  var exists bool

  // Test case where the path does exist
  //
  exists, err = root.PathExists("exists.txt")
  if err != nil {
    t.Fatal(err)
  }
  if exists == false {
    t.Fatal("PathExists returns that the created file doesn't exist")
  }

  // Test case where the path does not exist
  //
  exists, err = root.PathExists("doesnt-exist.txt")
  if err != nil {
    t.Fatal(err)
  }
  if exists == true {
    t.Fatal("PathExists returns that non-existing file doesn't exist")
  }
}


func TestSpecMakeFileKeyAssetValidFile (t *testing.T) {
  var source_dir string = t.TempDir()

  root := NewSpec("root", nil)
  root.Props["source_dir"] = source_dir

  // Make the file and Asset
  //
  var file_path = filepath.Join(source_dir, "file.txt")
  os.WriteFile(file_path, []byte("Test file!"), 0o660)
  
  asset, err := root.MakeFileKeyAsset("file.txt", "@emit", "file.txt")

  // Basic Asset data assertions
  //
  if err != nil {
    t.Fatal(err)
  }

  if asset == nil {
    t.Fatal("Asset is nil")
  }
  
  if url := asset.Url.String(); url != "ib://root/@emit/file.txt" {
    t.Fatalf("Asset Url is %s, expected ib://root/@emit/file.txt", url)
  }

  if !asset.IsSingle() || asset.IsMulti() {
    t.Fatal("Asset is not singular, or is pluralistic")
  }

  // Test reading the asset file and assert its content
  //
  reader, err := asset.ContentBytesGetReader()
  if err != nil {
    t.Fatal(err)
  }

  if bytes, err := io.ReadAll(reader); err != nil {
    t.Fatal(err)
  } else {
    if string(bytes) != "Test file!" {
      t.Fatalf("Expected file contents: \"Test file!\", got \"%s\"", bytes)
    }
  }

  if reader, ok := reader.(io.Closer); ok {
    reader.Close()
  } else {
    t.Fatal("Reader is not a closer")
  }

  // Test writing the asset file
  //
  writer, err := asset.ContentBytesGetWriter() 
  if err != nil {
    t.Fatal(err)
  }

  _, err = writer.Write([]byte("Test file updated"))
  if err != nil {
    t.Fatal(err)
  }

  if writer, ok := writer.(io.Closer); ok {
    writer.Close()
  } else {
    t.Fatal("Writer is not a closer")
  }

  // Assert the newly-updated asset file content
  //
  writen_bytes, err := os.ReadFile(file_path)
  if err != nil {
    t.Fatal(err)
  }

  if writen := string(writen_bytes); writen != "Test file updated" {
    t.Fatalf("File contents were \"%s\", expected \"Test file updated\"", writen)
  }
}


func TestSpecMakeFileKeyAssetValidDirectory (t *testing.T) {
  var source_dir string = t.TempDir()

  root := NewSpec("root", nil)
  root.Props["source_dir"] = source_dir

  // Make files and the directory asset
  //
  for dir_i := range 3 {
    for file_i := range 3 {
      var file_path = filepath.Join(source_dir, fmt.Sprintf("%d", dir_i), fmt.Sprintf("%d.txt", file_i))
      os.WriteFile(file_path, []byte("Test file!"), 0o660)
    }
  }
  
  asset, err := root.MakeFileKeyAsset("/", "@emit")

  // Basic Asset data assertions
  //
  if err != nil {
    t.Fatal(err)
  }

  if asset == nil {
    t.Fatal("Asset is nil")
  }
  
  if url := asset.Url.String(); url != "ib://root/@emit" {
    t.Fatalf("Asset Url is %s, expected ib://root/@emit", url)
  }

  if asset.IsSingle() || !asset.IsMulti() {
    t.Fatal("Asset is not pluralistic, or is singular")
  }

  // TODO: test reading the file and assert its content
}


func TestSpecAnnexAsset (t *testing.T) {
  var spec_a  *Spec  = NewSpec("a", nil)
  var spec_b  *Spec  = NewSpec("b", nil)
  var asset   *Asset = spec_a.MakeAsset("asset.txt")

  var asset_original_url = asset.Url.String()

  var annexed *Asset = spec_b.AnnexAsset(asset)

  // Assert the annexed asset and the original are different pointers
  //
  if asset == annexed {
    t.Fatalf("Annexed asset is the same as the original")
  }

  // Assert the annexed URL was updated
  //
  if got, expected := annexed.Url.Host, spec_b.Url.Host; got != expected {
    t.Errorf("Annexed asset hostname is not %s, got %s", expected, got)
  }

  // Assert the original asset URL is unmodified
  //
  if got := asset.Url.String(); got != asset_original_url {
    t.Errorf("Original asset URL is %s, expected %s", got, asset_original_url)
  }

  // Assert there is new history
  //
  if got := annexed.History; got == nil || got == asset.History {
    t.Errorf("Annexed asset does not have new history")
  }
}


func TestAssetGetSetContentBytes (t *testing.T) {
  var asset = & Asset {
    TypeMask: ASSET_QUANTITY_SINGLE,
  }

  // Read empty asset
  //
  if bytes, err := asset.GetContentBytes(); err != nil {
    t.Error(err)
  } else if bytes != nil {
    t.Errorf("Expected bytes to be nil, got \"%s\"", string(bytes))
  }

  // Write asset content
  //
  if err := asset.SetContentBytes([]byte("value")); err != nil {
    t.Fatal(err)
  }

  if asset.ContentModified != true {
    t.Errorf("Asset Modified flag not true")
  }

  // Reread and assert asset content
  //
  if bytes, err := asset.GetContentBytes(); err != nil {
    t.Error(err)
  } else if got, expected := string(bytes), "value"; got != expected {
    t.Errorf("Expected content to have a value of \"%s\", got \"%s\"", expected, got)
  }
}


func TestFileAssetGetContentBytes (t *testing.T) {
  var err        error
  var source_dir string = t.TempDir()
  var spec       *Spec  = NewSpec("spec", nil)
  spec.Props["source_dir"] = source_dir

  var file_path  string = filepath.Join(source_dir, "file.txt")

  err = os.WriteFile(file_path, []byte("This is a text file!"), 0o660)
  if err != nil {
    t.Fatal(err)
  }

  var asset *Asset
  var content []byte

  asset, err = spec.MakeFileKeyAsset("file.txt")
  
  if content, err = asset.GetContentBytes(); err != nil {
    t.Fatal(err)
  }

  if got, expected := string(content), "This is a text file!"; got != expected {
    t.Fatalf("File content expected to be %s, got %s", expected, got)
  }
}


func TestAssetContentData (t *testing.T) {
  var buffer = []byte("Unmodified content")

  var asset = & Asset {}

  // This asset has no content data access functions. Assert that
  // attempts to get the data, prior to it being set, result in
  // errors.
  //
  if data, err := asset.GetContentData(); err == nil {
    t.Error("Asset.GetContentData() did not error")
  } else if data != nil {
    t.Error("Asset.GetContentData() errored, but also returned data")
  }

  if dataWriteFunc, err := asset.GetContentDataWriteFunc(); err == nil {
    t.Error("Asset.GetContentDataWriteFunc() did not error")
  } else if dataWriteFunc != nil {
    t.Error("Asset.GetContentDataWriteFunc() errored, but also returned a function")
  }

  if dataWriteFunc, err := asset.GetContentDataWriteFunc(); err == nil {
    t.Error("Asset.GetContentDataWriteFunc() did not error")
  } else if dataWriteFunc != nil {
    t.Error("Asset.GetContentDataWriteFunc() errored, but also returned a function")
  }

  // After confirming invalid access, define readers and writers
  // (a means for valid access)
  //
  asset.SetContentBytesGetReaderFunc(func (*Asset) (io.Reader, error) {
    return bytes.NewReader(buffer), nil
  })

  asset.SetContentDataReadFunc(func (a *Asset, r io.Reader) (any, error) {
    content, err := io.ReadAll(r)
    if err != nil { return "", err }
    string_content := string(content)
    return &string_content, nil
  })

  asset.SetContentDataWriteFunc(func (a *Asset, w io.Writer, data_any any) (int, error) {
    data, ok := data_any.(*string)
    if ok == false {
      return 0, fmt.Errorf("Cannot write data, expected string, got %T", data_any)
    }

    if w == nil {
      return 0, fmt.Errorf("Cannot write data, writer is nil")
    }

    return w.Write([]byte(*data))
  })

  // Get and update the asset's internal content data
  //
  data_any, err := asset.GetContentData()
  if err != nil {
    t.Fatal(err)
  } else if data, ok := data_any.(*string); !ok {
    t.Fatalf("Data is not a string, got %T", data_any)
  } else if expect, got := "Unmodified content", *data; expect != got {
    t.Fatalf("Content data is %s, got %s", got, expect)
  }

  // Assert that the content data is equal upon setting it and
  // getting it again (verifying that the cache is being used)
  //
  var new_content_value  string = "MODIFIED CONTENT"
  var new_content       *string = &new_content_value
  if err := asset.SetContentData(new_content); err != nil {
    t.Fatal(err)
  }

  if data_again_any, err := asset.GetContentData(); err != nil {
    t.Error(err)
  } else if data_again, ok := data_again_any.(*string); !ok {
    t.Errorf("Got data again, but got a %T, expected *string", data_again_any)
  } else if data_again != new_content {
    t.Errorf("Upon getting content data again, the new data is not an equal pointer value, indicating that cached data is not being used.")
  }

  // Update content bytes and assert their value
  //
  writer := bytes.Buffer {}
  if _, err := asset.WriteContentDataTo(&writer); err != nil {
    t.Fatal(err)
  }

  if err := asset.SetContentBytes(writer.Bytes()); err != nil {
    t.Fatal(err)
  }

  if bytes_data, err := asset.GetContentBytes(); err != nil {
    t.Fatal(err)
  } else if got, expect := string(bytes_data), "MODIFIED CONTENT"; got != expect {
    t.Fatalf("Expected asset content bytes to be \"%s\", got \"%s\"", expect, got)
  }

  // Assert clearing the content cache before further tests
  //
  asset.ClearContentDataCache()
  if got := asset.ContentData; got != nil {
    t.Fatalf("Asset content data cache cleared, but content data is not nil, got \"%s\"", got)
  } else if asset.ContentDataModified != false {
    t.Fatalf("Asset content data cache was cleared, but still is considered modified (asset.ContentDataModified is true)")
  }

  // Mutate the content bytes, and reload the content data. The
  // content data should reflect this mutation instead of the
  // initial data.
  //
  if err != asset.SetContentBytes([]byte("MODIFIED AGAIN")) {
    t.Fatal(err)
  }

  if content_data_any, err := asset.GetContentData(); err != nil {
    t.Fatal(err)
  } else if content_data, ok := content_data_any.(*string); !ok {
    t.Fatalf("Expected content data to be a *string, got %T", content_data_any)
  } else if got, expect := *content_data, "MODIFIED AGAIN"; got != expect {
    t.Fatalf("Expected asset content data to be \"%s\", got \"%s\"", expect, got)
  }
}
