package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_LevelParsing(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"garbage", slog.LevelInfo},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := parseLevel(tc.in)
			if got != tc.want {
				t.Fatalf("parseLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNew_DevelopmentEmitsText(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(EnvDevelopment, "debug", &buf)
	l.Info("hello", "k", "v")
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "k=v") {
		t.Fatalf("dev output missing key=value: %q", out)
	}
	if strings.Contains(out, `"msg"`) {
		t.Fatalf("dev output looks like JSON: %q", out)
	}
}

func TestNew_ProductionEmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(EnvProduction, "info", &buf)
	l.Info("hello", "k", "v")
	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Fatalf("prod output missing JSON msg: %q", out)
	}
	if !strings.Contains(out, `"k":"v"`) {
		t.Fatalf("prod output missing JSON attr: %q", out)
	}
}

func TestContext_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(EnvProduction, "info", &buf)
	ctx := WithContext(context.Background(), l)
	got := FromContext(ctx)
	if got != l {
		t.Fatal("expected the logger we put in")
	}
}

func TestFromContext_NilSafe(t *testing.T) {
	got := FromContext(nil)
	if got == nil {
		t.Fatal("FromContext(nil) returned nil; expected default")
	}
}

func TestWith_AttachesAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := NewWithWriter(EnvProduction, "info", &buf)
	ctx := WithContext(context.Background(), base)
	With(ctx, "request_id", "abc-123").Info("ok")
	if !strings.Contains(buf.String(), `"request_id":"abc-123"`) {
		t.Fatalf("missing attached attr: %q", buf.String())
	}
}
