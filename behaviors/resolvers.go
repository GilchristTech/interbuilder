package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "fmt"
  "net/url"
  "path"
)


func ResolveSubspecs (s *Spec) error {
  subspecs_json, ok, found := s.GetPropJson("subspecs")

  if found == false {
    return nil
  }

  if ok == false {
    return fmt.Errorf("Subspecs prop expects a JSON object, got %T", s.Props["subspecs"])
  }

  subspecs := make([]*Spec, len(subspecs_json))
  i := 0

  for name, subspec_any := range subspecs_json {
    subspec := NewSpec(name, nil)
    subspec.Props = subspec_any.(map[string]any)  // can panic
    s.AddSubspec(subspec)
    subspecs[i] = subspec
    i++
  }

  for _, subspec := range subspecs {
    if err := subspec.Resolve(); err != nil {
      return err
    }
  }

  delete(s.Props, "subspecs")
  return nil
}


func ResolveSourceURLType (s *Spec) error {
  source_any, found := s.Props["source"]

  if found == false {
    return nil
  }

  switch source := source_any.(type) {
  case string:
    // Parse URL strings
    source_url, err := url.Parse(source)
    if err != nil {
      return err
    }
    s.Props["source"] = source_url
    return nil

  case url.URL:
    // Ensure we're working with pointers instead of values
    s.Props["source"] = &source
    return nil

  case *url.URL:
    // Nothing needs to be done. Exit and ensure idempotence
    return nil
  }

  return fmt.Errorf("SpecResolver type error: source property expects a string or *url.URL, got %T", source_any)
}


func ResolveSourceDir (s *Spec) error {
  // TODO: path strings should be coerced into file URLs

  // Inherit source_nest
  // TODO: if found, join to parent's source_nest, unless this one is absolute
  //
  source_nest, ok, found := s.InheritPropString("source_nest")
  if found && !ok {
    return fmt.Errorf("[%s] ResolveBuildDir error: Spec property 'source_nest' expects a String, got a %T", s.Name, s.Props["source_nest"])
  }

  if !found {
    source_nest = "."
  }

  // Check whether source_dir existence is defined
  //
  source_dir, ok, found := s.GetPropString("source_dir")

  if found && !ok {
    return fmt.Errorf("[%s] ResolveBuildDir error: Spec property 'source_dir' expects a String, got a %T", s.Name, s.Props["source_dir"])
  }

  if !found {
    source_dir = path.Join(source_nest, s.Name)
    s.Props["source_dir"] = source_dir
  }

  return nil
}


func ResolveTaskSourceGitClone (s *Spec) error {
  if s.GetTaskResolverById("source-git-clone") == nil {
    s.AddTaskResolver(&TaskResolverSourceGitClone)
  }

  // TODO: check whether the source is a git:// URL

  if _, has_source := s.Props["source"] ; has_source {
    s.EnqueueUniqueTaskName("git-clone")
    s.EnqueueUniqueTaskName("source-infer")
  }

  return nil
}


func ResolveTaskInferSource(s *Spec) error {
  if s.GetTaskResolverById("infer-source-root") == nil {
    s.AddTaskResolver(&TaskResolverInferSource)
  }
  return nil
}


func ResolveTasksNodeJS (s *Spec) error {
  if s.GetTaskResolverById("source-install-nodejs") == nil {
    s.AddTaskResolver(&TaskResolverSourceInstallNodeJS)
  }

  if s.GetTaskResolverById("source-build-nodejs") == nil {
    s.AddTaskResolver(&TaskResolverSourceBuildNodeJS)
  }

  return nil
}
