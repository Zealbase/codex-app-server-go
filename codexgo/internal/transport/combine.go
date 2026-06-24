package transport

import "io"

type combinedStream struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (c *combinedStream) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *combinedStream) Write(p []byte) (int, error) { return c.w.Write(p) }

func (c *combinedStream) Close() error {
	var firstErr error
	if err := c.r.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.w.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func Combine(stdin io.ReadCloser, stdout io.WriteCloser) io.ReadWriteCloser {
	return &combinedStream{r: stdin, w: stdout}
}
