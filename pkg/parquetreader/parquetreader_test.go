package parquetreader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsParquetRef(t *testing.T) {
	assert.True(t, IsParquetRef("cloudevent/valid/year=2025/month=06/day=15/batch-abc.parquet#3"))
	assert.True(t, IsParquetRef("path/file.parquet#0"))
	assert.False(t, IsParquetRef("a0000891234bA5738a18d83D41847dfFbDC6101d37C69c9B0cF00000042!2025-06-15T10:30:00Z!dimo.status!abc!id123"))
	assert.False(t, IsParquetRef("simple/s3/path.json"))
	assert.False(t, IsParquetRef(""))
}

func TestParseIndexKey(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantBucket string
		wantPath   string
		wantOffset int
		wantErr    bool
	}{
		{
			name:       "valid parquet ref (relative)",
			key:        "cloudevent/valid/year=2025/month=06/day=15/batch-abc.parquet#3",
			wantBucket: "",
			wantPath:   "cloudevent/valid/year=2025/month=06/day=15/batch-abc.parquet",
			wantOffset: 3,
		},
		{
			name:       "zero offset",
			key:        "path/file.parquet#0",
			wantBucket: "",
			wantPath:   "path/file.parquet",
			wantOffset: 0,
		},
		{
			name:       "full s3 URI",
			key:        "s3://dimo-iceberg-dev/warehouse/cloudevent/valid/year=2025/month=06/day=15/batch-uuid.parquet#0",
			wantBucket: "dimo-iceberg-dev",
			wantPath:   "warehouse/cloudevent/valid/year=2025/month=06/day=15/batch-uuid.parquet",
			wantOffset: 0,
		},
		{
			name:       "large offset",
			key:        "cloudevent/partial/year=2025/month=12/day=31/batch-xyz.parquet#9999",
			wantBucket: "",
			wantPath:   "cloudevent/partial/year=2025/month=12/day=31/batch-xyz.parquet",
			wantOffset: 9999,
		},
		{
			name:    "no hash separator",
			key:     "legacy/s3/path.json",
			wantErr: true,
		},
		{
			name:    "non-numeric offset",
			key:     "path/file.parquet#abc",
			wantErr: true,
		},
		{
			name:    "s3 URI with no path",
			key:     "s3://bucketonly#0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseIndexKey(tt.key)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantBucket, ref.Bucket)
			assert.Equal(t, tt.wantPath, ref.ObjectKey)
			assert.Equal(t, tt.wantOffset, ref.RowOffset)
		})
	}
}
