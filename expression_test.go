package interbuilder

import (
  "testing"
)

func TestExpressionLexer (t *testing.T) {
  var lexer = NewExpressionLexer(
    `format :  url,text, no-content filter:ext=html,prefix=/site/ transform:s/path\/name/name\/path/g,+"relative/path"`,
  )

  var expected_tokens = []struct {
    value      string
    token_type TokenType
  } {
    { `format`,          TOKEN_IDENTIFIER       },
    { ` `,               TOKEN_WHITESPACE       },
    { `:`,               TOKEN_COLON            },
    { `  `,              TOKEN_WHITESPACE       },
    { `url`,             TOKEN_IDENTIFIER       },
    { `,`,               TOKEN_COMMA            },
    { `text`,            TOKEN_IDENTIFIER       },
    { `,`,               TOKEN_COMMA            },
    { ` `,               TOKEN_WHITESPACE       },
    { `no-content`,      TOKEN_IDENTIFIER       },
    { ` `,               TOKEN_WHITESPACE       },
    { `filter`,          TOKEN_IDENTIFIER       },
    { `:`,               TOKEN_COLON            },
    { `ext`,             TOKEN_IDENTIFIER       },
    { `=`,               TOKEN_EQUALS           },
    { `html`,            TOKEN_IDENTIFIER       },
    { `,`,               TOKEN_COMMA            },
    { `prefix`,          TOKEN_IDENTIFIER       },
    { `=`,               TOKEN_EQUALS           },
    { `/site/`,          TOKEN_PATH_LITERAL     },
    { ` `,               TOKEN_WHITESPACE       },
    { `transform`,       TOKEN_IDENTIFIER       },
    { `:`,               TOKEN_COLON            },
    { `s/path\/name/name\/path/g`, TOKEN_REGEXP },
    { `,`,               TOKEN_COMMA            },
    { `+`,               TOKEN_PLUS             },
    { `"relative/path"`, TOKEN_STRING_LITERAL   },
  }

  tokens, err := lexer.Lex()

  if err != nil {
    t.Error(err)
  }

  if got, expect := len(tokens), len(expected_tokens); got != expect {
    t.Errorf("Expected %d tokens, got %d", expect, got)
  }

  for token_i, token := range tokens {
    if token_i >= len(expected_tokens) {
      break
    }

    var expected = expected_tokens[token_i]

    if got := token.String(); got != expected.value {
      t.Errorf(`Token %d was expected to have value of "%s", got "%s"`, token_i, expected.value, got)
    }
    
    if got := token.TokenType; got != expected.token_type {
      t.Errorf(`Token %d was expected to have a type of %d, got %d"`, token_i, expected.token_type, got)
    }
  }
}


func TestExpressionParserInline (t *testing.T) {
  var lexer = NewExpressionLexer(
    `format :  url,text, no-content filter:ext=html,prefix=/site/ transform:s/path\/name/name\/path/g,+"relative/path"`,
  )

  tokens, err := lexer.Lex()
  if err != nil {
    t.Fatal(err)
  }

  var parser = NewExpressionParser(tokens, true)

  type expectNode struct {
    node_type ExpressionNodeType
    name      string
  }

  var expected_nodes = []expectNode {
    { EXPRESSION_NODE_SECTION,     "format"     },
    { EXPRESSION_NODE_VALUE,       "url"        },
    { EXPRESSION_NODE_VALUE,       "text"       },
    { EXPRESSION_NODE_VALUE,       "no-content" },

    { EXPRESSION_NODE_SECTION,     "filter"     },
    { EXPRESSION_NODE_ASSOCIATION, "ext"        },
    { EXPRESSION_NODE_ASSOCIATION, "prefix"     },

    { EXPRESSION_NODE_SECTION,     "transform"  },
    { EXPRESSION_NODE_VALUE,       "regexp"     },
    { EXPRESSION_NODE_ASSOCIATION, "prefix"     },
  }

  for i := 0; i < 50; i++ {
    if node, err := parser.ParseNext(); err != nil {
      t.Fatal("parsing error:", err)

    } else if node == nil {
      if got, expect := i, len(expected_nodes); expect != got {
        t.Errorf("Expected node length of %d, got %d", expect, got)
      }
      break

    } else {
      t.Log("node:", node.NodeType, node.Name, node.Value.String())

      if i >= len(expected_nodes) {
        t.Errorf(`Extra node at index %d with type %s, name "%s"`, i, node.NodeType, node.Name)

      } else {
        expected_node := expected_nodes[i]

        if got, expect := node.NodeType, expected_node.node_type; got != expect {
          t.Errorf("Node %d expected a node of type %s, got %s", i, got, expect)
        }

        if got, expect := node.Name, expected_node.name; got != expect {
          t.Errorf(`Node %d expected a node with name "%s", got "%s"`, i, got, expect)
        }
      }
    }
  }
}


func TestExpressionParser (t *testing.T) {
  type expectNode struct {
    node_type ExpressionNodeType
    name      string
  }

  var expected_sections = []expectNode {
    { EXPRESSION_NODE_SECTION,     "format"     },
    { EXPRESSION_NODE_SECTION,     "filter"     },
    { EXPRESSION_NODE_SECTION,     "transform"  },
  }

  var expected_children_lists = [][]expectNode {
    { { EXPRESSION_NODE_VALUE,       "url"        },
      { EXPRESSION_NODE_VALUE,       "text"       },
      { EXPRESSION_NODE_VALUE,       "no-content" },
    },

    { { EXPRESSION_NODE_ASSOCIATION, "ext"        },
      { EXPRESSION_NODE_ASSOCIATION, "prefix"     },
    },

    { { EXPRESSION_NODE_VALUE,       "regexp"     },
      { EXPRESSION_NODE_ASSOCIATION, "prefix"     },
    },
  }

  section_nodes, err := ParseExpressionString(
    `format :  url,text, no-content filter:ext=html,prefix=/site/ transform:s/path\/name/name\/path/g,+"relative/path"`,
    false,
  )

  if err != nil {
    t.Fatal("parsing error:", err)
  }

  if got, expect := len(section_nodes), len(expected_sections); got != expect {
    t.Errorf("Expected parser to return %d nodes, returned %d", expect, got)
  }

  for section_i, section_node := range section_nodes {
    if section_i >= len(expected_sections) {
      t.Fatalf(
        `Extra node at index %d with type %s, name "%s"`,
        section_i, section_node.NodeType, section_node.Name,
      )
    }

    expected_section := expected_sections[section_i]

    if got, expect := section_node.NodeType, expected_section.node_type; got != expect {
      t.Errorf("Section node %d expected a node of type %s, got %s", section_i, got, expect)
    }

    if got, expect := section_node.Name, expected_section.name; got != expect {
      t.Errorf(`Section node %d expected a node with name "%s", got "%s"`, section_i, got, expect)
    }

    // Assert that the section node's children match expectation
    //
    expected_children := expected_children_lists[section_i]

    if got, expect := len(section_node.Children), len(expected_children); got != expect {
      t.Errorf("Section %d has %d children, expected %d", section_i, got, expect)
    }

    for child_i, child_node := range section_node.Children {
      expected_child := expected_children[child_i]

      if got, expected := child_node.NodeType, expected_child.node_type; got != expected {
        t.Errorf("Section %d child node %d expected a node of type %s, got %s",
          section_i, child_i, got, expected,
        )
      }

      if got, expected := child_node.Name, expected_child.name; got != expected {
        t.Errorf(`Section %d child node %d expected a node of name "%s", got "%s"`,
          section_i, child_i, got, expected,
        )
      }
    }
  }
}


func TestExpressionLexingCommasBreakTokenization (t *testing.T) {
  var expression_src = `filter:-prefix=/comic/,mime=text/`

  section_nodes, err := ParseExpressionString(expression_src, false)
  if err != nil {
    t.Fatal(err)
  }

  if expect, got := 1, len(section_nodes); got == 0 {
    t.Fatalf("Expected %d top-level nodes, got none", expect)
  } else if got != expect {
    t.Errorf("Expected %d top-level nodes, got %d", expect, got)
  }

  filter_node := section_nodes[0]

  if expect, got := EXPRESSION_NODE_SECTION, filter_node.NodeType; expect != got {
    t.Errorf("Expected top-level node to be a %s, got %s", expect, got)
  }

  if expect, got := 2, len(filter_node.Children); got == 0 {
    t.Fatalf("Expected %d child nodes, got none", expect)
  } else if expect != got {
    t.Errorf("Expected %d child nodes, got %d", expect, got)
  }
}
