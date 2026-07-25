package core

import "testing"

func TestLooksLikeTestPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"testdata/fixture.pem", true},
		{"test/config.py", true},
		{"tests/config.py", true},
		{"fixture/key.pem", true},
		{"fixtures/key.pem", true},
		{"examples/http2/testdata/server.key", true},
		{"src/main.go", false},
		{"contest/config.py", false}, // "test" must be a whole path segment, not a substring
		{"protest/data.py", false},
		{"testdatax/config.py", false},
	}
	for _, c := range cases {
		if got := LooksLikeTestPath(c.path); got != c.want {
			t.Errorf("LooksLikeTestPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
