// internal/exit/exit_test.go
package exit

import (
	"errors"
	"testing"
)

func TestCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is ok", nil, 0},
		{"invocation is 1", Invocation(errors.New("bad flag")), 1},
		{"invocationf is 1", Invocationf("missing %s", "--epic"), 1},
		{"hard is 2", Hard(errors.New("jj blew up")), 2},
		{"untyped error defaults to hard", errors.New("surprise"), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Code(c.err); got != c.want {
				t.Fatalf("Code(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	base := errors.New("root cause")
	if !errors.Is(Hard(base), base) {
		t.Fatal("Hard should unwrap to its cause")
	}
}
