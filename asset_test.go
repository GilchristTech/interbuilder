package interbuilder

import (
  "testing"
  "net/url"
  "strconv"
)


func TestAssetExpandSingular (t *testing.T) {
  var testing_assets = []*Asset {
    & Asset {},
    & Asset {
      TypeMask: ASSET_TYPE_SINGULAR,
    },
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
  var test_url, _ = url.Parse("ib://testing")

  var testing_assets = make([]*Asset, 0)

  for type_mask := 0 ; type_mask <= ASSET_FIELDS_PLURAL ; type_mask++ {
    // Only generate test multiassets through combinations of valid
    // asset types which also include ASSET_TYPE_ARRAY
    //
    if type_mask & ASSET_FIELDS_SINGULAR != 0 {
      continue
    }

    if type_mask & ASSET_TYPE_ARRAY == 0 {
      continue
    }

    var base_url = test_url.JoinPath(strconv.Itoa(type_mask))
    var asset_array = make([]*Asset, 3)

    for i := 0 ; i < 3 ; i++ {
      asset_array[i] = & Asset {
        Url: base_url.JoinPath(strconv.Itoa(i)),
      }
    }

    testing_assets = append(testing_assets, & Asset {
      Url:         base_url,
      TypeMask:    type_mask,
      asset_array: asset_array,
    })
  }
  

  for i, asset := range testing_assets {
    assets, err := asset.Expand()
    if err != nil {
      t.Fatalf("Error in asset #%d: %s", i, err)
    }

    if len(assets) != 3 {
      t.Fatalf("Expanded assets array in asset #%d does not have a length of 3", i)
    }
  }
}


func TestAssetExpandArrayFunc (t *testing.T) {
  var test_url, _    = url.Parse("ib://testing")
  var type_mask int  = ASSET_TYPE_ARRAY_FUNC
  var base_url       = test_url.JoinPath(strconv.Itoa(type_mask))

  var test_asset = & Asset {
    Url:         base_url,
    TypeMask:    type_mask,
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
    var test_url, _    = url.Parse("ib://testing")
    var type_mask int  = ASSET_TYPE_GENERATOR
    var base_url       = test_url.JoinPath(strconv.Itoa(type_mask))

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
