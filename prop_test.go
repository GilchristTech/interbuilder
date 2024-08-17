package interbuilder

import (
  "testing"
)


func TestStringProps (t *testing.T) {
  root := NewSpec("root", nil)
  spec := root.AddSubspec( NewSpec("spec", nil) )
  spec.Props["string"] = "value"
  root.Props["inherited-string"] = "value"

  var value     string
  var found, ok   bool
  var err        error

  value, ok, found = spec.GetPropString("string")
  if !found { t.Fatal("Prop \"string\" was not found") }
  if !ok    { t.Fatalf("Prop \"string\" is not a string, got %T", value) }
  if value != "value" {
    t.Fatalf("Prop \"string\" has a value of \"%s\", expected \"value\"", value)
  }

  value, ok, found = spec.InheritPropString("inherited-string")
  if !found { t.Fatal("Prop \"inherited-string\" was not found") }
  if !ok    { t.Fatalf("Prop \"inherited-string\" is not a string, got %T", value) }
  if value != "value" {
    t.Fatalf("Prop \"string\" has a value of %s, expected \"value\"", value)
  }

  value, err = spec.RequirePropString("string")
  if err != nil { t.Fatal(err) }
  if value != "value" {
    t.Fatalf("Prop \"string\" has a value of \"%s\", expected \"value\"", value)
  }
}
