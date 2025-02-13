package main

import (
  . "gilchrist.tech/interbuilder"
  "github.com/spf13/cobra"
  "os"
  "io"
  "strings"
  "fmt"
  "regexp"
)


var Flag_print_spec    bool
var Flag_outputs       []string
var Flag_inputs        []string


func init () {
  cmd_root.AddCommand(cmd_run)
  cmd_root.AddCommand(cmd_assets)

  cmdAddSpecRunFlags(cmd_run)
  cmdAddSpecRunFlags(cmd_assets)

  cmdAddAssetIOFlags(cmd_run)
  cmdAddAssetIOFlags(cmd_assets)
}


func cmdAddSpecRunFlags (cmd *cobra.Command) {
  cmd.PersistentFlags().BoolVar(
    &Flag_print_spec, "print-spec", false,
    "Print the build specification tree when execution is finished",
  )
}


func cmdAddAssetIOFlags (cmd *cobra.Command) {
  cmd.Flags().StringArrayVarP(
    &Flag_outputs, "output", "o", []string{},
    "Specify an asset output",
  )

  cmd.Flags().StringArrayVarP(
    &Flag_inputs, "input", "i", []string{},
    "Specify an asset input",
  )
}


func outputStringToWriter (output_str string) (io.Writer, io.Closer, error) {
  if output_str == "-" {
    return os.Stdout, nil, nil
  } else {
    writer, err := os.Create(output_str)
    if err != nil {
      return nil, nil, err
    }
    return writer, writer, nil
  }
}


func inputStringToReader (input_str string) (io.Reader, io.Closer, error) {
  if input_str == "-" {
    return os.Stdin, nil, nil
  } else {
    reader, err := os.Open(input_str)
    if err != nil {
      return nil, nil, err
    }
    return reader, reader, nil
  }
}


type cliOutputDefinition struct {
  Dest      string
  Encoding  uint64
  Filters   []cliFilterDefinition
}


type cliFilterDefinition struct {
  Invert    bool
  Mimetype  string
  Prefix    string
  Suffix    string
}


func (od *cliOutputDefinition) EnqueueTasks (name string, spec *Spec) (error) {
  var writer io.Writer
  var closer io.Closer
  var err    error

  writer, closer, err = outputStringToWriter(od.Dest)

  if err != nil {
    return fmt.Errorf("Error opening output: %w", err)
  }

  // Enqueue a task to consume spec input and forward assets
  //
  err = spec.EnqueueTaskFunc(name+"-consume", func (s *Spec, tk *Task) error {
    if err := tk.ForwardAssets(); err != nil {
      return err
    }

    for {
      if asset_chunk, err := tk.AwaitInputAssetNext(); err != nil {
        return err

      } else if asset_chunk == nil {
        return nil

      } else if err := tk.EmitAsset(asset_chunk); err != nil {
        return err
      }
    }
  })

  if err != nil { return err }

  // Enqueue filter tasks
  //
  for filter_i, filter_definition := range od.Filters {
    var filter_name = fmt.Sprintf("%s-filter-%d", name, filter_i)
    filter_definition.EnqueueTask(filter_name, spec)
  }

  // Enqueue write task
  //
  spec.EnqueueTaskMapFunc(name, func (a *Asset) (*Asset, error) {
    asset_encoded, err := AssetMarshal(a, od.Encoding)
    if err != nil {
      return nil, err
    }
    writer.Write(asset_encoded)
    writer.Write([]byte("\n"))
    return a, nil
  })

  // Defer a Task to close the file, if applicable
  //
  if closer != nil {
    if closer == os.Stdout {
      goto DONT_CLOSE
    }

    var close_task = & Task {
      Name: name+"-close",
      IgnoreAssets: true,
      Func: func (*Spec, *Task) error {
        closer.Close()
        return nil
      },
    }

    if err := spec.DeferTask(close_task); err != nil {
      return err
    }
  }
  DONT_CLOSE:

  return nil
}


func (od *cliOutputDefinition) MakeSpec (spec_name string) (*Spec, error) {
  var output_spec = NewSpec(spec_name, nil)
  if err := od.EnqueueTasks(spec_name, output_spec); err != nil {
    return nil, err
  }
  return output_spec, nil
}


func (fd *cliFilterDefinition) EnqueueTask (name string, spec *Spec) {
  var prefix = strings.TrimPrefix(fd.Prefix, "/")

  spec.EnqueueTaskMapFunc(name, func (asset *Asset) (*Asset, error) {
    var path string

    if fd.Mimetype != "" {
      if strings.HasPrefix(asset.Mimetype, fd.Mimetype) == fd.Invert {
        return nil, nil
      }
    }

    path = strings.TrimLeft(asset.Url.Path, "/")
    path = strings.TrimPrefix(path, "@emit")
    path = strings.TrimLeft(path, "/")

    if fd.Suffix != "" {
      if strings.HasSuffix(path, fd.Suffix) == fd.Invert {
        return nil, nil
      }
    }

    if fd.Prefix != "" {
      if strings.HasPrefix(path, prefix) == fd.Invert {
        return nil, nil
      }
    }

    return asset, nil
  })
}


func (od *cliOutputDefinition) SetEncodingField (field_name string) error {
  if field_name == "default" {
    // Add all default fields to the encoding, but because the
    // format fields are meant to only have one positive bit,
    // zero-out that range of bits before doing a disjunction of
    // the default fields and the currently-enabled ones.
    //
    od.Encoding &= ^ASSET_ENCODING_FIELDS_FORMAT
    od.Encoding |=  ASSET_ENCODING_DEFAULT
    return nil
  }

  if field_name == "content" {
    od.Encoding |= ASSET_ENCODING_CONTENT_STRING
    od.Encoding |= ASSET_ENCODING_CONTENT_BASE64
    return nil
  } else if field_name == "no-content" {
    od.Encoding &= ^ASSET_ENCODING_FIELDS_CONTENT
    return nil
  }

  var field_values uint64 = ASSET_ENCODING_FIELDS
  var field_domain uint64 = 0

  // By default, set the field to true, unless it starts with "no-"
  if strings.HasPrefix(field_name, "no-") {
    field_values = 0
    field_name = field_name[3:]
  }


  switch field_name {
    default:
      return fmt.Errorf("Field not recognized: %s", field_name)

    /* Content format fields */
    case "json":     field_values &= ASSET_ENCODING_JSON
                     field_domain  = ASSET_ENCODING_FIELDS_FORMAT

    case "text":     field_values &= ASSET_ENCODING_TEXT
                     field_domain  = ASSET_ENCODING_FIELDS_FORMAT

    /* Property fields */
    case "url":      field_values &= ASSET_ENCODING_URL
                     field_domain  = ASSET_ENCODING_URL

    case "mimetype": field_values &= ASSET_ENCODING_MIMETYPE
                     field_domain  = ASSET_ENCODING_MIMETYPE

    case "format":   field_values &= ASSET_ENCODING_FORMAT
                     field_domain  = ASSET_ENCODING_FORMAT

    /* Content fields */
    case "string":   field_values &= ASSET_ENCODING_CONTENT_STRING
                     field_domain  = ASSET_ENCODING_CONTENT_STRING

    case "base64":   field_values &= ASSET_ENCODING_CONTENT_BASE64
                     field_domain  = ASSET_ENCODING_CONTENT_BASE64

    case "length":   field_values &= ASSET_ENCODING_CONTENT_LENGTH
                     field_domain  = ASSET_ENCODING_CONTENT_LENGTH
  }

  od.Encoding = (od.Encoding & ^field_domain) | field_values
  return nil
}


func parseOutputArgs (args []string) ([]cliOutputDefinition, error) {
  // Outputs definitions are built in-place within this array,
  // and the last element, an incomplete definition, is truncated
  // from what is returned.
  //
  var outputs = []cliOutputDefinition {
    cliOutputDefinition { },
  }
  var output_definition = &outputs[0]

  var expect_definition = false

  for arg_i, arg := range args {

    // Figure out what the argument is.
    // Only one of these conditions should be true.

    var rgx_match_section = regexp.MustCompile(`^\s*(\w+)\s*:`)
    var section_match     = rgx_match_section.FindStringSubmatch(arg)

    var section string = "output"

    if section_match != nil {
      switch matched_section := section_match[1]; matched_section {
      case "format", "filter":
        section = matched_section
      default:
        return nil, fmt.Errorf(`Error parsing argument, unknown section "%w"`, matched_section)
      }
    }

    var is_format      bool = section == "format"
    var is_filter      bool = section == "filter"
    var is_destination bool = !is_format && !is_filter

    var section_node *ExpressionNode = nil

    // If this argument is an output expression, parse it
    //
    if is_format || is_filter {
      if nodes, err := ParseExpressionString(arg, false); err != nil {
        return nil, fmt.Errorf("Error parsing expression in argument %d: %w", arg_i+1, err)

      } else if expect, got := 1, len(nodes); expect != got {
        return nil, fmt.Errorf("Argument %d contains %d sections, expected %d", arg_i, got, expect)

      } else {
        var node = nodes[0]

        if got, expect := node.NodeType, EXPRESSION_NODE_SECTION; got != expect {
          return nil, fmt.Errorf(
            "Argument %d expected to parse a section node of type %s, got %s",
            expect, got,
          )
        }

        section_node = node
      }
    }

    if is_format {
      for _, node := range section_node.Children {
        if node.Value.TokenType.IsValue() == false {
          return nil, fmt.Errorf("Error parsing format section, only values are expected, got an expression of type %s", node.NodeType)
        }

        var field = node.Value.String()
        if err := output_definition.SetEncodingField(field); err != nil {
          return nil, err
        }
      }

      // The next argument needs to be a destination
      expect_definition = true

    } else if is_filter {
      if filters, err := interpretFilterExpressionSection(section_node); err != nil {
        return nil, err
      } else if len(filters) >= 1 {
        output_definition.Filters = append(output_definition.Filters, filters...)
      }

    } else if is_destination {
      output_definition.Dest = arg
      expect_definition = false

      if output_definition.Encoding == 0 {
        output_definition.Encoding = ASSET_ENCODING_DEFAULT
      }

      // Work on a new, empty output definition
      //
      outputs = append(outputs, cliOutputDefinition {})
      output_definition = & outputs[len(outputs)-1]

    } else {
      panic("Argument is neither a format, filter, nor definition; this code should be unreachable")
    }
  }

  if expect_definition {
    var format_arg_num = len(args)
    var format_arg     = args[format_arg_num - 1]
    return nil, fmt.Errorf(
      "A destination was expected after the format in output argument %d (%s), but no additional arguments were defined",
      format_arg_num, format_arg,
    )
  }

  // Because new output objects are added to `outputs` when a
  // destination is defined, the last destination value is not
  // fully defined and does not reflect the outputs defined in
  // the CLI arguments. Truncate the last value.
  //
  outputs = outputs[:len(outputs)-1]

  return outputs, nil
}


func interpretFilterExpressionSection (filter_section *ExpressionNode) ([]cliFilterDefinition, error) {
  var filters = []cliFilterDefinition {}

  if got, expect := filter_section.NodeType, EXPRESSION_NODE_SECTION; got != expect {
    return nil, fmt.Errorf("Expected an filter expression section, got a %s", got)
  }

  for _, node := range filter_section.Children {
    filters = append(filters, cliFilterDefinition {})
    var filter = & filters[len(filters)-1]

    var field_name = node.Value.String()

    // Prefixing a filter with a minus inverts the query
    //
    if strings.HasPrefix(field_name, "-") {
      field_name    = strings.TrimLeft(field_name, "-")
      filter.Invert = true
    }

    if node.NodeType != EXPRESSION_NODE_ASSOCIATION {
      return nil, fmt.Errorf(
        `Unexpected %s in filter tokens of value "%s"`,
        node.NodeType, node.Name,
      )

    } else {
      var association = node
      var key_node, value_node *ExpressionNode

      for _, child := range association.Children {
        switch child.NodeType {
        case EXPRESSION_NODE_NAME:
          key_node = child
        case EXPRESSION_NODE_VALUE:
          value_node = child
        }
      }

      if key_node == nil || value_node == nil {
        return nil, fmt.Errorf("Could not determine key or value in association expression")
      }

      value, err := value_node.Value.EvaluateString()
      if err != nil {
        return nil, err
      }

      var name = key_node.Name

      if strings.HasPrefix(name, "-") {
        name = strings.TrimLeft(name, "-")
        filter.Invert = true
      }

      switch name {
        case "mimetype", "mime":
          filter.Mimetype = value
        case "prefix":
          filter.Prefix = value
        case "suffix":
          filter.Suffix = value
        case "extension", "ext":
          filter.Suffix = "." + strings.TrimLeft(value, ".")
        default:
          return nil, fmt.Errorf("Unrecognized filter field: %s", name)
      }
    }
  }

  return filters, nil
}
