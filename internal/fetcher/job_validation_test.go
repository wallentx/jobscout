package fetcher

import "testing"

func TestIsKnownNonJobApplyURLAllowsBuiltInDirectJobDetails(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "regional remote job detail",
			raw:  "https://builtin.com/jobs/remote/nyc/staff-devops-engineer/1234567",
			want: false,
		},
		{
			name: "singular job detail",
			raw:  "https://builtin.com/job/staff-software-engineer-local-environments-team/6315940",
			want: false,
		},
		{
			name: "remote listing",
			raw:  "https://builtin.com/jobs/remote",
			want: true,
		},
		{
			name: "remote listing with page query",
			raw:  "https://builtin.com/jobs/remote?page=2",
			want: true,
		},
		{
			name: "footer role search",
			raw:  "https://builtin.com/jobs/remote/qa-engineer-jobs",
			want: true,
		},
		{
			name: "regional search listing",
			raw:  "https://www.builtinchicago.org/jobs/remote/dev-engineering/senior/search/site-reliability-engineer",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownNonJobApplyURL(tt.raw); got != tt.want {
				t.Fatalf("isKnownNonJobApplyURL(%q) = %t; want %t", tt.raw, got, tt.want)
			}
		})
	}
}
