package main

import (
  . "gilchrist.tech/interbuilder"
  "github.com/spf13/cobra"
  "encoding/json"
  "encoding/base64"
  "net/url"
  "strings"
  "bufio"
  "bytes"
  "fmt"; "io"; "os"
)


var (
  ASSET_ENCODING_FIELDS            uint64 = 0b11_111_111
  ASSET_ENCODING_FIELDS_PROPERTIES uint64 = 0b00_000_111
  ASSET_ENCODING_FIELDS_CONTENT    uint64 = 0b00_111_000
  ASSET_ENCODING_FIELDS_FORMAT     uint64 = 0b11_000_000

  ASSET_ENCODING_JSON              uint64 = 0b01_000_000
  ASSET_ENCODING_TEXT              uint64 = 0b10_000_000

  ASSET_ENCODING_URL               uint64 = 0b00_000_001
  ASSET_ENCODING_MIMETYPE          uint64 = 0b00_000_010
  ASSET_ENCODING_FORMAT            uint64 = 0b00_000_100

  ASSET_ENCODING_CONTENT_STRING    uint64 = 0b00_001_000
  ASSET_ENCODING_CONTENT_BASE64    uint64 = 0b00_010_000
  ASSET_ENCODING_CONTENT_LENGTH    uint64 = 0b00_100_000
)


var ASSET_ENCODING_DEFAULT    = (
  ASSET_ENCODING_JSON           |
  ASSET_ENCODING_URL            |
  ASSET_ENCODING_CONTENT_STRING |
  ASSET_ENCODING_CONTENT_BASE64 |
  ASSET_ENCODING_MIMETYPE       )


type AssetEncoding struct {
  Url      string `json:"url"`
  Mimetype string `json:"mimetype,omitempty"`

  Content *AssetEncodingContent `json:"content,omitempty"`
}

type AssetEncodingContent struct {
  Length    int `json:"length,omitempty"`
  String string `json:"string,omitempty"`
  Base64 string `json:"base64,omitempty"`
}


func AssetJsonUnmarshal (data []byte) (*Asset, error) {
  var json_data AssetEncoding
  if err := json.Unmarshal(data, &json_data); err != nil {
    return nil, err
  }

  var asset = & Asset {}

  // Parse URL
  //
  if json_data.Url == "" {
    return nil, fmt.Errorf("Cannot parse asset from JSON object, `url` property is falsey")

  } else if asset_url, err := url.Parse(json_data.Url); err != nil {
    return nil, fmt.Errorf("Error parsing `url` property from JSON object: %w", err)

  } else {
    asset.Url = asset_url
  }

  // Copy Mimetype
  //
  asset.Mimetype = json_data.Mimetype

  // Decode content
  //
  if json_data.Content.String != "" {
    if err := asset.SetContentBytes([]byte(json_data.Content.String)); err != nil {
      return nil, fmt.Errorf("Error setting asset content from content.string: %w", err)
    }

  } else if content_base64 := json_data.Content.Base64; content_base64 != "" {
    content_bytes, err := base64.StdEncoding.DecodeString(content_base64)
    if err != nil {
      return nil, fmt.Errorf("Error decoding content.base64 from JSON object: %w", err)
    }

    if err := asset.SetContentBytes(content_bytes); err != nil {
      return nil, fmt.Errorf("Error setting asset content from content.base64: %w", err)
    }

  }

  return asset, nil
}


func AssetMarshal (a *Asset, encoding_mask uint64) ([]byte, error) {
  // Get the type of asset and use the appropriate marshal function
  //
  var asset_encoding_format = encoding_mask & ASSET_ENCODING_FIELDS_FORMAT
  
  switch asset_encoding_format {
    case 0:
      return nil, fmt.Errorf("Encoding format is undefined")
    case ASSET_ENCODING_JSON:
      return AssetJsonMarshal(a, encoding_mask)
    case ASSET_ENCODING_TEXT:
      return AssetTextMarshal(a, encoding_mask)
  }

  return nil, fmt.Errorf("Unrecognized format in asset encoding mask with value 0o%o", encoding_mask)
}


func AssetJsonMarshal (a *Asset, encoding_mask uint64) ([]byte, error) {
  if encoding_mask == 0 {
    encoding_mask  = ASSET_ENCODING_DEFAULT & ^ASSET_ENCODING_FIELDS_FORMAT
    encoding_mask |= ASSET_ENCODING_JSON
  }

  var encode_json           = encoding_mask & ASSET_ENCODING_JSON           != 0
  var encode_url            = encoding_mask & ASSET_ENCODING_URL            != 0
  var encode_mimetype       = encoding_mask & ASSET_ENCODING_MIMETYPE       != 0
  var encode_content        = encoding_mask & ASSET_ENCODING_FIELDS_CONTENT != 0
  var encode_content_string = encoding_mask & ASSET_ENCODING_CONTENT_STRING != 0
  var encode_content_base64 = encoding_mask & ASSET_ENCODING_CONTENT_BASE64 != 0
  var encode_content_length = encoding_mask & ASSET_ENCODING_CONTENT_LENGTH != 0

  if encode_json == false {
    return nil, fmt.Errorf("Asset encoding is not JSON")
  }

  var marshal_data AssetEncoding

  if encode_url {
    marshal_data.Url = a.Url.String()
  }

  var is_text = false

  if encode_mimetype {
    if a.Mimetype != "" {
      marshal_data.Mimetype = a.Mimetype
      if strings.HasPrefix(a.Mimetype, "text") {
        is_text = true
      }
    }
  }

  if encode_content {
    marshal_data.Content = & AssetEncodingContent {}

    content, err := a.GetContentBytes()
    
    if encode_content_length {
      marshal_data.Content.Length = len(content)
    }

    if err != nil {
      return nil, err
    }

    var use_base64 = false
    var use_string = false

    if encode_content_string && encode_content_base64 {
      use_string =  is_text
      use_base64 = !is_text
    } else if encode_content_string {
      use_string = true
    } else if encode_content_base64 {
      use_base64 = true
    }

    if use_string {
      marshal_data.Content.String = string(content)
    } else if use_base64 {
      marshal_data.Content.Base64 = base64.StdEncoding.EncodeToString(content)
    }
  }

  return json.Marshal(&marshal_data)
}


func AssetTextMarshal (a *Asset, encoding_mask uint64) ([]byte, error) {
  if encoding_mask == 0 {
    encoding_mask  = ASSET_ENCODING_DEFAULT & ^ASSET_ENCODING_FIELDS_FORMAT
    encoding_mask |= ASSET_ENCODING_TEXT
  }

  var encode_text           = encoding_mask & ASSET_ENCODING_TEXT           != 0
  var encode_url            = encoding_mask & ASSET_ENCODING_URL            != 0
  var encode_mimetype       = encoding_mask & ASSET_ENCODING_MIMETYPE       != 0
  var encode_content        = encoding_mask & ASSET_ENCODING_FIELDS_CONTENT != 0
  var encode_content_string = encoding_mask & ASSET_ENCODING_CONTENT_STRING != 0
  var encode_content_base64 = encoding_mask & ASSET_ENCODING_CONTENT_BASE64 != 0
  var encode_content_length = encoding_mask & ASSET_ENCODING_CONTENT_LENGTH != 0

  if encode_text == false {
    return nil, fmt.Errorf("Asset encoding is not text")
  }

  var encoded = bytes.NewBuffer([]byte{})
  var writen_field = false

  if encode_url {
    if writen_field { encoded.WriteString("\t") }; writen_field = true
    encoded.WriteString(a.Url.String())
  }

  var is_text = false

  if encode_mimetype {
    if writen_field { encoded.WriteString("\t") }; writen_field = true
    if a.Mimetype != "" {
      encoded.WriteString(a.Mimetype)

      if strings.HasPrefix(a.Mimetype, "text") {
        is_text = true
      }
    }
  }

  if encode_content {
    content, err := a.GetContentBytes()
    if writen_field { encoded.WriteString("\t") }; writen_field = true
    
    if encode_content_length {
      encoded.WriteString(string(len(content)))
    }

    if err != nil {
      return nil, err
    }

    var use_base64 = false
    var use_string = false

    if encode_content_string && encode_content_base64 {
      use_string =  is_text
      use_base64 = !is_text
    } else if encode_content_string {
      use_string = true
    } else if encode_content_base64 {
      use_base64 = true
    }

    if use_string {
      encoded.Write(content)
    } else if use_base64 {
      content := base64.StdEncoding.EncodeToString(content)
      encoded.WriteString(content)
    }
  }

  return encoded.Bytes(), nil
}


var cmd_assets = & cobra.Command {
  Use: "assets",
  Short: "Operate on Interbuilder assets and run simple ETL operations",
  Run: func (cmd *cobra.Command, args []string) {
    // Parse outputs
    //
    var output_definitions []cliOutputDefinition
    var err error

    // Parse output positional arguments
    if output_definitions, err = parseOutputArgs(args); err != nil {
      fmt.Printf("Error parsing output arguments:\n\t%v\n", err)
      os.Exit(1)
    }

    // Parse flag outputs (--output and -o)
    if flag_outputs, err := parseOutputArgs(Flag_outputs); err != nil {
      fmt.Printf("Error parsing output flags:\n\t%v\n", err)
      os.Exit(1)
    } else if len(flag_outputs) > 0 {
      output_definitions = append(output_definitions, flag_outputs...)
    }

    // Check if we are reading from a pipe
    //
    var read_stdin = false
    if stdin_stat, err := os.Stdin.Stat(); err != nil {
      fmt.Println(err)
      os.Exit(1)
    } else {
      read_stdin = (stdin_stat.Mode() & os.ModeCharDevice) == 0
    }

    // Input is needed to build a pipeline. Error if no
    // inputs are found.
    //
    if len(Flag_inputs) == 0 && !read_stdin {
      fmt.Println("Error: no inputs are defined")
      cmd.Help()
      os.Exit(1)
    }

    // Piping should imply a STDIN input flag, if one has not
    // been defined.
    //
    if read_stdin {
      for _, input := range Flag_inputs {
        if input == "-" {
          goto EXIT_IMPLY_STDIN_INPUT
        }
      }
      // No STDIN input was explicitly defined, add it
      Flag_inputs = append(Flag_inputs, "-")

      EXIT_IMPLY_STDIN_INPUT:
    }

    // Set up a root spec
    //
    var root = NewSpec("root", nil)

    if Flag_print_spec {
      defer PrintSpec(root)
    }

    // TRANSFORM
    //
    // Currently, this Spec for transformations simply acts as a
    // central point for collecting assets from inputs and
    // distributing them to outputs, but defining transformation
    // tasks from the CLI is intended.
    //
    var transform = root.AddSubspec(NewSpec("cli-transform", nil))

    // WRITE/LOAD
    //
    for output_i, output_definition := range output_definitions {
      var spec_name   string = fmt.Sprintf("cli-output-%d", output_i)
      var output_spec  *Spec = root.AddSubspec(NewSpec(spec_name, nil))

      var writer io.Writer
      var closer io.Closer
      var err    error

      writer, closer, err = outputStringToWriter(output_definition.Dest)

      if err != nil {
        fmt.Printf("Error opening output %d:\n%v\n", output_i, err)
        os.Exit(1)
      }

      output_spec.EnqueueTaskFunc("consume", func (s *Spec, tk *Task) error {
        for { select {
        case <-tk.CancelChan:
          return nil
        case asset_chunk, ok := <- s.Input:
          if !ok {
            return nil
          }
          tk.EmitAsset(asset_chunk)
        }}
      })

      output_spec.EnqueueTaskMapFunc(spec_name, func (a *Asset) (*Asset, error) {
        asset_encoded, err := AssetMarshal(a, output_definition.Encoding)
        if err != nil {
          return nil, err
        }
        writer.Write(asset_encoded)
        writer.Write([]byte("\n"))
        return a, nil
      })

      if closer != nil {
        close_task := output_spec.DeferTaskFunc(spec_name +"-close", func (s *Spec, tk *Task) error {
          return closer.Close()
        })
        close_task.IgnoreAssets = true
      }

      transform.AddOutputSpec(output_spec)
    }

    // READ/EXTRACT
    //
    for input_i, input_src := range Flag_inputs {
      var spec_name  string = fmt.Sprintf("cli-input-%d", input_i)
      var input_spec  *Spec = transform.AddSubspec(NewSpec(spec_name, nil))

      var reader io.Reader
      var closer io.Closer
      var err    error

      reader, closer, err = inputStringToReader(input_src)

      if err != nil {
        fmt.Printf("Error reading input %d:\n%v\n", input_i, err)
        os.Exit(1)
      }

      input_spec.EnqueueTaskFunc(spec_name + "-read-assets", func (s *Spec, tk *Task) error {
        var line_scanner = bufio.NewScanner(reader)
        var read_buffer  = make([]byte, 0, 64*1024)
        line_scanner.Buffer(read_buffer, 1024*1024*1024)

        for line_scanner.Scan() {
          bytes := line_scanner.Bytes()
          if asset, err := AssetJsonUnmarshal(bytes); err != nil {
            return fmt.Errorf("Error parsing asset in input %s (input #%d): %w", input_src, input_i, err)
          } else {
            new_asset := s.AnnexAsset(asset)
            tk.EmitAsset(new_asset)
          }
        }

        if err := line_scanner.Err(); err != nil {
          return fmt.Errorf("Error while reading input %s (input #%d): %w", input_src, input_i, err)
        }

        return nil
      })

      if closer != nil {
        close_task := input_spec.DeferTaskFunc(spec_name + "-read-assets-close", func (*Spec, *Task) error {
          return closer.Close()
        })
        close_task.IgnoreAssets = true
      }
    }

    if err := root.Run(); err != nil {
      fmt.Printf("Error while running root spec:\n%v", err)
      os.Exit(1)
    }
  },
}
