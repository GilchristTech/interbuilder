package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "testing"
  "bytes"
  "net/url"
  "strings"
)

func TestCss (t *testing.T) {
  var css_raw = bytes.NewBufferString(`
    :root {
      --variable: url(/static/background.png);
    }

    body {
      background: url(/static/background.png);
      background: var(--variable);
    }
  `)

  path_transformations, err := PathTransformationsFromAny("s`^/?`transformed/`")
  if err != nil { t.Fatal(err) }

  var buffer_out = bytes.NewBuffer(nil)

  var base_url, _ = url.Parse("/")

  modified, err := CssReaderApplyPathTransformationsTo(
    css_raw, buffer_out, base_url, path_transformations )

  if err != nil { t.Error(err) }

  if modified == false {
    t.Errorf("Expected CSS to be modified, but it was not")
  }

  var transformed_css = buffer_out.String()

  var expected_strings = []string {
    `--variable: url(/transformed/static/background.png`,
    `background: url(/transformed/static/background.png)`,
  }

  var printed_css = false

  for _, expected := range expected_strings {
    if strings.Contains(transformed_css, expected) == false {
      if !printed_css {
        t.Log(transformed_css)
        printed_css = true
      }
      t.Errorf("Expected transformed CSS to contain \"%s\", but it does not", expected)
    }
  }
}
