package extcontainer

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-container/pkg/stress"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_stressIo(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   stress.StressOpts
	}{
		{
			name: "default mode",
			config: map[string]interface{}{
				"workers":    1,
				"duration":   1000,
				"path":       "/somepath",
				"percentage": 50.0,
			},
			want: stress.StressOpts{
				HddWorkers: extutil.Ptr(1),
				HddBytes:   "50%",
				IoWorkers:  extutil.Ptr(1),
				TempPath:   "/somepath",
				Timeout:    1000000000,
			},
		},
		{
			name: "flush only",
			config: map[string]interface{}{
				"workers":    1,
				"duration":   1000,
				"path":       "/somepath",
				"percentage": 50.0,
				"mode":       "flush",
			},
			want: stress.StressOpts{
				IoWorkers: extutil.Ptr(1),
				TempPath:  "/somepath",
				Timeout:   1000000000,
			},
		},
		{
			name: "read/write only",
			config: map[string]interface{}{
				"workers":    1,
				"duration":   1000,
				"path":       "/somepath",
				"percentage": 50.0,
				"mode":       "read_write",
			},
			want: stress.StressOpts{
				HddWorkers: extutil.Ptr(1),
				HddBytes:   "50%",
				TempPath:   "/somepath",
				Timeout:    1000000000,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stressIo(action_kit_api.PrepareActionRequestBody{Config: tt.config})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
