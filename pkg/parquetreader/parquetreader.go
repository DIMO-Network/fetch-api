// Package parquetreader provides utilities for reading individual rows from
// Parquet files stored on S3-compatible object storage.
//
// Index key format (produced by the DPS Benthos output dimo_parquet_writer):
//
//	[{full_uri}|]{object_key}#{row_offset}
//
// Full URI form: s3://bucket/prefix/.../file.parquet#row (bucket and key are parsed).
// Relative form: prefix/.../file.parquet#row (caller must supply bucket).
// Use ParseIndexKey to split into bucket (if present), object key, and 0-based row offset.
// Use IsParquetRef to distinguish from legacy JSON object keys (no '#').
package parquetreader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// IndexKeyRef holds the parsed components of a Parquet index_key reference.
type IndexKeyRef struct {
	// Bucket is set when index_key is a full s3://bucket/key#row URI; otherwise empty.
	Bucket string
	// ObjectKey is the S3 object key (path) of the Parquet file, without bucket.
	ObjectKey string
	// RowOffset is the 0-based row index within the Parquet file.
	RowOffset int
}

// IsParquetRef returns true if the given index_key uses the new Parquet reference
// format (contains #). Legacy keys (individual S3 JSON files) do not contain #.
func IsParquetRef(indexKey string) bool {
	return strings.Contains(indexKey, "#")
}

// ParseIndexKey parses an index_key string into bucket (if s3:// URI), object key, and row offset.
// Supports: "s3://bucket/key.parquet#row" (sets Bucket) or "key.parquet#row" (Bucket empty).
func ParseIndexKey(indexKey string) (IndexKeyRef, error) {
	parts := strings.SplitN(indexKey, "#", 2)
	if len(parts) != 2 {
		return IndexKeyRef{}, fmt.Errorf("invalid parquet index_key format (missing #): %s", indexKey)
	}

	rowOffset, err := strconv.Atoi(parts[1])
	if err != nil {
		return IndexKeyRef{}, fmt.Errorf("invalid row offset in index_key %q: %w", indexKey, err)
	}

	pathPart := parts[0]
	ref := IndexKeyRef{RowOffset: rowOffset}
	if strings.HasPrefix(pathPart, "s3://") {
		// e.g. s3://mybucket/prefix/.../file.parquet -> bucket=mybucket, key=prefix/.../file.parquet
		rest := pathPart[5:] // after "s3://"
		idx := strings.Index(rest, "/")
		if idx < 0 {
			return IndexKeyRef{}, fmt.Errorf("invalid s3 URI in index_key (no path): %s", indexKey)
		}
		ref.Bucket = rest[:idx]
		ref.ObjectKey = rest[idx+1:]
	} else {
		ref.ObjectKey = pathPart
	}
	return ref, nil
}

//------------------------------------------------------------------------------

// ObjectGetter is an interface for fetching objects from S3-compatible storage.
// *s3.Client implements it; use this for testing or alternate backends.
type ObjectGetter interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Reader reads individual row payloads from Parquet files on S3-compatible storage.
// Schema is compatible with CloudEvent Parquet files written by dimo_parquet_writer.
type Reader struct {
	objGetter ObjectGetter
}

// New returns a Reader that uses the given ObjectGetter (e.g. *s3.Client).
func New(objGetter ObjectGetter) *Reader {
	return &Reader{objGetter: objGetter}
}

// ReadData reads the "data" column value for the row at ref.RowOffset in the
// Parquet file at ref.ObjectKey. Bucket is used only when ref.Bucket is empty (relative key).
// Returns nil, nil for null data.
func (r *Reader) ReadData(ctx context.Context, bucket string, ref IndexKeyRef) ([]byte, error) {
	bucketToUse := ref.Bucket
	if bucketToUse == "" {
		bucketToUse = bucket
	}
	if bucketToUse == "" {
		return nil, fmt.Errorf("no bucket for parquet ref (index_key has no s3:// URI and parquet bucket not configured)")
	}
	// Fetch the Parquet file from object storage.
	obj, err := r.objGetter.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketToUse),
		Key:    aws.String(ref.ObjectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get parquet file %s: %w", ref.ObjectKey, err)
	}
	defer obj.Body.Close() //nolint

	rawData, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read parquet file body: %w", err)
	}

	// Open the Parquet file from the in-memory buffer.
	// bytes.Reader satisfies parquet.ReaderAtSeeker (io.ReaderAt + io.ReadSeeker).
	pqReader, err := file.NewParquetReader(bytes.NewReader(rawData))
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet reader: %w", err)
	}
	defer func() { _ = pqReader.Close() }()

	// Read the entire file as an Arrow table.
	tbl, err := pqarrow.ReadTable(ctx, bytes.NewReader(rawData), nil, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	if err != nil {
		return nil, fmt.Errorf("failed to read arrow table: %w", err)
	}
	defer tbl.Release()

	// Validate row offset.
	if int64(ref.RowOffset) >= tbl.NumRows() {
		return nil, fmt.Errorf("row offset %d out of range (table has %d rows)", ref.RowOffset, tbl.NumRows())
	}

	// Find the "data" column.
	dataColIdx := -1
	for i, field := range tbl.Schema().Fields() {
		if field.Name == "data" {
			dataColIdx = i
			break
		}
	}
	if dataColIdx < 0 {
		return nil, fmt.Errorf("parquet file %s has no 'data' column", ref.ObjectKey)
	}

	// Extract the value at the target row offset.
	// Arrow tables store data in chunked arrays; navigate to the right chunk and index.
	col := tbl.Column(dataColIdx)
	remaining := int64(ref.RowOffset)
	for _, chunk := range col.Data().Chunks() {
		if remaining < int64(chunk.Len()) {
			// Target row is in this chunk.
			if chunk.IsNull(int(remaining)) {
				return nil, nil // null data value
			}
			// Binary/ByteArray columns store data as []byte via Value().
			rawVal := chunk.ValueStr(int(remaining))
			out := make([]byte, len(rawVal))
			copy(out, rawVal)
			return out, nil
		}
		remaining -= int64(chunk.Len())
	}

	return nil, fmt.Errorf("row offset %d not found in chunked column data", ref.RowOffset)
}
