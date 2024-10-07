package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "github.com/tdewolff/parse/v2"
  "github.com/tdewolff/parse/v2/css"

  "net/url"
  "bytes"
  "fmt"
  "io"
  "regexp"
)

var css_url_regexp = regexp.MustCompile(`(\s*[uU][rR][lL]\(\s*"?)(.*)("?\s*\))`)


func CssReaderApplyPathTransformationsTo (reader io.Reader, writer io.Writer, base_url *url.URL, transformations []*PathTransformation) (modified bool, err error) {
  var input = parse.NewInput(reader)
  var lexer = css.NewLexer(input)

  var line_number int = 1

  for {
    token_type, token_data := lexer.Next()

    if token_type == css.ErrorToken {
      if err := lexer.Err(); err != io.EOF {
        return false, fmt.Errorf("CSS error on line %d: %v", line_number, err)
      }
      break
    }

    if token_type != css.URLToken {
      writer.Write(token_data)

    } else {

      // Match the URL definition to get the URL value for
      // applying PathTransformations
      //
      var url_definition = string(token_data)
      var url_definition_matches = css_url_regexp.FindStringSubmatch(url_definition)

      if len(url_definition_matches) == 0 {
        writer.Write(token_data)
        continue
      }

      // This URL definition matches. Extract parts of its text
      // to reconstruct it as-is, after changing the URL value
      // itself. This will maintain spacing and capitalization of
      // the "url" function itself (which is case-insensitive)
      //
      var new_url_token []byte = nil

      var prefix  string = url_definition_matches[1]
      var url_raw string = url_definition_matches[2]
      var suffix  string = url_definition_matches[3]

      var url_value *url.URL

      if url_parsed, err := url.Parse(url_raw); err != nil {
        return false, err
      } else {
        url_value = base_url.ResolveReference(url_parsed)
      }

      if url_value.Host == "" {
        // this is a relative URL, pass
      } else if url_value.Host != base_url.Host {
        // this is an external URL, do not modify
        continue
      }

      var original_path string = url_value.Path
      var path          string = original_path

      for _, transformation := range transformations {
        path = transformation.TransformPath(path)
      }

      if original_path != path {
        modified = true

        var new_url string

        if url_value.Host == "" {
          new_url = path
        } else {
          url_value.Path = path
          new_url = url_value.String()
        }

        new_url_token = []byte(prefix + new_url + suffix)
      }

      if new_url_token == nil {
        writer.Write(token_data)
      } else {
        writer.Write(new_url_token)
      }
    }

    line_number += bytes.Count(token_data, []byte("\n"))
  }

  return modified, nil
}
