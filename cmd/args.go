package main

import (
  "github.com/spf13/cobra"
  "os"
  "io"
  "strings"
  "fmt"
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
  Dest     string
  Encoding uint64
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
  var outputs = []cliOutputDefinition {
    cliOutputDefinition { },
  }
  var output_definition = &outputs[0]

  var expect_definition = false

  for arg_i, arg := range args {
    var is_format      bool = strings.HasPrefix(arg, "format:")
    var is_destination bool = ! is_format

    if !is_destination && expect_definition {
      // A destination is expected, but this argument is not a
      // destination; exit with an error.
      //
      return nil, fmt.Errorf(
        "A destination was expected in output argument %d, but got %s instead\n",
        arg_i+1, arg,
      )
    }

    if is_format {
      // Parse format definition

      var format_fields []string = strings.Split(
        arg[ len("format:") : ], ",",
      )

      for _, field := range format_fields {
        if err := output_definition.SetEncodingField(field); err != nil {
          return nil, err
        }
      }

      // The next argument needs to be a destination
      expect_definition = true
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
      panic("Argument is neither a format nor definition; this should be impossible.")
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
