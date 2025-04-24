package result

import (
	"errors"
	"fmt"
	"testing"
)

func TestResultOk(t *testing.T) {
	r := Ok(42)

	if !r.IsOk() {
		t.Error("Expected IsOk to be true")
	}

	if r.IsErr() {
		t.Error("Expected IsErr to be false")
	}

	if r.Value() != 42 {
		t.Errorf("Expected Value to be 42, got %v", r.Value())
	}

	if r.Error() != nil {
		t.Errorf("Expected Error to be nil, got %v", r.Error())
	}

	if r.Unwrap(0) != 42 {
		t.Errorf("Expected Unwrap to be 42, got %v", r.Unwrap(0))
	}
}

func TestResultErr(t *testing.T) {
	testErr := errors.New("test error")
	r := Err[int](testErr)

	if r.IsOk() {
		t.Error("Expected IsOk to be false")
	}

	if !r.IsErr() {
		t.Error("Expected IsErr to be true")
	}

	if r.Error() != testErr {
		t.Errorf("Expected Error to be %v, got %v", testErr, r.Error())
	}

	if r.Unwrap(42) != 42 {
		t.Errorf("Expected Unwrap to be 42, got %v", r.Unwrap(42))
	}
}

func TestResultMap(t *testing.T) {
	r1 := Ok(42)
	r2 := Map(r1, func(i int) string {
		return "value: " + fmt.Sprintf("%d", i)
	})

	if !r2.IsOk() {
		t.Error("Expected mapped result to be Ok")
	}

	testErr := errors.New("test error")
	r3 := Err[int](testErr)
	r4 := Map(r3, func(i int) string {
		return "value: " + fmt.Sprintf("%d", i)
	})

	if !r4.IsErr() {
		t.Error("Expected mapped result to be Err")
	}

	if r4.Error() != testErr {
		t.Errorf("Expected error to be preserved, got %v", r4.Error())
	}
}

func TestResultFlatMap(t *testing.T) {
	r1 := Ok(42)
	r2 := FlatMap(r1, func(i int) Result[string] {
		return Ok("value: " + fmt.Sprintf("%d", i))
	})

	if !r2.IsOk() {
		t.Error("Expected flatmapped result to be Ok")
	}

	r3 := FlatMap(r1, func(i int) Result[string] {
		return Err[string](errors.New("inner error"))
	})

	if !r3.IsErr() {
		t.Error("Expected flatmapped result to be Err")
	}
}

func TestResultAndThen(t *testing.T) {
	r1 := Ok(42)
	success := false

	r2 := r1.AndThen(func(i int) error {
		success = true
		return nil
	})

	if !success {
		t.Error("Expected AndThen to execute function")
	}

	if !r2.IsOk() {
		t.Error("Expected result to remain Ok after AndThen")
	}
}

func TestResultMatch(t *testing.T) {
	r1 := Ok(42)
	okCalled := false
	errCalled := false

	r1.Match(
		func(i int) { okCalled = true },
		func(err error) { errCalled = true },
	)

	if !okCalled {
		t.Error("Expected okCalled to be true")
	}

	if errCalled {
		t.Error("Expected errCalled to be false")
	}

	r2 := Err[int](errors.New("test error"))
	okCalled = false
	errCalled = false

	r2.Match(
		func(i int) { okCalled = true },
		func(err error) { errCalled = true },
	)

	if okCalled {
		t.Error("Expected okCalled to be false")
	}

	if !errCalled {
		t.Error("Expected errCalled to be true")
	}
}
