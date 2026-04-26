package httpx

import (
	"errors"
	"net/http"
	"testing"
)

func TestError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "no-cause",
			err:  New(CodeBadRequest, "missing field"),
			want: "BAD_REQUEST: missing field",
		},
		{
			name: "with-cause",
			err:  Wrap(CodeInternal, "db down", errors.New("connection refused")),
			want: "INTERNAL: db down: connection refused",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("root")
	e := Wrap(CodeInternal, "wrapped", cause)
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is should find the wrapped cause")
	}
}

func TestAs_FindsTypedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"typed", New(CodeNotFound, "x"), true},
		{"wrapped", errors.Join(errors.New("ctx"), New(CodeNotFound, "x")), true},
		{"plain", errors.New("just a string"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := As(tc.err) != nil
			if got != tc.want {
				t.Fatalf("As() found=%v, want=%v", got, tc.want)
			}
		})
	}
}

func TestCode_HTTPStatus(t *testing.T) {
	cases := []struct {
		code Code
		want int
	}{
		{CodeBadRequest, http.StatusBadRequest},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeNotFound, http.StatusNotFound},
		{CodeConflict, http.StatusConflict},
		{CodeRateLimited, http.StatusTooManyRequests},
		{CodeUnavailable, http.StatusServiceUnavailable},
		{CodeInternal, http.StatusInternalServerError},
		{Code("UNKNOWN"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			if got := tc.code.HTTPStatus(); got != tc.want {
				t.Fatalf("HTTPStatus(%s) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}
