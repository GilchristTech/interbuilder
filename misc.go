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
