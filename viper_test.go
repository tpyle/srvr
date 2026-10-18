package srvr

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      any
		want    time.Duration
		wantErr bool
	}{
		{15, 15 * time.Second, false},
		{int64(2), 2 * time.Second, false},
		{0, 0, false},
		{1.5, 1500 * time.Millisecond, false},
		{float32(0.25), 250 * time.Millisecond, false},
		{"15", 15 * time.Second, false},
		{" 0.5 ", 500 * time.Millisecond, false},
		{"1m30s", 90 * time.Second, false},
		{"500ms", 500 * time.Millisecond, false},
		{"-1", -time.Second, false},
		{3 * time.Second, 3 * time.Second, false},
		{"soon", 0, true},
		{"", 0, true},
		{[]int{1}, 0, true},
	}
	for _, tt := range tests {
		got, err := parseDuration(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDuration(%#v) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseDuration(%#v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestWithViperKeepsSubSecondOptions(t *testing.T) {
	opts := defaultOptions()
	WithReadTimeout(500 * time.Millisecond)(opts)
	WithShutdownTimeout(1500 * time.Millisecond)(opts)
	WithViper(viper.New())(opts)

	if opts.readTimeout != 500*time.Millisecond {
		t.Errorf("readTimeout = %v, want 500ms", opts.readTimeout)
	}
	if opts.shutdownTimeout != 1500*time.Millisecond {
		t.Errorf("shutdownTimeout = %v, want 1.5s", opts.shutdownTimeout)
	}
}

func TestWithViperDurationFormats(t *testing.T) {
	v := viper.New()
	v.Set("srvr.readTimeout", "1m30s")
	v.Set("srvr.writeTimeout", "20") // environment variables arrive as strings
	v.Set("srvr.idleTimeout", 0.5)
	v.Set("srvr.readHeaderTimeout", 3)
	v.Set("srvr.shutdownTimeout", "45s")

	opts := defaultOptions()
	WithViper(v)(opts)

	for name, got := range map[string]time.Duration{
		"readTimeout":       opts.readTimeout,
		"writeTimeout":      opts.writeTimeout,
		"idleTimeout":       opts.idleTimeout,
		"readHeaderTimeout": opts.readHeaderTimeout,
		"shutdownTimeout":   opts.shutdownTimeout,
	} {
		want := map[string]time.Duration{
			"readTimeout":       90 * time.Second,
			"writeTimeout":      20 * time.Second,
			"idleTimeout":       500 * time.Millisecond,
			"readHeaderTimeout": 3 * time.Second,
			"shutdownTimeout":   45 * time.Second,
		}[name]
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestWithViperInvalidDurationIsIgnored(t *testing.T) {
	l, buf := newTestLogger()
	v := viper.New()
	v.Set("api.srvr.readTimeout", "soon")

	opts := defaultOptions()
	WithLogger(l)(opts)
	WithViperPrefix(v, "api.")(opts)

	if opts.readTimeout != 5*time.Second {
		t.Errorf("readTimeout = %v, want the 5s default", opts.readTimeout)
	}
	rec := findRecord(logRecords(t, buf), "Ignoring invalid duration in configuration")
	if rec == nil || rec["key"] != "api.srvr.readTimeout" {
		t.Errorf("expected a warning naming the key, got %q", buf.String())
	}
}

func TestWithViperShutdownTimeoutDefault(t *testing.T) {
	v := viper.New()
	opts := defaultOptions()
	WithViper(v)(opts)
	if got := v.GetFloat64("srvr.shutdownTimeout"); got != 10 {
		t.Errorf("viper default shutdownTimeout = %v, want 10", got)
	}
	if opts.shutdownTimeout != 10*time.Second {
		t.Errorf("shutdownTimeout = %v, want 10s", opts.shutdownTimeout)
	}
}

func TestOptionsAfterViperOverride(t *testing.T) {
	v := viper.New()
	v.Set("srvr.externalPort", 9000)
	v.Set("srvr.readTimeout", 30)

	opts := defaultOptions()
	WithExternalPort(3000)(opts) // before: overridden by the set key
	WithViper(v)(opts)
	WithReadTimeout(time.Minute)(opts) // after: wins

	if opts.externalPort != 9000 {
		t.Errorf("externalPort = %d, want 9000 from Viper", opts.externalPort)
	}
	if opts.readTimeout != time.Minute {
		t.Errorf("readTimeout = %v, want 1m from the later option", opts.readTimeout)
	}
}

func TestGlobalViperIgnoredWithoutOption(t *testing.T) {
	viper.Set("srvr.externalPort", 4242)
	t.Cleanup(viper.Reset)

	s := Create()
	if s.options.externalPort != 8080 {
		t.Errorf("externalPort = %d; the global Viper must not be read without WithViper", s.options.externalPort)
	}
}
