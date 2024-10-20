package interbuilder

import (
  "unicode"
  "fmt"
  "strconv"
)


type TokenType int
type ExpressionNodeType int

const (
  TOKEN_INVALID TokenType = iota
  TOKEN_WHITESPACE

  TOKEN_IDENTIFIER
  TOKEN_SEMICOLON
  TOKEN_COMMA
  TOKEN_COLON
  TOKEN_EQUALS
  TOKEN_PLUS

  TOKEN_STRING_LITERAL
  TOKEN_PATH_LITERAL
  TOKEN_REGEXP
)


func (tt TokenType) String () string {
  switch tt {
  default:
    return "token_unknown"

  case TOKEN_INVALID:        return "token_invalid"
  case TOKEN_WHITESPACE:     return "token_whitespace"

  case TOKEN_IDENTIFIER:     return "token_identifier"
  case TOKEN_SEMICOLON:      return "token_semicolon"
  case TOKEN_COMMA:          return "token_comma"
  case TOKEN_COLON:          return "token_colon"
  case TOKEN_EQUALS:         return "token_equals"
  case TOKEN_PLUS:           return "token_plus"

  case TOKEN_STRING_LITERAL: return "token_string_literal"
  case TOKEN_PATH_LITERAL:   return "token_path_literal"
  case TOKEN_REGEXP:         return "token_regexp"
  }
}

func (tt TokenType) IsValue () bool {
  switch tt {
  default:
    return false

  case TOKEN_IDENTIFIER, TOKEN_STRING_LITERAL, TOKEN_PATH_LITERAL, TOKEN_REGEXP:
    return true
  }
}


const (
  EXPRESSION_NODE_INVALID ExpressionNodeType = iota
  EXPRESSION_NODE_SECTION
  EXPRESSION_NODE_NAME
  EXPRESSION_NODE_VALUE
  EXPRESSION_NODE_ASSOCIATION
)


func (nt ExpressionNodeType) String () string {
  switch nt {
    default:
      return "expression_node_unknown"

    case EXPRESSION_NODE_INVALID:     return "expression_node_invalid"
    case EXPRESSION_NODE_SECTION:     return "expression_node_section"
    case EXPRESSION_NODE_NAME:        return "expression_node_name"
    case EXPRESSION_NODE_VALUE:       return "expression_node_value"
    case EXPRESSION_NODE_ASSOCIATION: return "expression_node_association"
  }
}


type ExpressionLexer struct {
  source          []rune
  tokens          []ExpressionToken
  index           int
  line_number     int
  last_index      int
  last_scan_index int

  token *ExpressionToken
}


type ExpressionToken struct {
  Lexeme      []rune
  Offset      int
  LineNumber  int
  TokenType   TokenType

  Value       string
}


func (et *ExpressionToken) EvaluateString () (string, error) {
  if et.TokenType.IsValue() == false {
    return "", fmt.Errorf("Cannot evaluate, token of type %s is not a value", et.TokenType)
  }

  // Parse the value out of string literals
  //
  if et.TokenType == TOKEN_STRING_LITERAL {
    // Use strconv.Unquote to parse string literals, but use a
    // hack to change the quote type to double-quotes if it's
    // something else. This is because Unquote() uses Go's string
    // parsing, which does not support single quotes. 
    // 
    // This assumes that the lexer has correct output, which
    // would mean this token starts and ends with quote
    // characters.

    var value []rune = et.Lexeme[:]

    if value[0] != '"' {
      value[0] = '"'
      value[len(value)-1] = '"'
    }
    return strconv.Unquote(string(value))
  }

  return et.String(), nil
}


type ExpressionParser struct {
  tokens          []*ExpressionToken
  index           int
  last_scan_index int

  // If the parser is inline, do not nest expression nodes into section nodes
  inline bool

  Sections []*ExpressionNode
  section  *ExpressionNode
}


type ExpressionNode struct {
  NodeType  ExpressionNodeType
  Name      string
  Value     ExpressionToken
  Children  []*ExpressionNode
}


type Expression struct {
  name  string
  value ExpressionToken
}


func (tk *ExpressionToken) String () string {
  return string(tk.Lexeme)
}


func NewExpressionLexer (source string) *ExpressionLexer {
  var lexer = & ExpressionLexer {
    source:          []rune(source + "\x00"),
    index:           0,
    last_index:      -1,
    last_scan_index: -1,
    line_number:     1,

    tokens: make([]ExpressionToken, 0, len(source)),
    token:  nil,
  }

  lexer.newToken()
  return lexer
}


func NewExpressionParser (tokens []*ExpressionToken, inline bool) *ExpressionParser {
  var parser = & ExpressionParser {
    tokens: tokens,
    last_scan_index: -1,
    inline: inline,
  }

  return parser
}


func (lx *ExpressionLexer) newToken () {
  var new_token_i = len(lx.tokens)
  lx.tokens = append(lx.tokens, ExpressionToken {
    Offset: lx.index,
    LineNumber: lx.line_number,
  })
  lx.token = & lx.tokens[new_token_i]
}
 

func (lx *ExpressionLexer) peek () rune {
  if lx.index >= len(lx.source) {
    return '\x00'
  }

  var char = lx.source[lx.index]

  // Count line numbers as characters are read
  //
  if lx.index > lx.last_index {
    lx.last_index = lx.index
    if char == '\n' {
      lx.line_number++
    }
  }

  return char
}


func (lx *ExpressionLexer) lookahead (offset int) rune {
  var index = lx.index + offset
  
  if index >= len(lx.source) {
    return '\x00'
  }

  return lx.source[index]
}


func (lx *ExpressionLexer) advance () rune {
  lx.index++
  if length := len(lx.source); lx.index >= length {
    lx.index = length - 1
  }
  return lx.peek()
}


func (lx *ExpressionLexer) finishToken (token_type TokenType) *ExpressionToken {
  var token       = lx.token
  token.Lexeme    = lx.source[token.Offset : lx.index]
  token.TokenType = token_type

  lx.newToken()
  return token
}


func (lx *ExpressionLexer) Tokens () []ExpressionToken {
  return lx.tokens[ : len(lx.tokens) - 1]
}


func (lx *ExpressionLexer) lexIdentifier () *ExpressionToken {
  for char := lx.peek(); char != 0; char = lx.advance() {
    if unicode.IsLetter(char) { continue }
    if unicode.IsDigit(char)  { continue }
    if char == '_'            { continue }
    if char == '-'            { continue }
    if char == '.'            { continue }
    break
  }
  return lx.finishToken(TOKEN_IDENTIFIER)
}


func (lx *ExpressionLexer) lexWhitespace () *ExpressionToken {
  for char := lx.peek(); char != 0; char = lx.advance() {
    if unicode.IsSpace(char) { continue }
    break
  }
  return lx.finishToken(TOKEN_WHITESPACE)
}


func (lx *ExpressionLexer) lexQuotes (quote rune) (*ExpressionToken, error) {
  lx.advance()

  var char_escaped = false

  for char := lx.peek(); char != 0; char = lx.advance() {
    if !char_escaped && char == quote {
      lx.advance()
      return lx.finishToken(TOKEN_STRING_LITERAL), nil
    }

    char_escaped = char == '\\'
  }

  return nil, fmt.Errorf(`Expected a %c, but reached EOF`, quote)
}


func (lx *ExpressionLexer) lexChar (token_type TokenType) *ExpressionToken {
  lx.advance()
  return lx.finishToken(token_type)
}


func (lx *ExpressionLexer) lexRegexp () (*ExpressionToken, error) {
  var num_delimiter_expected int = 2

  if lx.peek() == 's' {
    num_delimiter_expected = 3
  }

  var delimiter rune = lx.advance()
  var num_delimiter_encountered = 1

  if delimiter == 0 {
    return nil, fmt.Errorf(`Expected regular expression to end, but reached EOF`)
  } else if delimiter == '\\' {
    return nil, fmt.Errorf(`Regular expression delimiter cannot be a backslash`)
  }

  // Read until the number of delimiters is reached
  //
  var escape_char = false
  for {
    var char = lx.advance()

    if char == 0 {
      return nil, fmt.Errorf(`Expected regular expression to end, but reached EOF`)
    }

    if escape_char {
      escape_char = false
    } else if char == delimiter {
      num_delimiter_encountered++
      if num_delimiter_encountered >= num_delimiter_expected {
        break
      }
    } else if char == '\\' {
      escape_char = true
    } else {
      escape_char = false
    }
  }

  // Read over flags
  //
  for unicode.IsLetter(lx.advance()) {
    // pass, the loop consumes flag characters
  }

  return lx.finishToken(TOKEN_REGEXP), nil
}


func (lx *ExpressionLexer) lexPathLiteral () *ExpressionToken {
  SCAN:
  for char := lx.peek(); char != 0; char = lx.advance() {
    switch char {
      case '?', '#':
        break SCAN

      case  '/', '-', '.', '_', '~', '!', '$',
            '&', '"', '\'', '(', ')', '*',
            '+', ';', '=', '@', ':':
        // Valid characters, pass

      case ',':
        // Although commas are valid URL characters, in this
        // syntax they separate arguments and should break the
        // token. Instead of a path literal, a URL with a comma
        // should be put in quotes.
        //
        fallthrough

      default:
        if unicode.IsLetter(char) || unicode.IsDigit(char) {
          continue SCAN
        }
        break SCAN
    }
  }

  return lx.finishToken(TOKEN_PATH_LITERAL)
}


func (lx *ExpressionLexer) NextToken () (*ExpressionToken, error) {
  var char rune = lx.peek()

  // Assert that the character advanced. Not doing this would
  // is likely to produce an infinite loop, a state which
  // should be unreachable.
  //
  if lx.index == lx.last_scan_index {
    panic("Lexer scan loop did not advance character read index. This should never happen, and would probably cause an infinite loop. It is likely lx.advance() was not called in the previous scanning iteration.")
  } else {
    lx.last_scan_index = lx.index
  }

  if char == 0 {
    return nil, nil
  }

  if unicode.IsSpace(char) {
    return lx.lexWhitespace(), nil
  }

  switch char {
    case ';': return lx.lexChar(TOKEN_SEMICOLON), nil
    case ',': return lx.lexChar(TOKEN_COMMA),     nil
    case ':': return lx.lexChar(TOKEN_COLON),     nil
    case '=': return lx.lexChar(TOKEN_EQUALS),    nil
    case '+': return lx.lexChar(TOKEN_PLUS),      nil

    case 's', 'm':
      if char := lx.lookahead(1); char == 0 {
        return lx.lexChar(TOKEN_IDENTIFIER), nil
      } else if unicode.IsPunct(char) {
        return lx.lexRegexp()
      } else if unicode.IsSymbol(char) {
        return lx.lexRegexp()
      } else {
        return lx.lexIdentifier(), nil
      }

    case '/':
      return lx.lexPathLiteral(), nil

    case '"', '\'':
      return lx.lexQuotes(char)

    case '-', '_', '.':
      return lx.lexIdentifier(), nil
  }

  if unicode.IsLetter(char) {
    return lx.lexIdentifier(), nil
  }

  return nil, fmt.Errorf("Unexpected character: %c (%d)", char, char)
}

func (lx *ExpressionLexer) Lex () ([]*ExpressionToken, error) {
  var tokens = make([]*ExpressionToken, 0)

  for {
    if token, err := lx.NextToken(); err != nil {
      return tokens, err
    } else if token == nil {
      break
    } else {
      tokens = append(tokens, token)
    }
  }

  return tokens, nil
}


func (nd *ExpressionNode) addChild (child *ExpressionNode) {
  nd.Children = append(nd.Children, child)
}


func (nd *ExpressionNode) addChildToken (node_type ExpressionNodeType, token *ExpressionToken) {
  nd.addChild( & ExpressionNode {
    NodeType: node_type,
    Name:     token.String(),
    Value:    *token,
  })
}


func (pr *ExpressionParser) peek () *ExpressionToken {
  if pr.index >= len(pr.tokens) {
    return nil
  }
  return pr.tokens[pr.index]
}


func (pr *ExpressionParser) advance () *ExpressionToken {
  pr.index++
  return pr.peek()
}


func (pr *ExpressionParser) advancePastWhitespace () *ExpressionToken {
  for token := pr.advance(); token != nil; token = pr.advance() {
    if token.TokenType != TOKEN_WHITESPACE {
      return token
    }
  }
  return nil
}


func (pr *ExpressionParser) skipWhitespace () *ExpressionToken {
  for token := pr.peek(); token != nil; token = pr.advance() {
    if token.TokenType != TOKEN_WHITESPACE {
      return token
    }
  }
  return nil
}


func (pr *ExpressionParser) parseValue () (*ExpressionNode, error) {
  pr.skipWhitespace()
  var token = pr.peek()

  if token == nil {
    return nil, fmt.Errorf("Expected a value")
  }

  var node = & ExpressionNode {
    NodeType: EXPRESSION_NODE_VALUE,
  }

  switch token.TokenType {
  case 0:
    return nil, fmt.Errorf("Expected a value")
  case TOKEN_IDENTIFIER:
    node.Name = "identifer"
  case TOKEN_PATH_LITERAL:
    node.Name = "path"
  case TOKEN_REGEXP:
    node.Name = "regexp"
  case TOKEN_STRING_LITERAL:
    node.Name = "string"
  }
  node.Value = *token

  pr.advance()
  return node, nil
}

func (pr *ExpressionParser) parseFromIdentifier () (*ExpressionNode, error) {
  var node = & ExpressionNode {}
  var identifier *ExpressionToken = pr.peek()
  var next = pr.advancePastWhitespace()

  if next == nil {
    // Skip switch block which expects next to not to be null
    goto IDENTIFIER_IS_VALUE
  }

  if next.TokenType.IsValue() {
    goto IDENTIFIER_IS_VALUE
  }

  switch next.TokenType {
    default:
      return nil, fmt.Errorf(
        `Unexpected token type %s after identifier with value of "%s"`,
        next.TokenType, identifier,
      )

    case TOKEN_COLON:
      // this is a section designator
      node.NodeType = EXPRESSION_NODE_SECTION
      node.Name     = identifier.String()
      node.Value    = *identifier
      pr.advance()
      return node, nil

    case TOKEN_EQUALS:
      // this is a key=value pair
      node.NodeType = EXPRESSION_NODE_ASSOCIATION
      node.Name     = identifier.String()
      pr.advance()

      if value_node, err := pr.parseValue(); err != nil {
        return nil, fmt.Errorf("Cannot parse key=value pair: %w", err)
      } else {
        node.addChildToken(EXPRESSION_NODE_NAME, identifier)
        node.addChild(value_node)
        pr.advance()
        return node, nil
      }

    case TOKEN_COMMA:
      pr.advance()
      goto IDENTIFIER_IS_VALUE

    case TOKEN_IDENTIFIER, TOKEN_PLUS:
      goto IDENTIFIER_IS_VALUE
  }
  panic("This code should be unreachable. Each case in the switch above should result in an early exit, which either returns from the function or jumps over this statement, indicating the cases need to be more robust.")

  IDENTIFIER_IS_VALUE: 
  node.NodeType = EXPRESSION_NODE_VALUE
  node.Name     = identifier.String()
  node.Value    = *identifier
  return node, nil
}


func (pr *ExpressionParser) parsePlus () (*ExpressionNode, error) {
  var node = & ExpressionNode {
    NodeType: EXPRESSION_NODE_ASSOCIATION, 
  }

  var operator_token = pr.peek()
  node.Name = "prefix"
  node.addChildToken(EXPRESSION_NODE_NAME, operator_token)
  pr.advancePastWhitespace()

  if value_node, err := pr.parseValue(); err != nil {
    return nil, fmt.Errorf("Cannot parse plus, expected value and got error: %w", err)
  } else {
    node.Value = value_node.Value
    node.addChild(value_node)
  }

  return node, nil
}


func (pr *ExpressionParser) Parse () ([]*ExpressionNode, error) {
  var sections = make([]*ExpressionNode, 0)

  for {
    if section, err := pr.ParseNext(); err != nil {
      return nil, err
    } else if section == nil {
      return sections, nil
    } else {
      sections = append(sections, section)
    }
  }
}


func (pr *ExpressionParser) ParseNext () (*ExpressionNode, error) {
  TOKEN_SCAN_LOOP:
  for token := pr.peek(); token != nil; token = pr.peek() {
    if pr.last_scan_index == pr.index {
      panic("Parser scan loop did not advance token read index. This should never happen, and would probably cause an infinite loop. It is likely pr.advance() was not called in the previous parsing iteration.")
    } else {
      pr.last_scan_index = pr.index
    }

    var node *ExpressionNode
    var err  error

    switch token.TokenType {
      case TOKEN_WHITESPACE, TOKEN_COMMA:
        pr.advance()
        continue TOKEN_SCAN_LOOP

      case TOKEN_PLUS:
        if node, err = pr.parsePlus(); err != nil {
          return nil, err
        }

      case TOKEN_IDENTIFIER:
        if node, err = pr.parseFromIdentifier(); err != nil {
          return nil, err
        }

      case TOKEN_STRING_LITERAL, TOKEN_PATH_LITERAL, TOKEN_REGEXP:
        if node, err = pr.parseValue(); err != nil {
          return nil, err
        }

      default:
        return nil, fmt.Errorf(`Unexpected token: "%s"`, token)
    }

    if pr.inline == true {
      return node, nil
    }

    var previous_section = pr.section

    if node == nil {
      pr.section = nil
      return previous_section, nil
    }

    // Nest expression nodes into sections
    //
    if node.NodeType == EXPRESSION_NODE_SECTION {
      pr.section = node

      if previous_section != nil {
        return previous_section, nil
      } else {
        continue TOKEN_SCAN_LOOP
      }

    } else if pr.section == nil {
      pr.section = & ExpressionNode {
        NodeType: EXPRESSION_NODE_SECTION,
        Children: make([]*ExpressionNode, 0),
      }
    }
    pr.section.addChild(node)
  }

  // There are no more tokens to parse, but there may be one last
  // section to return. Either return the last section, or return
  // nothing (indicating that parsing is done)

  if final_section := pr.section; final_section != nil {
    pr.section = nil
    return final_section, nil
  }

  return nil, nil
}


func ParseExpressionString (expression string, inline bool) ([]*ExpressionNode, error) {
  var lexer = NewExpressionLexer(expression)

  tokens, err := lexer.Lex()
  if err != nil {
    return nil, err
  }

  var parser = NewExpressionParser(tokens, inline)
  return parser.Parse()
}
