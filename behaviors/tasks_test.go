package behaviors

import (
  "testing"
  . "gilchrist.tech/interbuilder"

  "strings"
  "bytes"

  "io"
  "os"
  "path/filepath"
  "fmt"
  "golang.org/x/net/html"
  "mime"
)


func TestTaskInferSourceNodeJS (t *testing.T) {
  var root      *Spec = NewSpec("root", nil)
  var node_spec *Spec = root.AddSubspec(NewSpec("node_spec", nil))

  root.AddSpecResolver(ResolveTaskInferSource)
  root.AddSpecResolver(ResolveTasksNodeJS)

  node_spec.Props["source_dir"] = t.TempDir()

  var root_package_json_src = []byte(/* package.json */`{
    "name": "interbuilder-test-npm-task",
    "type": "module",
    "scripts": {
      "start": "node main.js",
      "build": "node main.js"
    },
    "dependencies": {
      "my-module": "file:module-package"
    }
  }`)

  var module_package_json_src = []byte(/* ./module-package/main.js */`{
    "name": "my-module",
    "type": "module",
    "exports": {
      ".": {
        "require": "./main.js",
        "import":  "./main.js"
      }
    }
  }`)

  var root_main_js_src = []byte(/* main.js */`
    import fs   from 'fs';
    import path from 'path';

    import * as MyModule from "my-module"

    // Define the directory and file paths
    const dir_path  = path.join(".", "dist");
    const file_path = path.join(dir_path, "file.txt");

    // Create the dist/ directory if it doesn't exist
    if (!fs.existsSync(dir_path)) {
        fs.mkdirSync(dir_path, { recursive: true });
    }

    fs.writeFileSync(file_path, MyModule.content);

    console.log("File created at:", file_path);
  `)

  var module_main_js_src = []byte(/* module-package/main.js */`
    export const content = "hello world";
  `)

  // Write the package.json and main.js for the root module,
  // which is the main entrypoint for the test.
  //
  var err error
  err = node_spec.WriteFile("package.json", root_package_json_src,   0o660)
  if err != nil { t.Fatal(err) }
  err = node_spec.WriteFile("main.js", root_main_js_src, 0o660)
  if err != nil { t.Fatal(err) }

  // Write the package.json and main.js for my-module, a
  // dependancy of the root module. The test will fail if this is
  // not installed and read from by the root module.
  //
  node_spec.WriteFile("module-package/package.json", module_package_json_src, 0o660)
  if err != nil { t.Fatal(err) }
  node_spec.WriteFile("module-package/main.js", module_main_js_src, 0o660)
  if err != nil { t.Fatal(err) }

  if err := root.Resolve(); err != nil {
    t.Fatal("Could not resolve root spec:", err)
  }

  node_spec.EnqueueTaskName("source-infer")

  // Run a task to assert the content of the files emitted by the
  // Node build Spec.
  //
  var num_assets int = 0
  root.EnqueueTaskFunc("consume-dist", func (s *Spec, tk *Task) error {
    for asset_chunk := range s.Input {
      tk.Println("Asset chunk:", asset_chunk.Url)
      assets, err := asset_chunk.Expand() // TODO: assets.Flatten
      if err != nil { return err }

      for _, asset := range assets {
        num_assets++
        var asset_url_path string = strings.TrimLeft(asset.Url.Path, "/")

        if got, expect := asset_url_path, "@emit/file.txt"; got != expect {
          t.Fatalf("Unexpected asset path: %s in URL %s, expected %s", asset.Url.Path, asset.Url, expect)
        }

        if content_bytes, err := asset.GetContentBytes(); err != nil {
          t.Fatal(err)
        } else if got, expect := string(content_bytes), "hello world"; got != expect {
          t.Fatalf("Asset content is \"%s\", expected \"%s\"", got, expect)
        }
      }
    }
    return nil
  })

  if err := root.Run(); err != nil {
    t.Fatal(err)
  }

  if expect, got := 1, num_assets; got != expect {
    t.Fatalf("Expected %d assets, got %d", expect, got)
  }
}


func TestTaskConsumeLinkFilesSingularFiles (t *testing.T) {
  // Create root spec
  //
  root := NewSpec("root", nil)
  root.Props["source_dir"] = t.TempDir()
  root.Props["quiet"] = true

  subspec := root.AddSubspec( NewSpec("subspec", nil) )
  subspec.Props["source_dir"] = t.TempDir()
  path_transformations, err := PathTransformationsFromAny("s`^/?`new-`")
  if err != nil { t.Fatal(err) }
  subspec.PathTransformations = path_transformations

  subspec.EnqueueTaskFunc("produce-file", func (s *Spec, task *Task) error {
    source_dir, _ := s.RequirePropString("source_dir")

    for i := range 3 {
      file_name := fmt.Sprint(i, ".txt")
      file_path := filepath.Join(source_dir, file_name)

      file, err := os.Create(file_path)
      if err != nil { t.Fatal(err) }

      _, err = fmt.Fprintf(file, "Hello from %s", file_name)
      if err != nil { t.Fatal(err) }

      err = file.Close()
      if err != nil { t.Fatal(err) }

      if err := s.EmitFileKey(file_path, file_name); err != nil {
        t.Fatalf("Failed to emit file: %v", err)
      }
    }

    return nil
  })

  root.EnqueueTaskFunc("root-consume", TaskConsumeLinkFiles)

  TestWrapTimeoutError(t, root.Run)

  root_source_dir := root.Props["source_dir"].(string)

  stat, err := os.Stat(root_source_dir)

  if err != nil {
    t.Fatalf("Root spec source_dir stat error: %v", err)
  } else if ! stat.IsDir() {
    t.Fatal("Root spec source dir exists but is not a directory")
  }


  // Read each file in the root spec's source
  // directory and assert their contents.
  //
  dir_files, err := os.ReadDir(root_source_dir)
  if err != nil { t.Fatal(err) }

  for _, file_dirent := range dir_files {
    file_name := file_dirent.Name()
    file_path := filepath.Join(root_source_dir, file_name)

    file, err := os.Open(file_path)
    if err != nil { t.Fatal(err) }
    defer file.Close()

    file_content, err := io.ReadAll(file)
    if err != nil { t.Fatal(err) }

    switch file_name {
      case "new-0.txt", "new-1.txt", "new-2.txt":
        var expected = "Hello from " + file_name[4:]
        if string(file_content) != expected {
          t.Fatalf("Unexpected file contents in file %s, got %s, expected %s", file_name, file_content, expected)
        }
      default:
        t.Fatalf("Unexpected file name: %s", file_name)
    }
  }
}


func TestTaskTransformHtml (t *testing.T) {
  root             := NewSpec("root", nil)
  transformer_spec := root.AddSubspec( NewSpec("transformer", nil) )
  produce_spec     := transformer_spec.AddSubspec(NewSpec("produce", nil))

  // Path transformation
  //
  path_transformations, err := PathTransformationsFromAny("s`^/?`transformed/`")
  if err != nil { t.Fatal(err) }
  transformer_spec.PathTransformations = path_transformations

  // Prepare temporary source_dir's
  //
  produce_dir     := t.TempDir()
  index_file_path := filepath.Join(produce_dir, "index.html")
  text_file_path  := filepath.Join(produce_dir, "file.txt")

  root_source_dir := t.TempDir()

  root.Props["source_dir"]         = root_source_dir
  produce_spec.Props["source_dir"] = produce_dir

  // Write an HTML file and emit it
  //
  produce_spec.EnqueueTaskFunc("produce", func (s *Spec, task *Task) error {
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

    err = os.WriteFile(index_file_path, html_source, 0o660)
    if err != nil {
      return fmt.Errorf("Could not write asset HTML file to produce: %w", err)
    }

    text_source := []byte("This text file should go unmodified")
    err = os.WriteFile(text_file_path, text_source, 0o660)
    if err != nil {
      return fmt.Errorf("Could not write asset text file to produce: %w", err)
    }

    return s.EmitFileKey("/")
  })

  // Apply path transformations to file assets
  //
  transformer_spec.EnqueueTaskFunc("transform-html", func (s *Spec, task *Task) error {
    var num_assets int = 0

    for asset_chunk := range s.Input {
      assets, err :=  asset_chunk.Expand()
      if err != nil { return err }

      for _, a := range assets {
        num_assets++

        mediatype, _, err := mime.ParseMediaType(a.Mimetype)
        if err != nil { return err }

        switch mediatype {
        case "":
          return fmt.Errorf("Asset has an empty mimetype")
        default:
          return fmt.Errorf("Asset has an unexpected mimetype: %s", a.Mimetype)

        case "text/plain":
          // pass

        case "text/html":
          reader, err := a.GetReader()
          if err != nil { return err }

          doc, err := html.Parse(reader)
          if err != nil { return err }

          modified := HtmlNodeApplyPathTransformations(
              doc, a.Url, s.PathTransformations,
            )

          task.Println("modified:", modified)

          var buffer bytes.Buffer
          html.Render(& buffer, doc)
          err = a.SetContentBytes(buffer.Bytes())
          if err != nil { return err }
        }

        s.EmitAsset(a)
      }

      // Assert the number of assets
      //
      if expected := 1; num_assets != expected {
        return fmt.Errorf("Expected %d assets, got %d", expected, num_assets)
      }

    }
    return nil
  })

  // Consume the Spec trees and write them to the output directory (root's source_dir)
  //
  root.EnqueueTaskFunc("root-consume", func (s *Spec, task *Task) error {
    var num_assets int = 0
    for asset := range s.Input {
      num_assets++

      mediatype, _, err := mime.ParseMediaType(asset.Mimetype)
      if err != nil {
        return fmt.Errorf("Error parsing media type from mimetype \"%s\": %w", asset.Mimetype, err)
      }

      switch mediatype {
      case "":
        return fmt.Errorf("Asset has an empty mimetype")
      default:
        return fmt.Errorf("Asset has an unexpected mimetype: %s", asset.Mimetype)

      case "text/plain":
        // PASS
        asset := s.AnnexAsset(asset)

        content, err := asset.GetContentBytes()
        if err != nil { return err }

        writer, err := asset.GetWriter()
        _, err = writer.Write(content)
        if err != nil { return err }

        if writer, ok := writer.(io.Closer); ok {
          writer.Close()
        }

      case "text/html":
        if asset.ContentModified == false {
          return fmt.Errorf("HTML asset not modified")
        }

        asset = s.AnnexAsset(asset)

        writer, err := asset.GetWriter()
        if err != nil {
          return fmt.Errorf("Error getting asset writer for asset %s: %w", asset.Url, err)
        }

        content, err := asset.GetContentBytes()
        if err != nil { return err }

        _, err = writer.Write(content)
        if err != nil { return err }

        if writer, ok := writer.(io.Closer); ok {
          writer.Close()
        }
      }
    }

    if expected := 2; num_assets != expected {
      return fmt.Errorf("Expected %d assets, got %d", expected, num_assets)
    }

    return nil
  })

  if err := root.Run(); err != nil {
    t.Errorf("Error when running Spec tree: %v", err)
  }

  // Test Spec tree output
  //
  var root_spec_files_walked int = 0

  err = filepath.Walk(root_source_dir, func(file_path string, info os.FileInfo, err error) error {
    if err != nil { t.Fatal(err) }

    if info.IsDir() { // Skip directories
      return nil
    }

    // Read the file content
    content, err := os.ReadFile(file_path)
    if err != nil { t.Fatal(err) }

    // Store the file path and content in the map
    fmt.Println("Output:", file_path, len(content))

    _, file_name := filepath.Split(file_path)
    switch file_name {
      default:
        t.Errorf("Unrecognized file name: %s", file_name)

      case "index.html":
        content_bytes, err := os.ReadFile(file_path)
        if err != nil {
          t.Errorf("Error reading index.html: %v", err)
        }

        var content  string = string(content_bytes)
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
        content, err := os.ReadFile(file_path)
        if err != nil {
          t.Errorf("Error reading file.txt: %v", err)
        }

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
      expected, root_source_dir, root_spec_files_walked,
    )
  }
}


func TestTaskConsumeLinkFilesWithPathTransformations (t *testing.T) {
  var err error
  var root       *Spec = NewSpec("root", nil)
  var merge      *Spec = root.AddSubspec(NewSpec("merge", nil))
  var producer_a *Spec = merge.AddSubspec(NewSpec("producer_a", nil))
  var producer_b *Spec = merge.AddSubspec(NewSpec("producer_b", nil))

  merge.Props["source_dir"]      = t.TempDir()
  producer_a.Props["source_dir"] = t.TempDir()
  producer_b.Props["source_dir"] = t.TempDir()

  // Producer A and B should transform their paths
  //
  producer_a.PathTransformations, err = PathTransformationsFromAny("s`^/?`/a/`")
  if err != nil { t.Fatal(err) }
  producer_b.PathTransformations, err = PathTransformationsFromAny("s`^/?`/b/`")
  if err != nil { t.Fatal(err) }

  // Producer tasks
  //
  var produce_func = func (s *Spec, tk *Task) error {
    if err := s.WriteFile("output.txt", []byte(s.Name), 0o660); err != nil {
      return err
    }
    return s.EmitFileKey("/")
  }

  producer_a.EnqueueTaskFunc("produce", produce_func)
  producer_b.EnqueueTaskFunc("produce", produce_func)

  // Consumer
  //
  merge.EnqueueTaskFunc("consume", TaskConsumeLinkFiles)

  // The root spec checks the asset output
  //
  var num_assets int = 0
  root.EnqueueTaskFunc("assert-assets", func (s *Spec, tk *Task) error {
    for asset_chunk := range s.Input {
      assets, err := asset_chunk.Expand() // TODO: flatten, not expand
      if err != nil { return err }

      for i, asset := range assets {
        num_assets++
        tk.Println(i, asset.Url)

        url := asset.Url.String()
        var expected_content string
        switch url {
          default:
            t.Errorf("Asset with unrecognized URL: %s", url)
            continue
          case "ib://merge/@emit/a/output.txt":
            expected_content = "producer_a"
          case "ib://merge/@emit/b/output.txt":
            expected_content = "producer_b"
        }

        if bytes, err := asset.GetContentBytes(); err != nil {
          t.Errorf("Error reading %s: %v", url, err)
        } else if content := string(bytes); content != expected_content {
          t.Errorf("Asset %s has content \"%s\", expected \"%s\"", url, content, expected_content)
        }
      }
    }
    return nil
  })

  // Run the pipeline and assert the output
  //
  if err := root.Run(); err != nil {
    t.Fatal(err)
  }

  if expect, got := 2, num_assets; got != expect {
    t.Errorf("Expected %d assets, got %d", expect, got)
  }
}


func TestTaskConsumeLinkFilesModifiedFile (t *testing.T) {
  var err     error
  var consume *Spec = NewSpec("consume", nil)
  var produce *Spec = consume.AddSubspec(NewSpec("produce", nil))

  var output_dir string = t.TempDir()
  consume.Props["source_dir"] = output_dir
  produce.Props["source_dir"] = t.TempDir()

  var unmodified_content string = "unmodified"
  var   modified_content string =   "MODIFIED"

  produce.EnqueueTaskFunc("produce", func (s *Spec, tk *Task) error {
    err = s.WriteFile("unmodified.txt", []byte(unmodified_content), 0o660)
    if err != nil { return err }
    // TODO
    err = s.WriteFile("modified.txt", []byte(modified_content), 0o660)
    if err != nil { return err }

    if err = s.EmitFileKey("unmodified.txt"); err != nil { return err }
    if err = s.EmitFileKey("modified.txt"); err != nil { return err }

    return nil
  })

  consume.EnqueueTaskFunc("consume-link", TaskConsumeLinkFiles)

  if err = consume.Run(); err != nil {
    t.Fatal(err)
  }

  var unmodified_path string = filepath.Join(output_dir, "unmodified.txt")
  var   modified_path string = filepath.Join(output_dir,   "modified.txt")

  if bytes, err := os.ReadFile(unmodified_path); err != nil {
    t.Error(err)
  } else if content, expect := string(bytes), unmodified_content; content != expect {
    t.Errorf("File %s has content \"%s\", expected \"%s\"", "unmodified.txt", content, expect)
  }

  if bytes, err := os.ReadFile(modified_path); err != nil {
    t.Error(err)
  } else if content, expect := string(bytes), modified_content; content != expect {
    t.Errorf("File %s has content \"%s\", expected \"%s\"", "modified.txt", content, expect)
  }
}
