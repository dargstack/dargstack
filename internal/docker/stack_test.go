package docker

import (
	"errors"
	"testing"
)

func TestIsComposePluginMissing(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "docker cli rejects compose as unknown",
			err: errors.New(`docker compose -f - config --quiet: exit status 1
docker: 'compose' is not a docker command.`),
			want: true,
		},
		{
			name: "generic unknown command wording",
			err:  errors.New("docker: unknown command: docker compose"),
			want: true,
		},
		{
			name: "actual config error is not a missing plugin",
			err: errors.New(`docker compose -f - config --quiet: exit status 15
services.web.volumes must be a list`),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isComposePluginMissing(tc.err); got != tc.want {
				t.Errorf("isComposePluginMissing(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
