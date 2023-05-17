package helpers

func TruePtr() *bool {
	a := true
	return &a
}

func FalsePtr() *bool {
	a := false
	return &a
}

func Must[T any](resp T, err error) T {
	if err != nil {
		panic(err)
	}
	return resp
}

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func Index[T comparable](vs []T, t T) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

func Include[T comparable](vs []T, t T) bool {
	return Index(vs, t) >= 0
}

func Any[T comparable](vs []T, f func(T) bool) bool {
	for _, v := range vs {
		if f(v) {
			return true
		}
	}
	return false
}

func All[T comparable](vs []T, f func(T) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

func Filter[T comparable](vs []T, f func(T) bool) []T {
	vsf := make([]T, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func Find[T comparable](vs []T, f func(T) bool) *T {
	for _, v := range vs {
		if f(v) {
			return &v
		}
	}
	return nil
}

func Map[T comparable](vs []T, f func(T) T) []T {
	vsm := make([]T, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}
