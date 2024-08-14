package behaviors

import (
  "testing"
  . "gilchrist.tech/interbuilder"

  "io"
	"os"
	"path/filepath"
  "fmt"
)


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

  err = root.Run()
  if err != nil {
    t.Fatal(err) 
  }

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

    fmt.Println(file_path)

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
