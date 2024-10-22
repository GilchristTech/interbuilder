package behaviors

import (
  . "gilchrist.tech/interbuilder"
  "fmt"
  "net/url"
  "path"
  "strings"
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
    subspec.Props = subspec_any.(map[string]any)  // TODO: can panic
    s.AddSubspec(subspec)
    subspecs[i] = subspec
    i++
  }

  for _, subspec := range subspecs {
    if err := subspec.Build(); err != nil {
      return err
    }
  }

  delete(s.Props, "subspecs")
  return nil
}


func BuildSourceURLType (s *Spec) error {
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

  return fmt.Errorf("SpecBuilder type error: source property expects a string or *url.URL, got %T", source_any)
}


func BuildSourceDir (s *Spec) error {
  // TODO: path strings should be coerced into file URLs

  // Inherit source_nest
  // TODO: if found, join to parent's source_nest, unless this one is absolute
  //
  source_nest, ok, found := s.InheritPropString("source_nest")
  if found && !ok {
    return fmt.Errorf("[%s] BuildSourceDir error: Spec property 'source_nest' expects a String, got a %T", s.Name, s.Props["source_nest"])
  }

  if !found {
    source_nest = "."
  }

  // Check whether source_dir existence is defined
  //
  source_dir, ok, found := s.GetPropString("source_dir")

  if found && !ok {
    return fmt.Errorf("[%s] ResolveSourceDir error: Spec property 'source_dir' expects a String, got a %T", s.Name, s.Props["source_dir"])
  }

  if !found {
    source_dir = path.Join(source_nest, s.Name)
    s.Props["source_dir"] = source_dir
  }

  return nil
}


func BuildTaskSourceGitClone (s *Spec) error {
  if s.GetTaskResolverById("source-git-clone") == nil {
    s.AddTaskResolver(&TaskResolverSourceGitClone)
  }

  source, ok, _ := s.GetPropUrl("source")
  if ok == false {
    return nil
  }

  var is_git_scheme bool = source.Scheme == "git"
  var is_github     bool = source.Host == "github.com"
  var is_git_file   bool = strings.HasSuffix(source.Path, ".git") // TODO: suppose this is a URL with form parameters; this would not pick up such a case

  if ( is_git_scheme || is_github || is_git_file ){
    _, err := s.EnqueueUniqueTaskName("git-clone")
    if err != nil { return err }
    _, err  = s.EnqueueUniqueTaskName("source-infer")
    if err != nil { return err }
  }

  return nil
}


func BuildTaskInferSource (s *Spec) error {
  if s.GetTaskResolverById("source-infer-root") == nil {
    s.AddTaskResolver(&TaskResolverInferSource)
  }
  return nil
}


func BuildTasksNodeJS (s *Spec) error {
  if s.GetTaskResolverById("source-install-nodejs") == nil {
    s.AddTaskResolver(&TaskResolverSourceInstallNodeJS)
  }

  if s.GetTaskResolverById("source-build-nodejs") == nil {
    s.AddTaskResolver(&TaskResolverSourceBuildNodeJS)
  }

  return nil
}


func BuildTransform (s *Spec) error {
  transform_any, transform_found := s.GetProp("transform")
  if ! transform_found {
    return nil
  }

  transformations, err := PathTransformationsFromAny(transform_any)
  if err != nil { return err }

  if len(s.PathTransformations) == 0 {
    s.PathTransformations = transformations
  } else {
    s.PathTransformations = append(s.PathTransformations, transformations...)
  }

  return nil
}
