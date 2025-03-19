/*
  This example demonstrates using the Interbuilder Go module for
  parallel task execution, where one task emits an Asset, and
  another receives and processes it. It's a minimal "Hello World"
  program to introduce the concept of Assets moving between
  Interbuilder tasks running in parallel.

  The "produce-hello" Task in the "producer" Spec creates an
  Asset (`hello.txt`) and emits it.  Meanwhile, the
  "consume-hello" Task in the "root" Spec, waits for the Asset,
  verifies its content, and prints it.

  To run this example from within the examples directory, execute:
    go run hello-emit.go

  Here is its expected output:
    [root] Running
    [root] task: consume-hello
    [root/consume-hello] Waiting for @emit/hello.txt...
    [produce] Running
    [produce] task: produce-hello
    [produce/produce-hello] Writing hello.txt
    [produce/produce-hello] Sending hello...
    [produce/produce-hello] Hello sent! Task is done.
    [produce] Exit
    [root/consume-hello] Received hello! Content:
    [root/consume-hello] Hello, world!
    [root/consume-hello] Task done
    [root] Exit

    ib://root
      Tasks:
        > consume-hello ()
      Subspecs:
        ib://produce
          Tasks:
            > produce-hello () [assets: 1]

*/

package main

import (
  "fmt"
  ib "github.com/GilchristTech/interbuilder"
)

func main () {
  var root_spec    *ib.Spec = ib.NewSpec("root",    nil)
  var produce_spec *ib.Spec = ib.NewSpec("produce", nil)
  root_spec.AddSubspec(produce_spec)

  // The producer spec, which is a child of the root spec, will
  // produce an asset, hello.txt, and emit it to its parent.

  produce_spec.EnqueueTaskFunc("produce-hello", func (sp *ib.Spec, tk *ib.Task) error {
    tk.Println("Writing hello.txt")
    var asset *ib.Asset = sp.MakeAsset("hello.txt")
    asset.SetContentBytes([]byte("Hello, world!"))

    tk.Println("Sending hello...")
    if err := tk.EmitAsset(asset); err != nil {
      return err
    }

    tk.Println("Hello sent! Task is done.")
    return nil
  })

  // The root spec will run in parallel with its
  // children (produce_spec), and await assets until there are
  // none left to receive.

  root_spec.EnqueueTaskFunc("consume-hello", func (sp *ib.Spec, tk *ib.Task) error {
    tk.Println("Waiting for @emit/hello.txt...")
    var got_hello bool = false

    for {
      asset, await_err := tk.AwaitInputAssetNext()

      if await_err != nil {
        return await_err
      } else if asset == nil {
        break
      }

      if asset_path := asset.Url.Path; asset_path != "@emit/hello.txt" {
        return fmt.Errorf(
          "Expected to receive an asset with the path @emit/hello.txt, got %s",
          asset_path
        )
      }

      got_hello = true

      if content_bytes, err := asset.GetContentBytes(); err != nil {
        return err
      } else {
        tk.Println("Received hello! Content:")
        tk.Println(string(content_bytes))
      }
    }

    if ! got_hello {
      return fmt.Errorf("Never received hello.txt")
    }

    tk.Println("Task done")
    return nil
  })

  if err := root_spec.Run(); err != nil {
    fmt.Printf("Error when running root spec:\n%v\n", err)
  }

  fmt.Println()
  ib.PrintSpec(root_spec)
}
