package batch

import "testing"

func TestParseList(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    []ChartRef
		wantErr bool
	}{
		{
			name: "names and pinned versions",
			yaml: "charts:\n  - chart: bitnami/nginx\n    version: 15.1.0\n  - chart: prometheus-community/kube-prometheus-stack\n",
			want: []ChartRef{
				{Repo: "bitnami", Name: "nginx", Version: "15.1.0"},
				{Repo: "prometheus-community", Name: "kube-prometheus-stack"},
			},
		},
		{
			name: "trims whitespace",
			yaml: "charts:\n  - chart: \"  bitnami / nginx  \"\n    version: \" 1.0 \"\n",
			want: []ChartRef{{Repo: "bitnami", Name: "nginx", Version: "1.0"}},
		},
		{name: "empty list", yaml: "charts: []\n", wantErr: true},
		{name: "no charts key", yaml: "other: 1\n", wantErr: true},
		{name: "missing slash", yaml: "charts:\n  - chart: nginx\n", wantErr: true},
		{name: "empty repo", yaml: "charts:\n  - chart: /nginx\n", wantErr: true},
		{name: "empty name", yaml: "charts:\n  - chart: bitnami/\n", wantErr: true},
		{name: "extra slash", yaml: "charts:\n  - chart: bitnami/nginx/extra\n", wantErr: true},
		{name: "malformed yaml", yaml: "charts: [\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseList([]byte(tt.yaml))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d refs, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ref %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
