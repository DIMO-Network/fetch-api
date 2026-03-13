package graph

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubObjectGetter is a minimal test double for eventrepo.ObjectGetter.
// Only HeadObject is exercised by the presign path; the other methods panic
// if called so tests fail loudly if unexpected code paths are hit.
type stubObjectGetter struct {
	headObject func(bucket, key string) error
}

func (s *stubObjectGetter) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	panic("GetObject called unexpectedly in presign test")
}

func (s *stubObjectGetter) PutObject(_ context.Context, _ *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	panic("PutObject called unexpectedly in presign test")
}

func (s *stubObjectGetter) HeadObject(_ context.Context, params *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return &s3.HeadObjectOutput{}, s.headObject(aws.ToString(params.Bucket), aws.ToString(params.Key))
}

// fakePresignClient creates an s3.PresignClient backed by static fake
// credentials. Presigning is pure HMAC computation — no network calls occur.
func fakePresignClient() *s3.PresignClient {
	s3c := s3.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("TESTAKID", "TESTSECRET", ""),
	})
	return s3.NewPresignClient(s3c)
}

// presignQueryResolver builds a queryResolver wired with the given service,
// buckets, and expiry using the shared fake presign client.
func presignQueryResolver(svc *eventrepo.Service, buckets []string, expiry time.Duration) *queryResolver {
	return &queryResolver{&Resolver{
		EventService:  svc,
		Buckets:       buckets,
		Presigner:     fakePresignClient(),
		PresignExpiry: expiry,
	}}
}

// --- PresignedUrl field resolver ---

func TestPresignedUrlResolver(t *testing.T) {
	t.Parallel()
	r := &cloudEventResolver{&Resolver{}}

	t.Run("nil wrapper returns nil", func(t *testing.T) {
		t.Parallel()
		url, err := r.PresignedUrl(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, url)
	})

	t.Run("wrapper with empty PresignedURL returns nil", func(t *testing.T) {
		t.Parallel()
		w := &CloudEventWrapper{Raw: &cloudevent.RawEvent{}}
		url, err := r.PresignedUrl(context.Background(), w)
		require.NoError(t, err)
		assert.Nil(t, url)
	})

	t.Run("wrapper with PresignedURL returns pointer to URL", func(t *testing.T) {
		t.Parallel()
		const expected = "https://s3.example.com/bucket/single-xyz.json?sig=abc"
		w := &CloudEventWrapper{
			Raw:          &cloudevent.RawEvent{},
			PresignedURL: expected,
		}
		url, err := r.PresignedUrl(context.Background(), w)
		require.NoError(t, err)
		require.NotNil(t, url)
		assert.Equal(t, expected, *url)
	})
}

// --- Header resolver with a presigned wrapper ---
// Verifies that header fields are still accessible when data is replaced by
// a presigned URL (the Raw event carries the header from the index).

func TestHeaderResolverWithPresignedWrapper(t *testing.T) {
	t.Parallel()
	r := &cloudEventResolver{&Resolver{}}

	hdr := cloudevent.CloudEventHeader{
		ID:      "evt-001",
		Source:  "0xABC",
		Subject: "did:eth:137:0x1234:42",
		Type:    "dimo.status",
	}
	w := &CloudEventWrapper{
		Raw:          &cloudevent.RawEvent{CloudEventHeader: hdr},
		PresignedURL: "https://s3.example.com/bucket/single-uuid.json?sig=x",
	}

	got, err := r.Header(context.Background(), w)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "evt-001", got.ID)
	assert.Equal(t, "0xABC", got.Source)
	assert.Equal(t, "dimo.status", got.Type)

	// data and dataBase64 must be nil when only a presigned URL is present
	data, err := r.Data(context.Background(), w)
	require.NoError(t, err)
	assert.Nil(t, data)

	b64, err := r.DataBase64(context.Background(), w)
	require.NoError(t, err)
	assert.Nil(t, b64)
}

// --- presignSingleEvent helper ---

func TestPresignSingleEvent(t *testing.T) {
	t.Parallel()

	const (
		bucket1 = "cloud-events"
		bucket2 = "ephemeral-events"
		key     = "2024/01/15/single-abc123.json"
	)

	t.Run("object in first bucket returns presigned URL for that bucket", func(t *testing.T) {
		t.Parallel()
		stub := &stubObjectGetter{
			headObject: func(bucket, k string) error {
				if bucket == bucket1 && k == key {
					return nil
				}
				return &s3types.NoSuchKey{}
			},
		}
		svc := eventrepo.New(nil, stub, "")
		q := presignQueryResolver(svc, []string{bucket1, bucket2}, 15*time.Minute)

		url, err := q.presignSingleEvent(context.Background(), key)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(url, "https://"), "URL should be HTTPS")
		assert.Contains(t, url, bucket1)
		assert.Contains(t, url, "single-abc123.json")
	})

	t.Run("object absent from first bucket falls back to second", func(t *testing.T) {
		t.Parallel()
		stub := &stubObjectGetter{
			headObject: func(bucket, k string) error {
				if bucket == bucket2 && k == key {
					return nil // only in bucket2
				}
				return &s3types.NoSuchKey{}
			},
		}
		svc := eventrepo.New(nil, stub, "")
		q := presignQueryResolver(svc, []string{bucket1, bucket2}, 15*time.Minute)

		url, err := q.presignSingleEvent(context.Background(), key)
		require.NoError(t, err)
		assert.Contains(t, url, bucket2)
		assert.Contains(t, url, "single-abc123.json")
	})

	t.Run("object in neither bucket returns error mentioning the key", func(t *testing.T) {
		t.Parallel()
		stub := &stubObjectGetter{
			headObject: func(_, _ string) error {
				return &s3types.NoSuchKey{}
			},
		}
		svc := eventrepo.New(nil, stub, "")
		q := presignQueryResolver(svc, []string{bucket1, bucket2}, 15*time.Minute)

		url, err := q.presignSingleEvent(context.Background(), key)
		require.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), key)
	})

	t.Run("HeadObject error other than NoSuchKey still skips bucket", func(t *testing.T) {
		t.Parallel()
		// Any error (not just NoSuchKey) causes the bucket to be skipped.
		stub := &stubObjectGetter{
			headObject: func(bucket, k string) error {
				if bucket == bucket1 {
					return fmt.Errorf("access denied")
				}
				return nil // bucket2 succeeds
			},
		}
		svc := eventrepo.New(nil, stub, "")
		q := presignQueryResolver(svc, []string{bucket1, bucket2}, 15*time.Minute)

		url, err := q.presignSingleEvent(context.Background(), key)
		require.NoError(t, err)
		assert.Contains(t, url, bucket2)
	})

	t.Run("presigned URL embeds the configured expiry", func(t *testing.T) {
		t.Parallel()
		const expiry = 30 * time.Minute // 1800 seconds
		stub := &stubObjectGetter{
			headObject: func(_, _ string) error { return nil },
		}
		svc := eventrepo.New(nil, stub, "")
		q := presignQueryResolver(svc, []string{bucket1}, expiry)

		url, err := q.presignSingleEvent(context.Background(), key)
		require.NoError(t, err)
		assert.Contains(t, url, "X-Amz-Expires=1800")
	})

	t.Run("no buckets configured returns error", func(t *testing.T) {
		t.Parallel()
		svc := eventrepo.New(nil, &stubObjectGetter{headObject: func(_, _ string) error { return nil }}, "")
		q := presignQueryResolver(svc, nil, 15*time.Minute)

		url, err := q.presignSingleEvent(context.Background(), key)
		require.Error(t, err)
		assert.Empty(t, url)
	})
}
