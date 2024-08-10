package interbuilder

import (
  "bufio"
  "io"
)


func StreamPrefix (r io.ReadCloser, w io.WriteCloser, prefix string) {
  prefix_bytes := []byte(prefix)
  go func () {
    scanner := bufio.NewScanner(r)
    for scanner.Scan() {
      line := append(prefix_bytes, []byte(scanner.Text() + "\n")...)
      w.Write(line)
    }
  }()
}


func IsTruthy (x any) bool {
  if x == nil {
    return false
  }

  switch x := x.(type) {
    case 
      int,  int8,  int16,  int32,  int64,
      uint, uint8, uint16, uint32, uint64,
      float32, float64, complex64, complex128:

      return x != 0

    case string:
      return x != ""

    case bool:
      return x

    case []any:
      return len(x) > 0

    default:
      return true
  }
}


func IsFalsey (x any) bool {
  return ! IsTruthy(x)
}
