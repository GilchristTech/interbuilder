package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "net/url"
  "fmt"
  "golang.org/x/net/html"
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
        fmt.Println("url:", href_url)

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

          fmt.Println("modified:", node.Attr[attr_i].Val)
        }
      }
    }
  }

  for child := node.FirstChild; child != nil; child = child.NextSibling {
    modified = modified || HtmlNodeApplyPathTransformations(child, base_url, transformations)
  }

  return modified
}
