package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "net/url"
  "fmt"
  "golang.org/x/net/html"
  "io"
)


func HtmlNodeApplyPathTransformations (node *html.Node, base_url *url.URL, transformations []*PathTransformation) bool {
  var modified bool = false

  if node.Type == html.ElementNode && node.Data == "a" {
    for attr_i := range node.Attr {
      if node.Attr[attr_i].Key == "href" {
        href_relative, err := url.Parse(node.Attr[attr_i].Val)

        if err != nil {
          continue
        }

        href_url := base_url.ResolveReference(href_relative)

        var original_path string = href_url.Path
        var path          string = original_path

        for _, transformation := range transformations {
          path = transformation.TransformPath(path)
        }

        if original_path != path {
          modified = true
          href_url.Path = path

          if href_relative.Host == "" {
            node.Attr[attr_i].Val = href_url.Path
          } else {
            node.Attr[attr_i].Val = href_url.String()
          }
        }
      }
    }
  }

  for child := node.FirstChild; child != nil; child = child.NextSibling {
    modified = modified || HtmlNodeApplyPathTransformations(child, base_url, transformations)
  }

  return modified
}


func AssetContentDataReadHtml (a *Asset, r io.Reader) (any, error) {
  html_doc, err := html.Parse(r)
  if err != nil {
    return nil, fmt.Errorf("Error reading content data: %w", err)
  }
  return html_doc, nil
}


func AssetContentDataWriteHtml (a *Asset, w io.Writer, content_data any) (int, error) {
  html_doc, ok := content_data.(*html.Node)
  if !ok {
    return 0, fmt.Errorf("Error writing content data: expected content data to be an *html.Doc, got %T", content_data)
  }
  
  return -1, html.Render(w, html_doc)
}
