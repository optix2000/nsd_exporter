package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {

	testCases := []struct {
		name      string
		path      string
		expectErr error
	}{
		{"embed", "", nil},
		{"file", "config.yaml", nil},
		{"no-file", "not-existing.yaml", os.ErrNotExist},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			mc := new(MetricConfig)
			err := LoadConfig(tc.path, mc)

			if tc.expectErr != nil {
				assert.ErrorIs(err, tc.expectErr)
			} else {
				assert.NoError(err)
				assert.Contains(mc.Metrics, "num.queries")
			}
		})
	}
}
