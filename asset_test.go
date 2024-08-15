package interbuilder

import (
  "testing"
  "net/url"
  "strconv"
  "path/filepath"
  "os"
  "fmt"
  "io"
)


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
  wrapTimeout(t, func () {
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


func TestSpecPathExists (t *testing.T) {
  var source_dir string = t.TempDir()

  root := NewSpec("root", nil)
  root.Props["source_dir"] = source_dir

  // Make the file
  //
  var file_path = filepath.Join(source_dir, "exists.txt")
  os.WriteFile(file_path, []byte("Test file!"), 0o660)

  var exists bool
  var err    error

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
  reader, err := asset.GetReader()
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
  writer, err := asset.GetWriter() 
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
