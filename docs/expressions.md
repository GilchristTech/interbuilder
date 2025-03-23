# Expressions

Interbuilder spec files and CLI arguments have an expression
syntax. Spec files are able to define these in JSON, but it may
be shorter to use strings containing these expressions, and for
quick pipelines it can be easier to write out expressions in the
command-line arguments. The syntax is written to not conflict
with shell-specific syntax, in the hopes that CLI arguments can
be unquoted, and so that expressions can be more easily nested
inside JSON strings without quote-escaping.

## General syntax

## Expression types

### Format expressions

### Filter expressions

### Transformation expressions

(not yet implemented)
