package interbuilder

import (
  "fmt"
  "testing"
  "encoding/json"
  "regexp"
)


func TestTokenizeMatcherExpressionBasicCases (t *testing.T) {
  var test_cases = []struct {Src string; Tokens []string} {
    {Src: "m/test/",          Tokens: []string {"m", "test", ""}},
    {Src: "m`test`i",         Tokens: []string {"m", "test", "i"}},
    {Src: "s/find/replace/g", Tokens: []string {"s", "find", "replace", "g"}},
    {Src: "/asdf/",           Tokens: []string {"", "asdf", ""}},
    {Src: "`asdf`",           Tokens: []string {"", "asdf", ""}},
    {Src: "/find/replace/",   Tokens: []string {"", "find", "replace", ""}},
  }

  for _, test_case := range test_cases {
    fields, err := tokenizeMatcherExpression(test_case.Src)

    if err != nil {
      t.Fatal(err)
    }

    if len(fields) != len(test_case.Tokens) {
      t.Fatalf(
        "Improperly tokenized expression \"%s\", tokens are not the correct length, expected %d, got %d",
        test_case.Src, len(test_case.Tokens), len(fields),
      )
    }

    for i, field_value := range fields {
      if field_value != test_case.Tokens[i] {
        t.Fatalf(
          "Improperly tokenized expression \"%s\", field at index %d is incorrect, have \"%s\", want \"%s\"",
          test_case.Src, i, field_value, test_case.Tokens[i],
        )
      }
    }
  }
}


func TestInvalidPathTransformationsFromString (t *testing.T) {
  var invalid_transform_strings = []string {
    "",                  // Empty string
    "k/find/repl/flag",  // Unsupported mode: k
    "s`find`replace",    // Lacks a delimiter to denote flags
  }

  // Assert that bad transformation strings
  // only output errors when parsed.
  //
  for _, transform_definition := range invalid_transform_strings {
    transformation, err := PathTransformationFromString(transform_definition)

    if err == nil {
      t.Fatalf("Parsing transformation string \"%s\" did not output an error", transform_definition)
    }

    if transformation != nil {
      t.Fatalf("Parsing transformation string \"%s\" outputted an object", transform_definition)
    }
  }
}


func TestStringMatcherReplaceBasicCase (t *testing.T) {
  var expected string = "replaced"

  var matcher = & StringMatcher {
    MatchRegexp:    regexp.MustCompile("find"),
    OperandString:  expected,
    IsSubstitution: true,
  }

  var replaced string = matcher.ReplaceString("find")

  if replaced != expected {
    t.Fatalf("StringMatcher.ReplaceString returned %s, expected %s", replaced, expected)
  }
}


func TestPathTransformationsFromString (t *testing.T) {
  // Assert working transformations
  // parse and perform their transformations properly
  //

  var valid_transform_data = [][3]string {
    {
      "s/find/replace/i",
      "path/FiNd/",
      "path/replace/",
    },
    {
      "s/^/prefix-/",
      "string",
      "prefix-string",
    },
    {
      "s`/path/`/path/changed/`",
      "root/path/unchanged",
      "root/path/changed/unchanged",
    },
    {
      "s`(/?|^)delete/`$1`",
      "delete/path/",
      "path/",
    },
    {
      "s/one/not-one/",
      "one, one",
      "not-one, one",
    },
    {
      "s/one/many/g",
      "one, many",
      "many, many",
    },
  }

  for _, test_data := range valid_transform_data {
    var transform_str = test_data[0]
    var from_str      = test_data[1]
    var to_str        = test_data[2]

    var transform, err = PathTransformationFromString(transform_str)

    if err != nil {
      t.Fatalf("Error when parsing path transformation from string \"%s\": %s", transform_str, err)
    }

    result := transform.TransformPath(from_str)

    if result != to_str {
      t.Fatalf("Transform from string \"%s\" does not replace string \"%s\" with \"%s\", got \"%s\"", transform_str, from_str, to_str, result)
    }
  }
}


func TestInvalidPathTransformationsFromProp (t *testing.T) {
  var test_cases_src = []string {
    // Just find, but no replace
    `{ "find": "m/string/" }`,

    // Replace field with no find, and not a substitution expression
    `{ "replace": "m/string/" }`,

    // Matcher is defined as a substitution, but so are replacers
    `{ "match": "s/find/replace/", "replace": "s/find/replace/" }`,
    `{ "match": "s/find/replace/", "find": "m/find/", "replace": "find" }`,

    // Unrecognized properties
    `{ "this-property-doesnt-exist??!!": "value" }`,
  }

  for _, test_case_src := range test_cases_src {
    var test_case_prop map[string]any

    err := json.Unmarshal([]byte(test_case_src), &test_case_prop)
    if err != nil { t.Fatal(err) }

    path_transformation, err := PathTransformationFromProp(test_case_prop)
    if err == nil {
      t.Fatalf("Test case %s expected an error, but parsed without one", test_case_src)
    }
    if path_transformation != nil {
      t.Fatalf("Test case %s expected an error, but parsed without one", test_case_src)
    }
  }
}


func TestPathTransformationsFromProp (t *testing.T) {
  var test_cases_src = [] struct {Src string; Path string; Expect string; NoMatch bool } {
    {Src: `{ "match": "m/string/" }`,        Path: "/root/string/", Expect: "=" },
    {Src: `{ "replace": "/find/replace/" }`, Path: "/find/",        Expect: "/replace/" },

    { Src: `{ "match": "/don't match/", "replace": "s/don't/do/" }`, Path: "/does-not-match/", NoMatch: true },
    { Src: `{ "find": "m/find/", "replace": "replaced" }`, Path: "/find/", Expect: "/replaced/" },

    { Src: `{ "prefix": "prefix" }`, Path: "/path/", Expect: "/prefix/path/" },

    { Src: `{ "match": "/^match$/", "prefix": "prefix" }`, Path: "match",    Expect: "prefix/match" },
    { Src: `{ "match": "/^match$/", "prefix": "prefix" }`, Path: "no-match", NoMatch: true },
  }

  for _, test_case := range test_cases_src {
    test_case_src := test_case.Src
    var test_case_prop map[string]any

    if test_case.Expect == "=" || test_case.NoMatch {
      test_case.Expect = test_case.Path
    }

    err := json.Unmarshal([]byte(test_case_src), &test_case_prop)
    if err != nil { t.Fatal(err) }

    path_transformation, err := PathTransformationFromProp(test_case_prop)
    if err != nil {
      t.Fatalf("Test case %s encountered an error while parsing: %s", test_case.Src, err)
    }

    matches := path_transformation.MatchString(test_case.Path)

    if test_case.NoMatch {
      if matches {
        t.Fatalf("Test case %s matches", test_case.Src)
      }
    } else {
      if ! matches {
        t.Fatalf("Test case %s did not match", test_case.Src)
      }
    }

    transformed_path := path_transformation.TransformPath(test_case.Path)

    if transformed_path != test_case.Expect {
      fmt.Println(path_transformation.Replacer)
      t.Fatalf("Test case %s path replacement did not equal %s, got %s", test_case.Src, test_case.Expect, transformed_path)
    }
  }
}
