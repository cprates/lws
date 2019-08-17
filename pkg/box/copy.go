package box

import (
	"io"
	"os"
)

// CopyFile copies the contents from src to dst using io.Copy.
// If dst does not exist, CopyFile creates it with permissions perm;
// otherwise CopyFile truncates it before writing.
// Taken from: https://github.com/golang/go/issues/8868
func copyFile(dst, src string, perm os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer func() {
		if e := in.Close(); err == nil {
			err = e
		}
	}()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()
	_, err = io.Copy(out, in)
	return
}
