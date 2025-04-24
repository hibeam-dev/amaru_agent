package result

type Result[T any] struct {
	value T
	err   error
	isOk  bool
}

func Ok[T any](value T) Result[T] {
	return Result[T]{
		value: value,
		isOk:  true,
	}
}

func Err[T any](err error) Result[T] {
	var zero T
	return Result[T]{
		value: zero,
		err:   err,
		isOk:  false,
	}
}

func (r Result[T]) IsOk() bool {
	return r.isOk
}

func (r Result[T]) IsErr() bool {
	return !r.isOk
}

func (r Result[T]) Value() T {
	if !r.isOk {
		panic("attempted to get Value from an error Result")
	}
	return r.value
}

func (r Result[T]) Error() error {
	if r.isOk {
		return nil
	}
	return r.err
}

func (r Result[T]) Unwrap(defaultValue T) T {
	if r.isOk {
		return r.value
	}
	return defaultValue
}

func Map[T, U any](r Result[T], fn func(T) U) Result[U] {
	if r.isOk {
		return Ok(fn(r.value))
	}
	return Err[U](r.err)
}

func FlatMap[T, U any](r Result[T], fn func(T) Result[U]) Result[U] {
	if r.isOk {
		return fn(r.value)
	}
	return Err[U](r.err)
}

func (r Result[T]) AndThen(fn func(T) error) Result[T] {
	if !r.isOk {
		return r
	}

	if err := fn(r.value); err != nil {
		return Err[T](err)
	}
	return r
}

func (r Result[T]) Match(onOk func(T), onErr func(error)) {
	if r.isOk {
		onOk(r.value)
	} else {
		onErr(r.err)
	}
}
