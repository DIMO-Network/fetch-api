// Package eventrepo contains service code for gettting and managing cloudevent objects.
package eventrepo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/cloudevent"
	chindexer "github.com/DIMO-Network/cloudevent/pkg/clickhouse"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/volatiletech/sqlboiler/v4/drivers"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const tagsColumn = "JSONExtract(extras, 'tags', 'Array(String)')"

// Service manages and retrieves data messages from indexed objects in S3.
type Service struct {
	objGetter ObjectGetter
	chConn    clickhouse.Conn
}

// ObjectInfo is the information about the object in S3.
type ObjectInfo struct {
	Key string
}

// ObjectGetter is an interface for getting an object from S3.
type ObjectGetter interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// New creates a new instance of serviceService.
func New(chConn clickhouse.Conn, objGetter ObjectGetter) *Service {
	return &Service{
		objGetter: objGetter,
		chConn:    chConn,
	}
}

// GetLatestIndex returns the latest cloud event index that matches the given options.
func (s *Service) GetLatestIndex(ctx context.Context, opts *grpc.SearchOptions) (cloudevent.CloudEvent[ObjectInfo], error) {
	advancedOpts := convertSearchOptionsToAdvanced(opts)
	return s.GetLatestIndexAdvanced(ctx, advancedOpts)
}

// GetLatestIndexAdvanced returns the latest cloud event index that matches the given advanced options.
func (s *Service) GetLatestIndexAdvanced(ctx context.Context, advancedOpts *grpc.AdvancedSearchOptions) (cloudevent.CloudEvent[ObjectInfo], error) {
	if advancedOpts != nil {
		advancedOpts.TimestampAsc = wrapperspb.Bool(false)
	}
	events, err := s.ListIndexesAdvanced(ctx, 1, advancedOpts)
	if err != nil {
		return cloudevent.CloudEvent[ObjectInfo]{}, err
	}
	return events[0], nil
}

// ListIndexes fetches and returns a list of index for cloud events that match the given options.
func (s *Service) ListIndexes(ctx context.Context, limit int, opts *grpc.SearchOptions) ([]cloudevent.CloudEvent[ObjectInfo], error) {
	advancedOpts := convertSearchOptionsToAdvanced(opts)
	return s.ListIndexesAdvanced(ctx, limit, advancedOpts)
}

// ListIndexesAdvanced fetches and returns a list of index for cloud events that match the given advanced options.
func (s *Service) ListIndexesAdvanced(ctx context.Context, limit int, advancedOpts *grpc.AdvancedSearchOptions) ([]cloudevent.CloudEvent[ObjectInfo], error) {
	order := " DESC"
	if advancedOpts != nil && advancedOpts.GetTimestampAsc().GetValue() {
		order = " ASC"
	}
	mods := []qm.QueryMod{
		qm.Select(chindexer.SubjectColumn,
			chindexer.TimestampColumn,
			chindexer.TypeColumn,
			chindexer.IDColumn,
			chindexer.SourceColumn,
			chindexer.ProducerColumn,
			chindexer.DataContentTypeColumn,
			chindexer.DataVersionColumn,
			chindexer.ExtrasColumn,
			chindexer.IndexKeyColumn,
		),
		qm.From(chindexer.TableName),
		qm.OrderBy(chindexer.TimestampColumn + order),
		qm.Limit(limit),
	}

	// Apply advanced search options
	if advancedOpts != nil {
		advancedMods := AdvancedSearchOptionsToQueryMod(advancedOpts)
		mods = append(mods, advancedMods...)
	}
	query, args := newQuery(mods...)
	rows, err := s.chConn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud events: %w", err)
	}

	var cloudEvents []cloudevent.CloudEvent[ObjectInfo]
	var extras string
	for rows.Next() {
		var event cloudevent.CloudEvent[ObjectInfo]
		err = rows.Scan(&event.Subject, &event.Time, &event.Type, &event.ID, &event.Source, &event.Producer, &event.DataContentType, &event.DataVersion, &extras, &event.Data.Key)
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan cloud event: %w", err)
		}
		if extras != "" && extras != "null" {
			if err = json.Unmarshal([]byte(extras), &event.Extras); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("failed to unmarshal extras: %w", err)
			}
			// Restore non-column fields from extras
			chindexer.RestoreNonColumnFields(&event.CloudEventHeader)
		}
		cloudEvents = append(cloudEvents, event)
	}
	_ = rows.Close()
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over cloud events: %w", err)
	}
	if len(cloudEvents) == 0 {
		return nil, fmt.Errorf("no cloud events found %w", sql.ErrNoRows)
	}
	return cloudEvents, nil
}

// ListCloudEvents fetches and returns the cloud events that match the given options.
func (s *Service) ListCloudEvents(ctx context.Context, bucketName string, limit int, opts *grpc.SearchOptions) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	advancedOpts := convertSearchOptionsToAdvanced(opts)
	return s.ListCloudEventsAdvanced(ctx, bucketName, limit, advancedOpts)
}

// ListCloudEventsAdvanced fetches and returns the cloud events that match the given advanced options.
func (s *Service) ListCloudEventsAdvanced(ctx context.Context, bucketName string, limit int, advancedOpts *grpc.AdvancedSearchOptions) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	events, err := s.ListIndexesAdvanced(ctx, limit, advancedOpts)
	if err != nil {
		return nil, err
	}
	data, err := s.ListCloudEventsFromIndexes(ctx, events, bucketName)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetLatestCloudEvent fetches and returns the latest cloud event that matches the given options.
func (s *Service) GetLatestCloudEvent(ctx context.Context, bucketName string, opts *grpc.SearchOptions) (cloudevent.CloudEvent[json.RawMessage], error) {
	advancedOpts := convertSearchOptionsToAdvanced(opts)
	return s.GetLatestCloudEventAdvanced(ctx, bucketName, advancedOpts)
}

// GetLatestCloudEventAdvanced fetches and returns the latest cloud event that matches the given advanced options.
func (s *Service) GetLatestCloudEventAdvanced(ctx context.Context, bucketName string, advancedOpts *grpc.AdvancedSearchOptions) (cloudevent.CloudEvent[json.RawMessage], error) {
	cloudIdx, err := s.GetLatestIndexAdvanced(ctx, advancedOpts)
	if err != nil {
		return cloudevent.CloudEvent[json.RawMessage]{}, err
	}

	data, err := s.GetCloudEventFromIndex(ctx, cloudIdx, bucketName)
	if err != nil {
		return cloudevent.CloudEvent[json.RawMessage]{}, err
	}

	return data, nil
}

// ListCloudEventsFromIndexes fetches and returns the cloud events for the given index.
func (s *Service) ListCloudEventsFromIndexes(ctx context.Context, indexes []cloudevent.CloudEvent[ObjectInfo], bucketName string) ([]cloudevent.CloudEvent[json.RawMessage], error) {
	events := make([]cloudevent.CloudEvent[json.RawMessage], len(indexes))
	var err error
	objectsByKeys := map[string][]byte{}
	for i := range indexes {
		// Some objects have multiple cloud events so we cache the objects to avoid fetching them multiple times.
		if obj, ok := objectsByKeys[indexes[i].Data.Key]; ok {
			events[i] = cloudevent.CloudEvent[json.RawMessage]{CloudEventHeader: indexes[i].CloudEventHeader, Data: obj}
			continue
		}
		events[i], err = s.GetCloudEventFromIndex(ctx, indexes[i], bucketName)
		if err != nil {
			return nil, err
		}
		objectsByKeys[indexes[i].Data.Key] = events[i].Data
	}
	return events, nil
}

// GetCloudEventFromIndex fetches and returns the cloud event for the given index.
func (s *Service) GetCloudEventFromIndex(ctx context.Context, index cloudevent.CloudEvent[ObjectInfo], bucketName string) (cloudevent.CloudEvent[json.RawMessage], error) {
	rawData, err := s.GetObjectFromKey(ctx, index.Data.Key, bucketName)
	if err != nil {
		return cloudevent.CloudEvent[json.RawMessage]{}, err
	}
	return toCloudEvent(&index.CloudEventHeader, rawData), nil
}

// ListObjectsFromKeys fetches and returns the objects for the given keys.
func (s *Service) ListObjectsFromKeys(ctx context.Context, keys []string, bucketName string) ([][]byte, error) {
	data := make([][]byte, len(keys))
	var err error
	for i, key := range keys {
		data[i], err = s.GetObjectFromKey(ctx, key, bucketName)
		if err != nil {
			return nil, fmt.Errorf("failed to get data from key '%s': %w", key, err)
		}
	}
	return data, nil
}

// GetRawObjectFromKey fetches and returns the raw object for the given key without unmarshalling to a cloud event.
func (s *Service) GetObjectFromKey(ctx context.Context, key, bucketName string) ([]byte, error) {
	obj, err := s.objGetter.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer obj.Body.Close() //nolint

	data, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}
	return data, nil
}

// StoreObject stores the given data in S3 with the given cloudevent header.
func (s *Service) StoreObject(ctx context.Context, bucketName string, cloudHeader *cloudevent.CloudEventHeader, data []byte) error {
	key := chindexer.CloudEventToObjectKey(cloudHeader)
	_, err := s.objGetter.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &key,
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to store object in S3: %w", err)
	}

	values := chindexer.CloudEventToSlice(cloudHeader)

	err = s.chConn.Exec(ctx, chindexer.InsertStmt, values...)
	if err != nil {
		return fmt.Errorf("failed to store index in ClickHouse: %w", err)
	}

	return nil
}

// toCloudEvent converts the given data to a cloud event with the given header
// if the provided data is already a cloud event we will replace the header with the given one.
func toCloudEvent(dbHdr *cloudevent.CloudEventHeader, data []byte) cloudevent.CloudEvent[json.RawMessage] {
	retData := data
	event := cloudevent.CloudEvent[json.RawMessage]{}
	err := json.Unmarshal(data, &event)
	emptyHdr := cloudevent.CloudEventHeader{}
	if err == nil && !event.Equals(emptyHdr) {
		// if the data is already a cloud event we use the embedded data field
		retData = event.Data
	}
	return cloudevent.CloudEvent[json.RawMessage]{CloudEventHeader: *dbHdr, Data: retData}
}

func SearchOptionsToQueryMod(opts *grpc.SearchOptions) ([]qm.QueryMod, error) {
	if opts == nil {
		return nil, nil
	}
	var mods []qm.QueryMod
	if opts.GetId() != nil {
		mods = append(mods, qm.Where(chindexer.IDColumn+" = ?", opts.GetId().GetValue()))
	}
	if opts.GetAfter() != nil {
		mods = append(mods, qm.Where(chindexer.TimestampColumn+" > ?", opts.GetAfter().AsTime()))
	}
	if opts.GetBefore() != nil {
		mods = append(mods, qm.Where(chindexer.TimestampColumn+" < ?", opts.GetBefore().AsTime()))
	}
	if opts.GetType() != nil {
		mods = append(mods, qm.Where(chindexer.TypeColumn+" = ?", opts.GetType().GetValue()))
	}
	if opts.GetDataVersion() != nil {
		mods = append(mods, qm.Where(chindexer.DataVersionColumn+" = ?", opts.GetDataVersion().GetValue()))
	}
	if opts.GetSubject() != nil {
		mods = append(mods, qm.Where(chindexer.SubjectColumn+" = ?", opts.GetSubject().GetValue()))
	}
	if opts.GetSource() != nil {
		mods = append(mods, qm.Where(chindexer.SourceColumn+" = ?", opts.GetSource().GetValue()))
	}
	if opts.GetProducer() != nil {
		mods = append(mods, qm.Where(chindexer.ProducerColumn+" = ?", opts.GetProducer().GetValue()))
	}
	if opts.GetExtras() != nil {
		mods = append(mods, qm.Where(chindexer.ExtrasColumn+" = ?", opts.GetExtras().GetValue()))
	}
	return mods, nil
}

// convertSearchOptionsToAdvanced converts basic SearchOptions to AdvancedSearchOptions
func convertSearchOptionsToAdvanced(opts *grpc.SearchOptions) *grpc.AdvancedSearchOptions {
	if opts == nil {
		return nil
	}

	advanced := &grpc.AdvancedSearchOptions{
		After:        opts.GetAfter(),
		Before:       opts.GetBefore(),
		TimestampAsc: opts.GetTimestampAsc(),
	}

	// Convert each field to StringFilterOption with has_any logic
	if opts.GetType() != nil {
		advanced.Type = &grpc.StringFilterOption{
			In: []string{opts.GetType().GetValue()},
		}
	}
	if opts.GetDataVersion() != nil {
		advanced.DataVersion = &grpc.StringFilterOption{
			In: []string{opts.GetDataVersion().GetValue()},
		}
	}
	if opts.GetSubject() != nil {
		advanced.Subject = &grpc.StringFilterOption{
			In: []string{opts.GetSubject().GetValue()},
		}
	}
	if opts.GetSource() != nil {
		advanced.Source = &grpc.StringFilterOption{
			In: []string{opts.GetSource().GetValue()},
		}
	}
	if opts.GetProducer() != nil {
		advanced.Producer = &grpc.StringFilterOption{
			In: []string{opts.GetProducer().GetValue()},
		}
	}
	if opts.GetExtras() != nil {
		advanced.Extras = &grpc.StringFilterOption{
			In: []string{opts.GetExtras().GetValue()},
		}
	}
	if opts.GetId() != nil {
		advanced.Id = &grpc.StringFilterOption{
			In: []string{opts.GetId().GetValue()},
		}
	}

	return advanced
}

func AdvancedSearchOptionsToQueryMod(opts *grpc.AdvancedSearchOptions) []qm.QueryMod {
	if opts == nil {
		return nil
	}
	var mods []qm.QueryMod

	// Handle timestamp filtering (same as SearchOptions)
	if opts.GetAfter() != nil {
		mods = append(mods, qm.Where(chindexer.TimestampColumn+" > ?", opts.GetAfter().AsTime()))
	}
	if opts.GetBefore() != nil {
		mods = append(mods, qm.Where(chindexer.TimestampColumn+" < ?", opts.GetBefore().AsTime()))
	}

	// Handle advanced filtering for each field
	if opts.GetType() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetType(), chindexer.TypeColumn)...))
	}

	if opts.GetDataVersion() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetDataVersion(), chindexer.DataVersionColumn)...))
	}

	if opts.GetSubject() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetSubject(), chindexer.SubjectColumn)...))
	}

	if opts.GetSource() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetSource(), chindexer.SourceColumn)...))
	}

	if opts.GetProducer() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetProducer(), chindexer.ProducerColumn)...))
	}

	if opts.GetExtras() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetExtras(), chindexer.ExtrasColumn)...))
	}

	if opts.GetId() != nil {
		mods = append(mods, qm.Expr(stringFilterMods(opts.GetId(), chindexer.IDColumn)...))
	}

	if opts.GetTags() != nil {
		mods = append(mods, qm.Expr(arrayFilterMods(opts.GetTags(), tagsColumn)...))
	}

	return mods
}

// stringFilterMods converts a StringFilterOption to query modifications.
func stringFilterMods(filter *grpc.StringFilterOption, columnName string) []qm.QueryMod {
	var mods []qm.QueryMod
	if filter == nil {
		return nil
	}

	// Process has_any (OR logic)
	if len(filter.GetIn()) > 0 {
		mods = append(mods, qm.Where(columnName+" IN (?)", filter.GetIn()))
	}
	if len(filter.GetNotIn()) > 0 {
		mods = append(mods, qm.Where(columnName+" NOT IN (?)", filter.GetNotIn()))
	}
	if filter.GetOr() != nil {
		orMods := stringFilterMods(filter.GetOr(), columnName)
		mods = append(mods, qm.Or2(qm.Expr(orMods...)))
	}
	return mods
}

// arrayFilterMods converts an ArrayFilterOption to query modifications.
func arrayFilterMods(filter *grpc.ArrayFilterOption, columnName string) []qm.QueryMod {
	var mods []qm.QueryMod
	if filter == nil {
		return mods
	}

	if len(filter.GetContainsAny()) > 0 {
		mods = append(mods, qm.Where("hasAny("+columnName+", ?)", filter.GetContainsAny()))
	}
	if len(filter.GetContainsAll()) > 0 {
		mods = append(mods, qm.Where("hasAll("+columnName+", ?)", filter.GetContainsAll()))
	}
	if len(filter.GetNotContainsAny()) > 0 {
		mods = append(mods, qm.Where("NOT hasAny("+columnName+", ?)", filter.GetNotContainsAny()))
	}
	if len(filter.GetNotContainsAll()) > 0 {
		mods = append(mods, qm.Where("NOT hasAll("+columnName+", ?)", filter.GetNotContainsAll()))
	}
	// Process OR condition recursively
	if filter.GetOr() != nil {
		var orMods []qm.QueryMod
		orMods = arrayFilterMods(filter.GetOr(), columnName)
		mods = append(mods, qm.Or2(qm.Expr(orMods...)))
	}

	return mods
}

var dialect = drivers.Dialect{
	LQ:                      '`',
	RQ:                      '`',
	UseIndexPlaceholders:    false,
	UseLastInsertID:         false,
	UseSchema:               false,
	UseDefaultKeyword:       false,
	UseAutoColumns:          false,
	UseTopClause:            false,
	UseOutputClause:         false,
	UseCaseWhenExistsClause: false,
}

// newQuery initializes a new Query using the passed in QueryMods.
func newQuery(mods ...qm.QueryMod) (string, []any) {
	q := &queries.Query{}
	queries.SetDialect(q, &dialect)
	qm.Apply(q, mods...)
	return queries.BuildQuery(q)
}
