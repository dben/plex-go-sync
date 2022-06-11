package filesystem

import "os"

type removeFS interface {
	Remove(string) error
}

type osImpl struct{}

func (r *osImpl) Remove(p string) error {
	return os.Remove(p)
}
