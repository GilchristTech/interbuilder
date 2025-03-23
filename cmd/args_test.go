package main

import (
  "testing"
  "fmt"
)

func TestParseOutputArgs (t *testing.T) {
  // Test an empty set of arguments
  //
  if output_definitions, err := parseOutputArgs([]string{}); err != nil {
    t.Fatal("parseOutputArgs returned an error when given empty arguments:", err)
  } else if length, expect := len(output_definitions), 0; length != expect {
    t.Fatalf("parseOutputArgs returned %d definitions, expected %d", length, expect)
  }

  // Test a one-file set of arguments, no expressions
  //
  if output_definitions, err := parseOutputArgs(
    []string { "output.assets.json" },
  ); err != nil {

    t.Fatal("parseOutputArgs returned an error when given one file argument:", err)

  } else if length, expect := len(output_definitions), 1; length != expect {
    t.Fatalf("parseOutputArgs returned %d definitions, expected %d", length, expect)

  } else if dest, expect := output_definitions[0].Dest, "output.assets.json"; dest != expect {
    t.Fatalf("Expected output definition's path to be \"%s\", got \"%s\"", expect, dest)
  }

  // Test a one-file output with a format expression
  //
  if output_definitions, err := parseOutputArgs(
    []string { "format:text", "output.assets.txt" },
  ); err != nil {

    t.Fatal("parseOutputArgs returned an error when given an expression and file argument:", err)

  } else if length, expect := len(output_definitions), 1; length != expect {
    t.Fatalf("parseOutputArgs returned %d definitions, expected %d", length, expect)

  } else {
    var definition = output_definitions[0]

    if dest, expect :=  definition.Dest, "output.assets.txt"; dest != expect {
      t.Errorf("Expected output definition's path to be \"%s\", got \"%s\"", expect, dest)
    }

    if length := len(definition.Filters); length != 0 {
      t.Errorf("Expected output definition to have zero filters, got %d", length)
    }
  }

  // Test a two-file output
  //
  if output_definitions, err := parseOutputArgs(
    []string { "output1.assets.txt", "output2.assets.txt" },
  ); err != nil {

    t.Fatal("parseOutputArgs returned an error when given two file arguments:", err)

  } else if length, expect := len(output_definitions), 2; length != expect {
    t.Fatalf("parseOutputArgs returned %d definitions, expected %d", length, expect)

  } else {
    for definition_i, definition := range output_definitions {
      var expected_dest = fmt.Sprintf("output%d.assets.txt", definition_i+1)

      if dest := definition.Dest; dest != expected_dest {
        t.Errorf("Expected output definition's path to be \"%s\", got \"%s\"", expected_dest, dest)
      }

      if length := len(definition.Filters); length != 0 {
        t.Errorf("Expected output definition to have zero filters, got %d", length)
      }
    }
  }

  // Test that an expression without a file argument errors
  //
  if output_definitions, err := parseOutputArgs(
    []string { "format:json" },
  ); err == nil {
    t.Fatal("Expected parseOutputArgs to error when provided an expression with no file output, but its error was nil")
  } else if output_definitions != nil {
    t.Fatal("Expected parseOutputArgs to return nil when provided an expression with no file output, got", output_definitions)
  }
}
