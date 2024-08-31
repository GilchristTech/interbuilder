package behaviors

import (
  "fmt"
  . "gilchrist.tech/interbuilder"
  "sync"
  "path"
  "path/filepath"
  "os"
  "strings"
)



var DownloaderMutex sync.Mutex


var TaskResolverSourceGitClone = TaskResolver {
  Id: "source-git-clone",
  Name: "git-clone", // TODO: consider renaming to source-get-git
  TaskPrototype: Task {
    Func: TaskSourceGitClone,
  },
}

func TaskSourceGitClone (s *Spec, t *Task) error {
  DownloaderMutex.Lock()
  defer DownloaderMutex.Unlock()

  source, err := s.RequirePropUrl("source")
  if err != nil { return err }

  if source.Scheme == "git" {
    source_copy        := *source
    source_copy.Scheme  = "https"
    source              = &source_copy
  }

  source_dir, err := t.Spec.RequirePropString("source_dir")
  if err != nil { return err }

  source_dir, err = filepath.Abs(source_dir)
  if err != nil { return err }

  // Check whether source directory already exists;
  // exit if it exists or if an error occurred.
  // TODO: check for .git/ existence and `git status --porcelain`
  //
  if exists, err := s.PathExists("./"); exists || err != nil {
    return err
  }

  if err := os.MkdirAll(source_dir, os.ModePerm); err != nil {
    return err
  }

  _, err = t.CommandRun("git", "clone", source.String(), source_dir)
  return err
}


var TaskResolverInferSource = TaskResolver {
  Id:        "source-infer-root",
  Name:      "source-infer",
  MatchFunc: nil,
  Children:  &TaskResolverInferSourceNodeJS,

  TaskPrototype: Task {
    Func: func (spec *Spec, task *Task) error {
      tr, err := task.Resolver.MatchChildren(task.Name, spec)
      if err != nil {
        task.Println("Error when inferring source")
        return fmt.Errorf("Error inferring while inferring source: %w", err)
      }

      if tr == nil {
        task.Println("Could not infer source")
        return nil
      }

      spec.EnqueueTask(tr.NewTask())
      return nil
    },
  },
}


var TaskResolverInferSourceNodeJS = TaskResolver {
  Id:   "source-infer-nodejs",
  Name: "source-infer",
  MatchFunc: func (name string, spec *Spec) (bool, error) {
    return spec.PathExists("package.json")
  },
  TaskPrototype: Task {
    Func: func (spec *Spec, task *Task) error {
      if _, e := spec.EnqueueTaskName("source-install-nodejs"); e != nil { return e }
      if _, e := spec.EnqueueTaskName("source-build-nodejs");   e != nil { return e }
      // if _, e := spec.EnqueueTaskName("source-emit");           e != nil { return e }
      return nil
    },
  },
}


var TaskResolverSourceInstallNodeJS = TaskResolver {
  Id:   "source-install-nodejs",
  Name: "source-install-nodejs",
  TaskPrototype: Task { Func: TaskSourceInstallNodeJS },
}

func TaskSourceInstallNodeJS (s *Spec, t *Task) error {
  DownloaderMutex.Lock()
  defer DownloaderMutex.Unlock()

  install_cmd := [] string {"npm", "i"}

  if lock_exists, _ := s.PathExists("package.lock"); lock_exists {
    install_cmd[1] = "ci"
  }

  if node_modules_exists, _ := s.PathExists("node_modules"); node_modules_exists {
    return nil
  }

  prop_install_cmd, ok, found := s.GetPropString("install_cmd")
  if found && ok {
    install_cmd = strings.Split(prop_install_cmd, " ")
  }

  if len(install_cmd) >= 1 {
    _, err := t.CommandRun(install_cmd[0], install_cmd[1:]...)
    if err != nil {
      return err
    }
  }

  return nil
}


var TaskResolverSourceBuildNodeJS = TaskResolver {
  Id:   "source-build-nodejs",
  Name: "source-build-nodejs",
  TaskPrototype: Task { Func: TaskSourceBuildNodeJS },
}


func TaskSourceBuildNodeJS (s *Spec, t *Task) error {
  // Check if build path already exists and emit it, if so
  //
  exists, err := s.PathExists("dist")
  if err != nil { return err }
  if exists {
    s.EmitFileKey("dist", "/")
    return nil
  }

  exists, err = s.PathExists("build")
  if err != nil { return err }
  if exists {
    s.EmitFileKey("build", "/")
    return nil
  }

  // Run build command
  //
  _, err = t.CommandRun("npm", "run", "build")
  if err != nil { return err }

  // Emit output
  // TODO: break this up into its own task
  //
  exists, err = s.PathExists("dist")
  if err != nil { return err }
  if exists {
    s.EmitFileKey("dist", "/")
    return nil
  }

  exists, err = s.PathExists("build")
  if err != nil { return err }
  if exists {
    s.EmitFileKey("build", "/")
    return nil
  }

  return nil
}


func TaskConsumeLinkFiles (s *Spec, task *Task) error {
  source_dir, err := s.RequirePropString("source_dir")
  if err != nil { return err }

  // Remove directory contents, if it exists
  //
  stat, err := os.Stat(source_dir) 
  if stat != nil {
    dirents, err := os.ReadDir(source_dir)
    if err != nil { return err }

    for _, dirent := range dirents {
      path := filepath.Join(source_dir, dirent.Name())
      if err := os.RemoveAll(path); err != nil {
        return err
      }
    }
  }

  for input := range s.Input {
    assets, err := input.Flatten()
    if err != nil { return err }

    for _, asset := range assets {
      task.Println(asset.Url.String())
      if asset.FileSource == "" {
        // TODO: check more specifically if the file can be resolved into a file, i.e: if it has content
        s.EmitAsset(asset)
        continue
      }

      var key string = asset.Url.Path
      if strings.HasPrefix(key, "@emit") {
        key = key[len("@emit"):]
      }

      if exists, _ := s.PathExists(key); exists {
        continue
      }

      var dest string  = filepath.Join(source_dir, key)
      var directory, _ = path.Split(dest)
      if err != nil { return err }

      err = os.MkdirAll(directory, os.ModePerm)
      if err != nil { return err }

      // In the filesystem, either link the asset's source file,
      // or if the asset is moified, copy the new content into
      // this spec's source_dir
      //
      if asset.ContentModified == false {
        err = os.Link(asset.FileSource, dest)
        if err != nil { return err }

        new_asset := s.AnnexAsset(asset)
        new_asset.FileSource = dest
        if err := s.EmitAsset(new_asset); err != nil {
          return err
        }
      } else {
        content, err := asset.GetContentBytes()
        if err != nil { return err }

        new_asset   := s.AnnexAsset(asset)
        writer, err := new_asset.ContentBytesGetWriter()
        if err != nil { return err }

        if _, err := writer.Write(content); err != nil {
          return err
        }

        new_asset.ContentModified = false
        new_asset.FileSource = new_asset.FileDest
        if err := s.EmitAsset(new_asset); err != nil {
          return err
        }
      }
    }
  }

  return nil
}
