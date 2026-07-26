package main

import "testing"

func TestAppVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	for _, test := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "plain release", value: "0.1.0", want: "v0.1.0"},
		{name: "tag release", value: "v1.2.3", want: "v1.2.3"},
		{name: "empty development value", value: "", want: "development"},
	} {
		t.Run(test.name, func(t *testing.T) {
			version = test.value
			if got := appVersion(); got != test.want {
				t.Fatalf("appVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAppTitle(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	version = "v1.2.3"

	if got, want := appTitle(), "tutorio v1.2.3"; got != want {
		t.Fatalf("appTitle() = %q, want %q", got, want)
	}
}
