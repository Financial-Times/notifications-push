package publication

import "errors"

var (
	ErrEmptyPubList = errors.New("empty publication list")
	ErrMoreThanOne  = errors.New("more than one publication in the list")
)
