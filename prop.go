package interbuilder

import (
  "fmt"
  "net/url"
)


type SpecProps map[string]any


func (s *Spec) InheritProp (key string) (val any, found bool) {
  if val, found = s.Props[key] ; found {
    return val, found
  }

  if s.Parent == nil {
    return nil, false
  }

  return s.Parent.InheritProp(key)
}


func (s *Spec) InheritPropString (key string) (value string, ok, found bool) {
  value_any, found := s.InheritProp(key)
  value,     ok     = value_any.(string)
  return value, ok, found
}


func (s *Spec) GetProp (key string) (value any, found bool) {
  if value, found := s.Props[key] ; found {
    return value, found
  }
  return nil, false
}


func (s *Spec) GetPropBool (key string) (value bool, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(bool)
  return value, ok, found
}


func (s *Spec) InheritPropBool (key string) (value bool, ok, found bool) {
  value_any, found := s.InheritProp(key)
  value,     ok     = value_any.(bool)
  return value, ok, found
}


func (s *Spec) GetPropString (key string) (value string, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(string)
  return value, ok, found
}


func (s *Spec) GetPropUrl (key string) (value *url.URL, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(*url.URL)
  return value, ok, found
}


func (s *Spec) GetPropJson (key string) (value map[string]any, ok, found bool) {
  value_any, found := s.Props[key]
  value,     ok     = value_any.(map[string]any)
  return value, ok, found
}


func (s *Spec) RequireProp (key string) (value any, err error) {
  value, found := s.Props[key]

  if !found {
    return nil, fmt.Errorf(
      "Spec %s requires spec prop %s to exist",
      s.Name, key,
    )
  }

  return value, nil
}


func (s *Spec) RequirePropString (key string) (string, error) {
  value_any, err := s.RequireProp(key)
  if err != nil {
    return "", err
  }

  value, ok := value_any.(string)
  if !ok {
    return "", fmt.Errorf(
      "Spec %s requires spec prop %s to be a string, got %T",
      s.Name, key, s.Props[key],
    )
  }

  return value, nil
}


func (t *Task) GetProp (key string) (value any, found bool) {
  if t.Spec == nil {
    return nil, false
  }

  if value, found := t.Spec.Props[key] ; found {
    return value, found
  }
  return nil, false
}


func (t *Task) GetPropString (key string) (value string, ok, found bool) {
  if t.Spec == nil {
    return "", false, false
  }

  value_any, found := t.Spec.Props[key]
  value,     ok     = value_any.(string)
  return value, ok, found
}


func (t *Task) GetPropUrl (key string) (value *url.URL, ok, found bool) {
  if t.Spec == nil {
    return nil, false, false
  }

  value_any, found := t.Spec.Props[key]
  value,     ok     = value_any.(*url.URL)
  return value, ok, found
}


func (t *Task) RequireProp (name string) (value any, err error) {
  if t.Spec == nil {
    var resolver_id = t.ResolverId
    if resolver_id == "" {
      resolver_id = "<no resolver>"
    }

    return nil, fmt.Errorf(
      "Task %s (%s) does not have Spec defined",
      t.Name, resolver_id,
    )
  }

  value, found := t.Spec.Props[name]

  if !found {
    var resolver_id = t.ResolverId
    if resolver_id == "" {
      resolver_id = "<no resolver>"
    }

    return nil, fmt.Errorf(
      "Task %s/%s (%s) requires spec prop %s to exist",
      t.Spec.Name, t.Name, resolver_id, name,
    )
  }

  return value, nil
}


func (t *Task) RequireInheritProp (name string) (value any, err error) {
  if t.Spec == nil {
    return nil, fmt.Errorf(
      "Task %s (%s) does not have Spec defined",
      t.Name, t.ResolverId,
    )
  }

  value, found := t.Spec.InheritProp(name)

  if found == false {
    var resolver_id = t.ResolverId
    if resolver_id == "" {
      resolver_id = "<no resolver>"
    }

    return nil, fmt.Errorf(
      "Task %s/%s (%s) requires inherited spec prop %s to exist",
      t.Spec.Name, t.Name, resolver_id, name,
    )
  }

  return value, nil
}



func (t *Task) RequireInheritPropString (key string) (string, error) {
  value_any, err := t.RequireInheritProp(key)
  if err != nil {
    return "", err
  }

  value, ok := value_any.(string)
  if ok == false {
    var resolver_id = t.ResolverId
    if resolver_id == "" {
      resolver_id = "<no resolver>"
    }

    return "", fmt.Errorf(
      "Task %s/%s (%s) requires inherited spec prop %s to be a string, got %T",
      t.Spec.Name, t.Name, resolver_id, key, t.Spec.Props[key],
    )
  }

  return value, nil
}


func (t *Task) RequirePropString (name string) (value string, err error) {
  value_any, err := t.RequireProp(name)
  if err != nil {
    return "", err
  }

  value, ok := value_any.(string)
  if !ok {
    var resolver_id = t.ResolverId
    if resolver_id == "" {
      resolver_id = "<no resolver>"
    }

    return "", fmt.Errorf(
      "Task %s/%s (%s) requires spec prop %s to be a string, got %T",
      t.Spec.Name, t.Name, resolver_id, name, t.Spec.Props[name],
    )
  }

  return value, nil
}


func (t *Task) RequirePropURL (name string) (value *url.URL, err error) {
  value_any, err := t.RequireProp(name)
  if err != nil {
    return nil, err
  }

  switch value := value_any.(type) {
  case string:
    return url.Parse(value)
  case *url.URL:
    return value, nil
  case url.URL:
    return &value, nil
  }

  var resolver_id = t.ResolverId
  if resolver_id == "" {
    resolver_id = "<no resolver>"
  }


  return nil, fmt.Errorf(
    "Task %s/%s (%s) requires spec prop %s to be a URL string, url.URL, or *url.URL, got %T",
    t.Spec.Name, t.Name, resolver_id, name, t.Spec.Props[name],
  )
}
