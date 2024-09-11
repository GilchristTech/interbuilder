package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "testing"
  "fmt"
  "strings"
  "os"
  "path/filepath"
)


func TestHtmlPipeline (t *testing.T) {
  root := NewSpec("root", nil)
  spec := root.AddSubspec(NewSpec("spec", nil))
  var source_dir = t.TempDir()
  var output_dir = t.TempDir()
  root.Props["quiet"]      = true
  root.Props["source_dir"] = output_dir
  spec.Props["source_dir"] = source_dir

  // Path transformation
  //
  path_transformations, err := PathTransformationsFromAny("s`^/?`transformed/`")
  if err != nil { t.Fatal(err) }
  spec.PathTransformations = path_transformations


  // Produce an HTML asset and pass it
  //
  spec.EnqueueTaskFunc("produce", func (s *Spec, tk *Task) error {
    // Write HTML file
    //
    html_source := []byte(
      `<!DOCTYPE html>
      <html lang="en">
      <head>
        <meta charset="UTF-8">
        <title>Internal page</title>
      </head>
      <body>
        <a href="/page/">Internal link</a>
        <a href="http://example.com">External link</a>
      </body>
      </html>
    `)

    if err := s.WriteFile("index.html", html_source, 0o660); err != nil {
      return fmt.Errorf("Could not write asset HTML file to produce: %w", err)
    }

    index_asset, err := s.MakeFileKeyAsset("index.html")
    if err != nil { return err }
    err = tk.PassSingularAsset(index_asset)
    if err != nil { return err }

    // Write control TXT file
    //
    txt_source := []byte("This text file should go unmodified")

    if err := s.WriteFile("file.txt", txt_source, 0o660); err != nil {
      return fmt.Errorf("Could not write asset HTML file to produce: %w", err)
    }

    text_asset, err := s.MakeFileKeyAsset("file.txt")
    if err != nil { return err }
    err = tk.PassSingularAsset(text_asset)
    if err != nil { return err }

    return nil
  })

  // Assign HTML ContentData handlers and transform links in HTML
  // based on path transformations
  //
  spec.EnqueueTask( TaskResolverApplyPathTransformationsToHtmlContent.NewTask() )

  // Consume assets
  //
  root.EnqueueTaskFunc("write", TaskConsumeLinkFiles)

  if err := root.Run(); err != nil {
    t.Errorf("Error when running Spec tree: %v", err)
  }

  // Test Spec tree output
  //
  var root_spec_files_walked int = 0

  err = filepath.Walk(output_dir, func(file_path string, info os.FileInfo, err error) error {
    if err != nil { t.Fatal(err) }

    if info.IsDir() { // Skip directories
      return nil
    }

    // Read the file content
    content_bytes, err := os.ReadFile(file_path)
    if err != nil { t.Fatal(err) }
    content := string(content_bytes)

    // Store the file path and content in the map

    relative_path, err := filepath.Rel(output_dir, file_path)
    if err != nil { t.Fatal(err) }

    if ! strings.HasPrefix(relative_path, "transformed") {
      t.Errorf("File path was not transformed on output asset with path %s", file_path)
    }

    _, file_name := filepath.Split(file_path)
    switch file_name {
      default:
        t.Errorf("Unrecognized file name: %s", file_name)

      case "index.html":
        var expected string

        expected = "href=\"/transformed/page/\""
        if ! strings.Contains(content, expected) {
          t.Errorf("HTML content does not contain %s", expected)
        }

        expected = "href=\"http://example.com\""
        if ! strings.Contains(content, expected) {
          t.Errorf("HTML content does not contain %s", expected)
        }

      case "file.txt":
        expected := "This text file should go unmodified"
        if content := string(content); content != expected {
          t.Errorf("file.txt content is \"%s\", expected \"%s\"", content, expected)
        }
    }

    root_spec_files_walked++
    return nil
  })

  if err != nil {
    t.Fatalf("Error walking root spec source_dir: %v", err)
  }

  if expected := 2; root_spec_files_walked != expected {
    t.Fatalf(
      "Expected to walk %d files in root spec source dir (%s), walked %d",
      expected, output_dir, root_spec_files_walked,
    )
  }
}
