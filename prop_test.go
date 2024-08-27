package interbuilder

import (
  "testing"
  "reflect"
  "net/url"
)


func testPropType[T comparable] (
  t               *testing.T,
  prop_type       reflect.Type,
  type_name       string,
  local_value     T,
  inherited_value T,
) {
  t.Helper()

  var T_nil T
  if reflect.TypeOf(T_nil) != prop_type {
    t.Fatal("testPropType prop_type argument is not the same as the generic type")
  }

  var key string

  root := NewSpec("root", nil)
  spec := root.AddSubspec( NewSpec("spec", nil) )
  spec.Props["local"]     = local_value
  root.Props["inherited"] = inherited_value

  root_value := reflect.ValueOf(root)

  /*
    Assert prop method existance and signatures
  */

  // GetProp[T]
  //
  var get_prop_method_name = "GetProp" + type_name
  root_getProp_method := root_value.MethodByName(get_prop_method_name)

  if ! root_getProp_method.IsValid() {
    t.Fatalf("Spec method %s does not exist", get_prop_method_name)
  }

  if got, expect := root_getProp_method.Type().NumOut(), 3; got != expect {
    t.Fatalf("Spec method %s has %d outputs, expected %d", get_prop_method_name, got, expect)
  }

  // RequireProp[T]
  //
  var require_prop_method_name = "RequireProp" + type_name
  root_require_getProp_method := root_value.MethodByName(require_prop_method_name)

  if ! root_require_getProp_method.IsValid() {
    t.Fatalf("Spec method %s does not exist", require_prop_method_name)
  }

  if got, expect := root_require_getProp_method.Type().NumOut(), 2; got != expect {
    t.Fatalf("Spec method %s has %d outputs, expected %d", require_prop_method_name, got, expect)
  }

  // InheritProp[T]
  //
  var inherit_prop_method_name = "InheritProp" + type_name
  root_inheritProp_method := root_value.MethodByName(inherit_prop_method_name)

  if ! root_inheritProp_method.IsValid() {
    t.Fatalf("Spec method %s does not exist", inherit_prop_method_name)
  }

  if got, expect := root_inheritProp_method.Type().NumOut(), 3; got != expect {
    t.Fatalf("Spec method %s has %d outputs, expected %d", inherit_prop_method_name, got, expect)
  }

  // RequireInheritProp[T]
  //
  var require_inherit_prop_method_name = "RequireInheritProp" + type_name
  root_require_inheritProp_method := root_value.MethodByName(require_inherit_prop_method_name)

  if ! root_require_inheritProp_method.IsValid() {
    t.Fatalf("Spec method %s does not exist", require_inherit_prop_method_name)
  }

  if got, expect := root_require_inheritProp_method.Type().NumOut(), 2; got != expect {
    t.Fatalf("Spec method %s has %d outputs, expected %d", require_inherit_prop_method_name, got, expect)
  }

  /*
    Test GetProp[T]
  */
  var specGetProp = func (key string) (value T, ok, found bool) {
    method := reflect.ValueOf(spec).MethodByName(get_prop_method_name)
    values := method.Call( []reflect.Value {
      reflect.ValueOf(key),
    })
    return values[0].Interface().(T), values[1].Interface().(bool), values[2].Interface().(bool)
  }

  key = "local" // Valid case

  if value, ok, found := specGetProp(key); found == false {
    t.Errorf("spec.%s(\"%s\") was not found", get_prop_method_name, key)
  } else if ok == false {
    t.Errorf("spec.%s(\"%s\") was not the correct type", get_prop_method_name, key)
  } else if got, expect := value, local_value; got != expect {
    t.Errorf("spec.%s(\"%s\") had the value %v, expected %v", get_prop_method_name, key, got, expect)
  } else {
    dynamic_value, dynamic_ok, dynamic_found := spec.GetPropType(key, prop_type)
    if value != dynamic_value || ok != dynamic_ok || found != dynamic_found {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v, %v) does not return the same values as spec.GetPropType(%s, %s)(%v, %v, %v)",
        get_prop_method_name, key, value, ok, found,
        key, prop_type, dynamic_value, dynamic_ok, dynamic_found,
      )
    }
  }

  key = "doesnt-exist" // Invalid case

  if value, ok, found := specGetProp(key); found == true {
    t.Errorf("spec.%s(\"%s\") was found", get_prop_method_name, key)
  } else if got, expect := value, T_nil; got != expect {
    t.Errorf("spec.%s(\"%s\") had the value %v, expected a nullish value", get_prop_method_name, key, got)
  } else {
    dynamic_value, dynamic_ok, dynamic_found := spec.GetPropType(key, prop_type)
    if value != dynamic_value || ok != dynamic_ok || found != dynamic_found {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v, %v) does not return the same values as spec.GetPropType(%s, %s)(%v, %v, %v)",
        get_prop_method_name, key, value, ok, found,
        key, prop_type, dynamic_value, dynamic_ok, dynamic_found,
      )
      t.Log(value == dynamic_value)
    }
  }

  /*
    Test RequireProp[T]
  */

  var specRequireProp = func (key string) (value T, err error) {
    method := reflect.ValueOf(spec).MethodByName(require_prop_method_name)
    values := method.Call( [] reflect.Value { reflect.ValueOf(key) })

    if values[1].Interface() == nil {
      err = nil
    } else {
      err = values[1].Interface().(error)
    }

    return values[0].Interface().(T), err
  }

  key = "local" // Valid case

  if value, err := specRequireProp(key); err != nil {
    t.Errorf("spec.%s(\"%s\") returned an error: %v", require_prop_method_name, key, err)
  } else if got, expect := value, local_value; got != expect {
    t.Errorf("spec.%s(\"%s\") had the value %v, expected %v", require_prop_method_name, key, got, expect)
  } else {
    dynamic_value, dynamic_err := spec.RequirePropType(key, prop_type)
    if value != dynamic_value || (err == nil) != (dynamic_err == nil) {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v) does not return the same values as spec.RequirePropType(\"%s\", %v)(%v %T, %v)",
        require_prop_method_name,
        key, value,     err,
        key, prop_type, dynamic_value, dynamic_value, dynamic_err,
      )
    }
  }

  key = "doesnt-exist" // Invalid case

  if value, err := specRequireProp(key); err == nil {
    t.Errorf("spec.%s(\"%s\") was expected to return an error, but did not", require_prop_method_name, key)
  } else {
    dynamic_value, dynamic_err := spec.RequirePropType(key, prop_type)
    if value != dynamic_value || (err == nil) != (dynamic_err == nil) {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v) does not return the same values as spec.RequirePropType(\"%s\", %v)(%v %T, %v)",
        require_prop_method_name,
        key, value,     err,
        key, prop_type, dynamic_value, dynamic_value, dynamic_err,
      )
    }
  }

  /*
    Test InheritProp[T]
  */
  var specInheritProp = func (key string) (value T, ok, found bool) {
    method := reflect.ValueOf(spec).MethodByName(inherit_prop_method_name)
    values := method.Call( []reflect.Value {
      reflect.ValueOf(key),
    })
    return values[0].Interface().(T), values[1].Interface().(bool), values[2].Interface().(bool)
  }

  key = "inherited" // Valid case

  if value, ok, found := specInheritProp(key); found == false {
    t.Errorf("spec.%s(\"%s\") was not found", get_prop_method_name, key)
  } else if ok == false {
    t.Errorf("spec.%s(\"%s\") was not the correct type", get_prop_method_name, key)
  } else if got, expect := value, inherited_value; got != expect {
    t.Errorf("spec.%s(\"%s\") had the value %v, expected %v", get_prop_method_name, key, got, expect)
  } else {
    dynamic_value, dynamic_ok, dynamic_found := spec.InheritPropType(key, prop_type)
    if value != dynamic_value || ok != dynamic_ok || found != dynamic_found {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v, %v) does not return the same values as spec.InheritPropType(%s, %s)(%v, %v, %v)",
        get_prop_method_name, key, value, ok, found,
        key, prop_type, dynamic_value, dynamic_ok, dynamic_found,
      )
    }
  }

  key = "doesnt-exist" // Invalid case

  if value, ok, found := specInheritProp(key); found == true {
    t.Errorf("spec.%s(\"%s\") was found", get_prop_method_name, key)
  } else if got, expect := value, T_nil; got != expect {
    t.Errorf("spec.%s(\"%s\") had the value %v, expected a nullish value", get_prop_method_name, key, got)
  } else {
    dynamic_value, dynamic_ok, dynamic_found := spec.InheritPropType(key, prop_type)
    if value != dynamic_value || ok != dynamic_ok || found != dynamic_found {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v, %v) does not return the same values as spec.InheritPropType(%s, %s)(%v, %v, %v)",
        get_prop_method_name, key, value, ok, found,
        key, prop_type, dynamic_value, dynamic_ok, dynamic_found,
      )
      t.Log(value == dynamic_value)
    }
  }

  /*
    Test RequireInheritProp[T]
  */
  var specRequireInheritProp = func (key string) (value T, err error) {
    method := reflect.ValueOf(spec).MethodByName(require_inherit_prop_method_name)
    values := method.Call( [] reflect.Value { reflect.ValueOf(key) })

    if values[1].Interface() == nil {
      err = nil
    } else {
      err = values[1].Interface().(error)
    }

    return values[0].Interface().(T), err
  }

  key = "inherited" // Valid case

  if value, err := specRequireInheritProp(key); err != nil {
    t.Errorf("spec.%s(\"%s\") returned an error: %v", require_inherit_prop_method_name, key, err)
  } else if got, expect := value, inherited_value; got != expect {
    t.Errorf("spec.%s(\"%s\") had the value %v, expected %v", require_inherit_prop_method_name, key, got, expect)
  } else {
    dynamic_value, dynamic_err := spec.RequireInheritPropType(key, prop_type)
    if value != dynamic_value || (err == nil) != (dynamic_err == nil) {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v) does not return the same values as spec.RequireInheritPropType(\"%s\", %v)(%v %T, %v)",
        require_inherit_prop_method_name,
        key, value,     err,
        key, prop_type, dynamic_value, dynamic_value, dynamic_err,
      )
    }
  }

  key = "doesnt-exist" // Invalid case

  if value, err := specRequireInheritProp(key); err == nil {
    t.Errorf("spec.%s(\"%s\") was expected to return an error, but did not", require_inherit_prop_method_name, key)
  } else {
    dynamic_value, dynamic_err := spec.RequireInheritPropType(key, prop_type)
    if value != dynamic_value || (err == nil) != (dynamic_err == nil) {
      t.Errorf(
        "spec.%s(\"%s\")(%v, %v) does not return the same values as spec.RequireInheritPropType(\"%s\", %v)(%v %T, %v)",
        require_inherit_prop_method_name,
        key, value,     err,
        key, prop_type, dynamic_value, dynamic_value, dynamic_err,
      )
    }
  }
}


func TestStringProps (t *testing.T) {
  testPropType(
    t, reflect.TypeOf(""), "String", "value", "inherited-value",
  )
}


func TestBoolProps (t *testing.T) {
  testPropType(
    t, reflect.TypeOf(true), "Bool", true, true,
  )
}


func TestIntProps (t *testing.T) {
  testPropType(
    t, reflect.TypeOf(int(0)), "Int", 1, 1,
  )
}


func TestUrlProps (t *testing.T) {
  var url_local, _     = url.Parse("ib://local")
  var url_inherited, _ = url.Parse("ib://inherited")
  testPropType(
    t, reflect.TypeOf(& url.URL{}), "Url", url_local, url_inherited,
  )
}
