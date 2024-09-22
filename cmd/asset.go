package main

import (
  . "gilchrist.tech/interbuilder"
  "encoding/json"
  "encoding/base64"
  "strings"
)


type AssetEncodingContent struct {
}

type AssetEncoding struct {
  Url      string `json:"url"`
  Mimetype string `json:"mimetype,omitempty"`

  Content struct {
    String string `json:"string,omitempty"`
    Base64 string `json:"base64,omitempty"`
    // Bytes  []byte `json:"bytes,omitempty"`
  } `json:"content"`
}


func AssetJsonUnmarshal (data []byte) (*Asset, error) {
  var json_data AssetEncoding
  if err := json.Unmarshal(data, &json_data); err != nil {
    return nil, err
  }
  return nil, nil
}


func AssetJsonMarshal (a *Asset) ([]byte, error) {
  var marshal_data AssetEncoding

  marshal_data.Url = a.Url.String()

  var is_text = false

  if a.Mimetype != "" {
    marshal_data.Mimetype = a.Mimetype
    if strings.HasPrefix(a.Mimetype, "text") {
      is_text = true
    }
  }

  content, err := a.GetContentBytes()
  if err != nil {
    return nil, err
  }

  if is_text {
    marshal_data.Content.String = string(content)
  } else {
    marshal_data.Content.Base64 = base64.StdEncoding.EncodeToString(content)
  }

  return json.Marshal(&marshal_data)
}
