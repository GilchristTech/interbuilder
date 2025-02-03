package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "net/url"
  "fmt"
  "golang.org/x/net/html"
  "io"
  "strings"
)


var TaskResolverApplyPathTransformationsToHtmlContent = TaskResolver {
  Id:   "apply-path-transformations-html",
  Name: "apply-path-transformations-html",
  MatchFunc: func (name string, spec *Spec) (bool, error) {
    if name != "apply-path-transformations-html" {
      return false, nil
    }
    return len(spec.PathTransformations) > 0, nil
  },
  TaskPrototype: Task {
    Mask: TASK_ASSETS_MUTATE,
    MatchMimePrefix: "text/html",
    MapFunc: TaskMapApplyPathTransformationsToHtmlContent,
  },
}


/*
  HtmlNodeApplyPathTransformations, given an HTML document/node,
  a base URL, and an array of transformations, traverses the HTML
  document looking for matching URLs in 'href' attributes, and
  applies those transformations which match. The document is
  mutated in-place, and true is returned if the document was
  modified.
*/
func HtmlNodeApplyPathTransformations (node *html.Node, base_url *url.URL, transformations []*PathTransformation) bool {
  var modified bool = false

  if node.Type == html.ElementNode {
    for attr_i := range node.Attr {
      var modify_attribute bool   = false
      var attribute_key    string = node.Attr[attr_i].Key
      var attribute_value  string = node.Attr[attr_i].Val

      if strings.HasPrefix(attribute_value, "javascript:") {
        continue
      }

      switch attribute_key {
        case "href", "src", "srcset":
          modify_attribute = true
      }

      if modify_attribute == false {
        continue
      }

      href_relative, err := url.Parse(attribute_value)

      if href_relative.Host != "" {
        continue
      }

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

  for child := node.FirstChild; child != nil; child = child.NextSibling {
    child_modified := HtmlNodeApplyPathTransformations(child, base_url, transformations)
    modified = modified || child_modified
  }

  return modified
}


/*
  AssetContentDataReadHtml is an Asset ContentData handler which
  reads bytes and returns an HTML document tree in a *html.Node.
*/
func AssetContentDataReadHtml (a *Asset, r io.Reader) (any, error) {
  html_doc, err := html.Parse(r)
  if err != nil {
    return nil, fmt.Errorf("Error parsing HTML content data: %w", err)
  }
  return html_doc, nil
}


/*
  AssetContentDataWriteHtml is an Asset ContentData writer, which
  renders an HTML document into the provided writer.
*/
func AssetContentDataWriteHtml (a *Asset, w io.Writer, content_data any) (int, error) {
  html_doc, ok := content_data.(*html.Node)
  if !ok {
    return 0, fmt.Errorf("Error writing content data: expected content data to be an *html.Doc, got %T", content_data)
  }
  
  return -1, html.Render(w, html_doc)
}


/*
  TaskMapContentDataHtmlHandlers is a Task MapFunc which assigns
  ContentData handlers to the Asset, but only if the asset has a
  MIME type prefixed with "test/html"; otherwise the asset is
  returned as-is.
*/
func TaskMapContentDataHtmlHandlers (a *Asset) (*Asset, error) {
  // if ! strings.HasPrefix(a.Mimetype, "text/html") {
  //   return a, nil
  // }

  if err := a.SetContentDataWriteFunc(AssetContentDataWriteHtml); err != nil {
    return nil, err
  }

  if a.HasContentData() || a.HasContentDataReadFunc() {
    return a, nil
  }

  if err := a.SetContentDataReadFunc(AssetContentDataReadHtml); err != nil {
    return nil, err
  }

  return a, nil
}


/*
  TaskMapApplyPathTransformationsToHtmlContent is a Task MapFunc
  which reads an Asset's Spec's PathTransformations and applies
  them to assets, assuming their content is HTML.
*/
func TaskMapApplyPathTransformationsToHtmlContent (a *Asset) (*Asset, error) {
  var err error

  if a, err = TaskMapContentDataHtmlHandlers(a); err != nil {
    return nil, err
  }

  doc_any, err := a.GetContentData()
  if err != nil { return nil, err }
  doc, ok := doc_any.(*html.Node)

  if ! ok {
    return nil, fmt.Errorf("Asset ContentData was expected to be a *html.Node, got a %T", doc_any)
  }

  modified := HtmlNodeApplyPathTransformations(
      doc, a.Url, a.Spec.PathTransformations,
    )

  if modified {
    a.SetContentData(doc)
  }

  return a, nil
}


/*
  HTML task inference
*/
var TaskResolverAssetsInferHtml = TaskResolver {
  Name: "assets-infer",
  Id:   "assets-infer-html-path-transformations",
  MatchFunc: func (name string, spec *Spec) (bool, error) {
    if ! strings.HasPrefix(name, "assets-infer") {
      return false, nil
    }

    if len(spec.PathTransformations) == 0 {
      return false, nil
    }

    return true, nil
  },

  AcceptMask: TASK_ASSETS_GENERATE | TASK_ASSETS_FILTER | TASK_ASSETS_MUTATE,

  TaskPrototype: Task {
    MatchMimePrefix: "text/html",
    Mask: TASK_TASKS_QUEUE,

    Func: func (sp *Spec, tk *Task) error {
      _, err := tk.EnqueueUniqueTaskName("apply-path-transformations-html")
      return err
    },
  },
}
