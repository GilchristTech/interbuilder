package main

import (
  . "gilchrist.tech/interbuilder"
  "github.com/spf13/cobra"
  "encoding/json"
  "encoding/base64"
  "net/url"
  "strings"
  "bufio"
  "fmt"; "io"; "os"
)


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


var cmd_assets = & cobra.Command {
  Use: "assets",
  Short: "Operate on Interbuilder assets and run simple ETL operations",
  Args: cobra.ExactArgs(0),
  Run: func (cmd *cobra.Command, args []string) {
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
    var transform = root.AddSubspec(NewSpec("cli-transform", nil))

    // WRITE/LOAD
    //
    for output_i, output_dest := range Flag_outputs {
      var spec_name   string = fmt.Sprintf("cli-output-%d", output_i)
      var output_spec  *Spec = root.AddSubspec(NewSpec(spec_name, nil))

      var writer io.Writer
      var closer io.Closer
      var err    error

      writer, closer, err = outputStringToWriter(output_dest)

      if err != nil {
        fmt.Println("Error opening output %d:\n%v\n", output_i, err)
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
        asset_json, err := AssetJsonMarshal(a)
        if err != nil {
          return nil, err
        }
        writer.Write(asset_json)
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
      fmt.Println("Error while running root spec:\n%v", err)
      os.Exit(1)
    }
  },
}
