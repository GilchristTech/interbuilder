package interbuilder

import (
  "fmt"
  "regexp"
  "strings"
  "path"
)


type StringMatcher struct {
  MatchRegexp    *regexp.Regexp
  IsSubstitution bool
  OperandString  string
  OperandFunc    func (string) string
  FlagGlobal     bool
  FlagIgnoreCase bool
}


func (sm *StringMatcher) MatchString (m string) bool {
  if sm.MatchRegexp == nil {
    return true
  }

  return sm.MatchRegexp.MatchString(m)
}


func (sm *StringMatcher) ReplaceString (str string) (string) {
  if sm.MatchRegexp == nil {
    return str
  }

  if ! sm.IsSubstitution {
    return str
  }

  if sm.OperandFunc != nil {
    // sm.OperandFunc is defined, substitute function
    if sm.FlagGlobal {
      return sm.MatchRegexp.ReplaceAllStringFunc(str, sm.OperandFunc)
    } else {
      return RegexpReplaceOneStringFunc(sm.MatchRegexp, str, sm.OperandFunc)
    }
  } else {
    // sm.OperandFunc is not defined, substitute string
    if sm.FlagGlobal {
      return sm.MatchRegexp.ReplaceAllString(str, sm.OperandString)
    } else {
      return RegexpReplaceOneString(sm.MatchRegexp, str, sm.OperandString)
    }
  }
}


type PathTransformation struct {
  Matcher              *StringMatcher
  Replacer             *StringMatcher

  do_normalize         bool
  do_prefix            bool

  Prefix               string
}


/*
  By default, every PathTransformation matches, and this method will return
  true. However, if pt.MatchRegexp is defined, it will be compared against the
  match string. If it is not defined, pt.FindRegexp will be used as a fallback.
*/
func (pt *PathTransformation) MatchString (m string) bool {
  if pt.Matcher != nil {
    return pt.Matcher.MatchString(m)
  }

  if pt.Replacer != nil {
    return pt.Replacer.MatchString(m)
  }

  return true
}


func (pt *PathTransformation) TransformPath (src string) string {
  if pt.Matcher != nil && !pt.Matcher.MatchString(src) {
    return src
  }

  var leading_slash  bool
  var trailing_slash bool

  if len(src) > 0 {
    leading_slash  = (src[0] == '/')
    trailing_slash = (src[len(src)-1] == '/')
  }

  if pt.Replacer != nil {
    src = pt.Replacer.ReplaceString(src)
  }

  if pt.Prefix != "" {
    var prefix_path = pt.Prefix
    if leading_slash {
      prefix_path = "/" + prefix_path
    }
    src = path.Join(prefix_path, src)
  }

  if leading_slash {
    if len(src) == 0 {
      src = "/"
    } else if src[0] != '/' {
      src = "/" + src
    }
  }

  if trailing_slash {
    if len(src) == 0 {
      src = "/"
    } else if src[len(src)-1] != '/' {
      src = src + "/"
    }
  }

  // TODO: assert path validity
  return src
}


func tokenizeMatcherExpression (src string) (tokens []string, err error) {
  if len(src) < 2 {
    return nil, fmt.Errorf("Cannot parse match expression from string \"%s\"", src)
  }

  var char      byte   = src[0]
  var delimiter string = ""

  // Detect delimiter
  //
  switch char {
    case 'm', 's':
      delimiter = string(src[1])

    case '/', '`':
      delimiter = string(char)

    default:
      return nil, fmt.Errorf(
        "Cannot tokenize match expression: unrecognized match mode character: %c", char,
      )
  }

  return strings.Split(src, delimiter), nil
}


func parseMatcherExpression (fields []string) (*StringMatcher, error) {
  if num := len(fields); num < 1 {
    return nil, fmt.Errorf("Error parsing match expression, expected at least one delimited field, got %d", num)
  }

  var mode string = fields[0]

  // If the mode is not defined, infer it based on the number of fields
  //
  if mode == "" {
    switch len(fields) {
      case 3:
        mode = "m"
      case 4:
        mode = "s"
      default:
        return nil, fmt.Errorf("Error parsing match expression, cannot infer the matching mode by the number of delimited fields")
    }
  }

  // For each mode, assert the correct number of delimited fields
  switch mode {
    default:
      return nil, fmt.Errorf("Error parsing match expression, cannot determine matching mode")

    case "m":
      if len(fields) != 3 {
        return nil, fmt.Errorf("Error parsing match expression, incorrect number of delimited fields for match-mode expression, got %d, expected 3", len(fields))
      }
      return parseMatcherMatchExpression(fields[1], fields[2])

    case "s":
      if len(fields) != 4 {
        return nil, fmt.Errorf("Error parsing expression, incorrect number of delimited fields for substitution-mode expression, got %d, expected 4", len(fields))
      }
      return parseMatcherSubstitutionExpression(fields[1], fields[2], fields[3])
  }
}


func parseMatcherExpressionString (src string) (*StringMatcher, error) {
  tokens, err := tokenizeMatcherExpression(src)
  if err != nil { return nil, err }
  return parseMatcherExpression(tokens)
}


func parseMatcherRegexp (rgx_src, flags string) (*StringMatcher, error) {
  var matcher StringMatcher

  for _, flag := range flags {
    switch flag {
      case 'i': matcher.FlagIgnoreCase = true
      case 'g': matcher.FlagGlobal     = true

      default:
        return nil, fmt.Errorf(
          "Error parsing match expression, unrecognized flag: '%c'", flag,
        )
    }
  }

  if matcher.FlagIgnoreCase {
    rgx_src = "(?i)" + rgx_src
  }

  rgx_obj, err := regexp.Compile(rgx_src)
  if err != nil {
    return nil, err
  }

  matcher.MatchRegexp = rgx_obj
  return &matcher, nil
}


func parseMatcherMatchExpression (find, flags string) (*StringMatcher, error) {
  // Assume that len(fields) has been checked in the function calling this one
  return parseMatcherRegexp(find, flags)
}


func parseMatcherMatchExpressionString (src string) (*StringMatcher, error) {
  tokens, err := tokenizeMatcherExpression(src)
  if err != nil { return nil, err }

  if len(tokens) != 3 {
    return nil, fmt.Errorf("Error parsing match expression, expected 3 delimited fields, got %d", len(tokens))
  }

  switch tokens[0] {
    default:
      return nil, fmt.Errorf("Error parsing match expression, expected \"m\" matcher flag, got %s", tokens[0])

    case "m", "":
      return parseMatcherMatchExpression(tokens[1], tokens[2])
  }
}


func parseMatcherSubstitutionExpression (find, replace, flags string) (*StringMatcher, error) {
  // Assume that len(fields) has been checked in the function calling this one
  matcher, err := parseMatcherRegexp(find, flags)
  if err != nil {
    return nil, err
  }

  matcher.IsSubstitution = true
  matcher.OperandString  = replace
  return matcher, nil
}

func parseMatcherSubstitutionExpressionString (src string) (*StringMatcher, error) {
  tokens, err := tokenizeMatcherExpression(src)
  if err != nil { return nil, err }

  if len(tokens) != 4 {
    return nil, fmt.Errorf("Error parsing substitution expression, expected 4 delimited fields, got %d", len(tokens))
  }

  switch tokens[0] {
    default:
      return nil, fmt.Errorf("Error parsing match expression, expected \"s\" matcher flag, got %s", tokens[0])
    case "s", "":
      return parseMatcherSubstitutionExpression(tokens[1], tokens[2], tokens[3])
  }
}


func PathTransformationFromString (src string) (*PathTransformation, error) {
  string_matcher, err := parseMatcherExpressionString(src)
  if err != nil { return nil, err }

  var transformation PathTransformation

  if string_matcher.IsSubstitution {
    transformation.Replacer = string_matcher
  } else {
    transformation.Matcher = string_matcher
  }

  return &transformation, nil
}


func PathTransformationFromProp (prop map[string]any) (*PathTransformation, error) {
  var transformation PathTransformation

  var match_src,   find_src,   replace_src,  prefix_src   string
  var match_found, find_found, replace_found,prefix_found bool

  for key, value := range prop {
    var string_ok bool

    switch key {
      case "match":
        match_src, string_ok = value.(string)
        match_found = true
      case "find":
        find_src, string_ok = value.(string)
        find_found = true
      case "replace":
        replace_src, string_ok = value.(string)
        replace_found = true
      case "prefix":
        prefix_src, string_ok = value.(string)
        prefix_found = true

      default:
        return nil, fmt.Errorf("Error parsing path transformation object, unrecognized property \"%s\"", key)
    }

    if !string_ok && value != nil {
      return nil, fmt.Errorf("Error parsing path transformation object, property \"%s\" expects a string, got %T", key, value)
    }
  }

  if match_found {
    match_matcher, err := parseMatcherExpressionString(match_src)
    if err != nil {
      return nil, fmt.Errorf("Error parsing path transformation match property: %w", err)
    }

    if match_matcher.IsSubstitution {
      if replace_found {
        return nil, fmt.Errorf("Error parsing path transformation, 'match' property is a substitution while a 'replace' property was defined")
      }

      if find_found {
        return nil, fmt.Errorf("Error parsing path transformation, 'match' property is a substitution while a 'find' property was defined")
      }

      transformation.Replacer = match_matcher
    } else {
      // This matcher is not a substitution
      transformation.Matcher = match_matcher
    }
  }

  if replace_found {
    if transformation.Replacer != nil {
      return nil, fmt.Errorf("Error parsing path transformation replace property, a substitution is already defined")
    }

    var replace_matcher *StringMatcher
    var err error

    if find_found {
      replace_matcher, err = parseMatcherMatchExpressionString(find_src)

      if err != nil {
        return nil, fmt.Errorf("Error parsing path transformation find property: %w", err)
      }

      replace_matcher.IsSubstitution = true
      replace_matcher.OperandString  = replace_src
    } else {
      replace_matcher, err = parseMatcherSubstitutionExpressionString(replace_src)

      if err != nil {
        return nil, fmt.Errorf("Error parsing path transformation replace property: %w", err)
      }
    }

    transformation.Replacer = replace_matcher
  } else if find_found {
    return nil, fmt.Errorf("Error parsing path transformation object, 'find' property defined while replace is not defined")
  }

  if prefix_found {
    transformation.do_prefix = true
    transformation.Prefix = prefix_src
  }

  return &transformation, nil
}


func RegexpReplaceOneStringFunc (rgx *regexp.Regexp, find string, replace func (string) string) string {
  var break_replace bool
  return rgx.ReplaceAllStringFunc(find, func (match string) string {
    if break_replace {
      return match
    }
    break_replace = true
    return rgx.ReplaceAllStringFunc(match, replace)
  })
}


func RegexpReplaceOneString (rgx *regexp.Regexp, find, replace string) string {
  var break_replace bool
  return rgx.ReplaceAllStringFunc(find, func (match string) string {
    if break_replace {
      return match
    }
    break_replace = true
    return rgx.ReplaceAllString(match, replace)
  })
}
