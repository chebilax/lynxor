package cli

import "testing"

func TestParsePluginArgs(t *testing.T) {
	tests := []struct {
		name    string
		raw     []string
		want    map[string]map[string]string
		wantErr bool
	}{
		{
			name: "single plugin, single arg",
			raw:  []string{"my-plugin:key=value"},
			want: map[string]map[string]string{"my-plugin": {"key": "value"}},
		},
		{
			name: "single plugin, multiple args accumulate",
			raw:  []string{"my-plugin:key=value", "my-plugin:other=thing"},
			want: map[string]map[string]string{"my-plugin": {"key": "value", "other": "thing"}},
		},
		{
			name: "two distinct plugins stay separate",
			raw:  []string{"plugin-a:key=1", "plugin-b:key=2"},
			want: map[string]map[string]string{"plugin-a": {"key": "1"}, "plugin-b": {"key": "2"}},
		},
		{
			name: "value may contain an '=' -- only the first is the separator",
			raw:  []string{"my-plugin:query=a=b=c"},
			want: map[string]map[string]string{"my-plugin": {"query": "a=b=c"}},
		},
		{
			name: "no plugins at all",
			raw:  nil,
			want: map[string]map[string]string{},
		},
		{
			name:    "missing the plugin_name: separator",
			raw:     []string{"key=value"},
			wantErr: true,
		},
		{
			name:    "missing the key=value separator",
			raw:     []string{"my-plugin:novalue"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePluginArgs(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got nil error, want one for %v", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePluginArgs(%v): %v", tt.raw, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for plugin, wantArgs := range tt.want {
				gotArgs, ok := got[plugin]
				if !ok {
					t.Fatalf("missing plugin %q in %v", plugin, got)
				}
				if len(gotArgs) != len(wantArgs) {
					t.Fatalf("plugin %q: got %v, want %v", plugin, gotArgs, wantArgs)
				}
				for k, v := range wantArgs {
					if gotArgs[k] != v {
						t.Errorf("plugin %q key %q: got %q, want %q", plugin, k, gotArgs[k], v)
					}
				}
			}
		})
	}
}
