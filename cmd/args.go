package main

import (
  "github.com/spf13/cobra"
  "os"
  "io"
)


var Flag_print_spec bool
var Flag_outputs    []string
var Flag_inputs     []string


func init () {
  cmd_root.AddCommand(cmd_run)
  cmd_root.AddCommand(cmd_assets)

  cmdAddSpecRunFlags(cmd_run)
  cmdAddSpecRunFlags(cmd_assets)

  cmdAddAssetIoFlags(cmd_run)
  cmdAddAssetIoFlags(cmd_assets)
}


func cmdAddSpecRunFlags (cmd *cobra.Command) {
  cmd.PersistentFlags().BoolVar(
    &Flag_print_spec, "print-spec", false,
    "Print the build specification tree when execution is finished",
  )
}


func cmdAddAssetIoFlags (cmd *cobra.Command) {
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
