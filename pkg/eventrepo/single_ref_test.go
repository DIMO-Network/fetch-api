package eventrepo_test

import (
	"testing"

	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/stretchr/testify/assert"
)

func TestIsSingleEventRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		// Positive cases
		{
			name:     "date-prefixed single event",
			key:      "2024/01/15/single-abc123.json",
			expected: true,
		},
		{
			name:     "no path prefix",
			key:      "single-abc123.json",
			expected: true,
		},
		{
			name:     "deep path prefix",
			key:      "did:eth:153:0x1234/events/single-uuid.json",
			expected: true,
		},
		{
			name:     "single- prefix with UUID-style name",
			key:      "2025/06/01/single-f47ac10b-58cc-4372-a567-0e02b2c3d479.json",
			expected: true,
		},
		// Negative cases
		{
			name:     "parquet batch ref",
			key:      "batch-abc123.parquet#42",
			expected: false,
		},
		{
			name:     "legacy JSON key (DID path)",
			key:      "did:eth:153:0x1234/2024-01-15T10:00:00Z",
			expected: false,
		},
		{
			name:     "single- prefix but wrong extension",
			key:      "single-abc123.parquet",
			expected: false,
		},
		{
			name:     "single- prefix but no extension",
			key:      "single-abc123",
			expected: false,
		},
		{
			name:     "basename does not start with single-",
			key:      "2024/01/15/large-abc123.json",
			expected: false,
		},
		{
			name:     "contains single- but not at start of basename",
			key:      "path/not-single-abc.json",
			expected: false,
		},
		{
			name:     "empty key",
			key:      "",
			expected: false,
		},
		{
			name:     "only a slash",
			key:      "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, eventrepo.IsSingleEventRef(tt.key))
		})
	}
}
