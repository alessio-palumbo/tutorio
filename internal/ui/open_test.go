package ui

import "testing"

func TestSourceOpenCommandUsesNativeFileHandlers(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: "open"},
		{goos: "linux", want: "xdg-open"},
		{goos: "windows", want: "rundll32"},
	}
	for _, test := range tests {
		name, args, err := sourceOpenCommand(test.goos, "/tmp/guide.pdf")
		if err != nil {
			t.Fatalf("%s: %v", test.goos, err)
		}
		if name != test.want || len(args) == 0 || args[len(args)-1] != "/tmp/guide.pdf" {
			t.Fatalf("%s: got %q %#v", test.goos, name, args)
		}
	}
}
