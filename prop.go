package interbuilder

import (
  "fmt"
  "net/url"
  "reflect"
)


type SpecProps map[string]any


/*
  Generic propert access methods
*/


func (s *Spec) GetProp (key string) (value any, found bool) {
  if value, found := s.Props[key]; found {
    return value, found
  }
  return nil, false
}


func (s *Spec) GetPropType (key string, prop_type reflect.Type) (value any, found, type_ok bool) {
  if value, found := s.Props[key]; found {
    return value, true, reflect.TypeOf(value) == prop_type
  }
  return nil, false, false
}


func (s *Spec) RequireProp (key string) (value any, err error) {
  if value, found := s.Props[key]; found {
    return value, nil
  }

  return nil, fmt.Errorf(
    "Prop \"%s\" required in spec %s",
    key, s.Name,
  )
}


func (s *Spec) RequirePropType (key string, prop_type reflect.Type) (value any, err error) {
  value_any, err := s.RequireProp(key)

  if reflect.TypeOf(value_any) == prop_type {
    return value_any, nil
  }

  return nil, fmt.Errorf(
    "Prop \"%s\" in Spec %s is expected to be a %v, got %T",
    key, s.Name, prop_type, value,
  )
}


func (s *Spec) InheritProp (key string) (val any, found bool) {
  if val, found = s.Props[key] ; found {
    return val, found
  }

  if s.Parent == nil {
    return nil, false
  }

  return s.Parent.InheritProp(key)
}


func (s *Spec) InheritPropType (key string, prop_type reflect.Type) (value any, found, type_ok bool) {
  if value, found = s.Props[key] ; found {
    return value, true, reflect.TypeOf(value) == prop_type
  }

  if s.Parent == nil {
    return nil, false, false
  }

  return s.Parent.InheritPropType(key, prop_type)
}


func (s *Spec) RequireInheritProp (key string) (value any, err error) {
  if value, found := s.InheritProp(key); found {
    return value, nil
  }

  return nil, fmt.Errorf(
    "Inherited prop \"%s\" required in spec %s", 
    key, s.Name,
  )
}


func (s *Spec) RequireInheritPropType (key string, prop_type reflect.Type) (value any, err error) {
  if value, found := s.Props[key]; found {
    if reflect.TypeOf(value) == prop_type {
      return value, nil
    }

    return nil, fmt.Errorf(
      "Inherited prop \"%s\" in Spec %s is expected to be a %v, got %T",
      key, s.Name, prop_type, value,
    )
  }

  return nil, fmt.Errorf(
    "Inherited prop \"%s\" in Spec %s of type %v not found",
    key, s.Name, prop_type,
  )
}

/*
  String prop access methods
*/

func (s *Spec) GetPropString (k string) (value string, ok, found bool) {
  value_any, found := s.Props[k]
  value, ok = value_any.(string)
  return value, ok, found
}
func (s *Spec) InheritPropString (k string) (value string, ok, found bool) {
  value_any, found := s.InheritProp(k)
  value, ok = value_any.(string)
  return value, ok, found
}
func (s *Spec) RequireInheritPropString (k string) (value string, err error) {
  value_any, err := s.RequireInheritPropType(k, reflect.TypeOf(""))
  if err != nil { return }
  return value_any.(string), nil
}
func (s *Spec) RequirePropString (k string) (value string, err error) {
  value_any, err := s.RequirePropType(k, reflect.TypeOf(""))
  if err == nil { return value_any.(string), nil }
  return "", err
}

/*
  Boolean prop access methods
*/

func (s *Spec) GetPropBool (k string) (value bool, ok, found bool) {
  value_any, found := s.Props[k]
  value, ok = value_any.(bool)
  return value, ok, found
}
func (s *Spec) InheritPropBool (k string) (value bool, ok, found bool) {
  value_any, found := s.InheritProp(k)
  value, ok = value_any.(bool)
  return value, ok, found
}
func (s *Spec) RequireInheritPropBool (k string) (value bool, err error) {
  value_any, err := s.RequireInheritPropType(k, reflect.TypeOf(""))
  if err != nil { return }
  return value_any.(bool), nil
}
func (s *Spec) RequirePropBool (k string) (value bool, err error) {
  value_any, err := s.RequirePropType(k, reflect.TypeOf(""))
  if err == nil { return value_any.(bool), nil }
  return false, err
}


/*
  URL prop access methods
*/

func (s *Spec) GetPropUrl (k string) (value *url.URL, ok, found bool) {
  value_any, found := s.Props[k]
  value, ok = value_any.(*url.URL)
  return value, ok, found
}
func (s *Spec) InheritPropUrl (k string) (value *url.URL, ok, found bool) {
  value_any, found := s.InheritProp(k)
  value, ok = value_any.(*url.URL)
  return value, ok, found
}
func (s *Spec) RequireInheritPropUrl (k string) (value *url.URL, err error) {
  value_any, err := s.RequireInheritPropType(k, reflect.TypeOf(""))
  if err != nil { return }
  return value_any.(*url.URL), nil
}
func (s *Spec) RequirePropUrl (k string) (value *url.URL, err error) {
  value_any, err := s.RequirePropType(k, reflect.TypeOf(& url.URL {}))
  if err == nil { return value_any.(*url.URL), nil }
  return nil, err
}

/*
  JSON prop access methods
*/

func (s *Spec) GetPropJson (k string) (value map[string]any, ok, found bool) {
  value_any, found := s.Props[k]
  value, ok = value_any.(map[string]any)
  return value, ok, found
}
func (s *Spec) InheritPropJson (k string) (value map[string]any, ok, found bool) {
  value_any, found := s.InheritProp(k)
  value, ok = value_any.(map[string]any)
  return value, ok, found
}
func (s *Spec) RequireInheritPropJson (k string) (value map[string]any, err error) {
  value_any, err := s.RequireInheritPropType(k, reflect.TypeOf(""))
  if err != nil { return }
  return value_any.(map[string]any), nil
}
func (s *Spec) RequirePropJson (k string) (value map[string]any, err error) {
  value_any, err := s.RequirePropType(k, reflect.TypeOf(& url.URL {}))
  if err == nil { return value_any.(map[string]any), nil }
  return nil, err
}
