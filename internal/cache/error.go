package cache

type typeAssertionError struct {
	key      string
	expected string
}

func (e *typeAssertionError) Error() string {
	return "cache: value for key '" + e.key + "' is not of type " + e.expected
}
