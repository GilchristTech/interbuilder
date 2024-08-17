package interbuilder

import "testing"
import "time"

var TIMEOUT time.Duration = time.Second


func TestWrapTimeout (t *testing.T, f func ()) {
  timeout := time.After(TIMEOUT)
  done    := make(chan bool)

  go func () {
    f()
    done <- true
  }()

  select {
  case <- timeout:
    t.Fatal("Exceeded timeout")
  case <- done:
    // NO-OP
  }
}


func TestWrapTimeoutError (t *testing.T, f func () error) {
  timeout := time.After(TIMEOUT)
  done    := make(chan bool)

  go func () {
    err := f()
    done <- true

    if err != nil {
      t.Fatal("Function exited with error: ", err)
    }
  }()

  select {
  case <- timeout:
    t.Fatal("Exceeded timeout")
  case <- done:
    // NO-OP
  }
}


