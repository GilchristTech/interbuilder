# Interbuilder Specs

Specs, short for "build specification" are the primary
organizational unit in Interbuilder. They start with JSON objects
and are ran through a series of functions which eventually
translate them into a task queue.

This document concerns itself with JSON structures Interbuilder
recognizes, and also the interpretation of certain values, such
as strings in path transformations.

## Spec JSON Properties (Props)

Build specifications can be defined in JSON or YAML. Interbuilder
uses these to build a concurrent process and file pipeline prior
to running. 

### `example.spec.json`
```json
{
  "source_nest": "build",

  "subspecs": {
    "site-a": {
      "source": "git://example.com/my-nodejs-static-site",
      "transform": {
        "prefix": "join-this-onto-all-url-paths"
      }
    },
    "site-b": {
      "source": "git://example.com/other-nodejs-static-site",
      "transform": {
        "prefix": "this-site-has-its-urls-on-a-different-path"
      }
    },
  }
}
```

Alternatively, you may find it cleaner and easier to write this in YAML:
### `example.spec.yaml`
```YAML
source_nest: build

subspecs:
  site-a:
    source: "git://example.com/my-nodejs-static-site"
    transform:
      prefix: join-this-onto-all-url-paths

  site-b:
    source: "git://example.com/other-nodejs-static-site"
    transform:
      prefix: this-site-has-its-urls-on-a-different-path
```

When ran with `interbuilder run example.spec.json` (the spec file
format is inferred by the file extension), this would do
the following, in parallel:
  * Clone two git repositories*
  * Detect they are NodeJS packages
  * Install NodeJS packages*
  * Run each packages' build command
  * Collect the build output files, and manipulate their file
    trees.
  * Combine the two file trees together, forming a new static
    site.

&ast;Tasks which download use a mutex lock so they will run in series.

## Prop reference

The following properties are directly used and recognized by the
Interbuilder core:

* `source_dir`: A working directory for system commands and
                relative file paths.

* `quiet`:      Prevent this spec and its children from writing
                to STDOUT.

Interbuilder's default behavior set recognizes the following
properties:

* `subspecs`: A dictionary of spec names to spec prop objects.
              Used to construct a nested spec pipeline.

* `source`
* `source_nest`
* `install_cmd`

* `transform`: Perform a transformation on the URLs of assets.
  This can be used to rearrange static site file structures.
  These transformations also get applied to URL paths inside HTML
  and CSS content. 

  A transformation is expressed as a JSON object with the
  following attributes:
  - `prefix`: Join a URL path to the beginning of each matching URL.
  - `match`
  - `find`
  - `replace`

## Interbuilder URLs (ib://)

Internally, Interbuilder uses URLs with the `ib://` scheme to
denote resources.
