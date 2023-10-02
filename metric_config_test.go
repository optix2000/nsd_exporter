package main

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
		{"file", "config/config.yaml", nil},
		{"no-file", "config/not-existing.yaml", os.ErrNotExist},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			mc := new(metricConfig)
			err := loadConfig(tc.path, mc)

			if tc.expectErr != nil {
				assert.ErrorIs(err, tc.expectErr)
			} else {
				assert.NoError(err)
				assert.Contains(mc.Metrics, "num.queries")
			}
		})
	}
}
