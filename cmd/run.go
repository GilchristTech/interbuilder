package main

import (
  . "github.com/GilchristTech/interbuilder"

  "github.com/spf13/cobra"

  "fmt"
  "os"
  "path"
  "encoding/json"
  "gopkg.in/yaml.v3"
)


var cmd_run = & cobra.Command {
  Use: "run [file] [outputs...]",
  Short: "Run from a build specification file",
  Args: cobra.MinimumNArgs(1),
  Run: func (cmd *cobra.Command, args []string) {
    var spec_file string = args[0]
    var output_args = args[1:]

    // Parse outputs
    //
    var output_definitions []cliOutputDefinition
    var err error

    // Parse output positional arguments
    if output_definitions, err = parseOutputArgs(output_args); err != nil {
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

    var root *Spec = MakeDefaultRootSpec()

    // handle flag: --print-spec
    //
    if Flag_print_spec {
      defer func () {
        fmt.Println()
        PrintSpec(root)
      }()
    }

    // Load spec configuration from file - read the file's bytes
    // and infer how to unmarshal it from the file extension.
    // TODO: add CLI flag for specifying the format of a spec file. This would allow an input of "-" to specify a on-default format build spec from STDIN, or input from a file with no extension.
    //
    specs_bytes, err := os.ReadFile(spec_file)
    if err != nil {
      fmt.Printf("Could not read spec file: %v\n", err)
      os.Exit(1)
    }

    switch extension := path.Ext(spec_file); extension {
    default:
      fmt.Printf("Unknown spec file format: %s\n", extension)
      os.Exit(1)

    case ".json":
      if err := json.Unmarshal(specs_bytes, &root.Props); err != nil {
        fmt.Printf("Could not parse spec JSON file: %v\n", err)
        os.Exit(1)
      }

    case ".yaml":
      var props = make(map[string]any)
      if err := yaml.Unmarshal(specs_bytes, props); err != nil {
        fmt.Printf("Could not parse spec YAML file: %v\n", err)
        os.Exit(1)
      }
      root.Props = props
    }

    // Create tasks for outputs
    //
    for output_i, output_definition := range output_definitions {
      var task_name = fmt.Sprintf("cli-output-%d", output_i)
      if err := output_definition.EnqueueTasks(task_name, root); err != nil {
        fmt.Println("Error while creating creating output tasks:\n\t%v\n", err)
        os.Exit(1)
      }
    }

    // Resolve
    //
    if err = root.Build() ; err != nil {
      fmt.Printf("Error while building build specs: %v\n", err)
      os.Exit(1)
    }

    // Run tasks
    //
    if err = root.Run() ; err != nil {
      if Flag_print_spec {
        PrintSpec(root)
      }
      fmt.Printf("Error while running build specs: %v\n", err)
      os.Exit(1)
    }
  },
}
