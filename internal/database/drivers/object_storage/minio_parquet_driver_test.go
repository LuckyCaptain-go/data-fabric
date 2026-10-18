package object_storage

import (
	"context"
	"testing"
)

// MinIO Parquet reads are not implemented. The reader must say so instead of
// handing back a ParquetFileReader whose Parquet backend is nil, which panics as
// soon as anything tries to read a row.
func TestMinIOParquetReadReportsMissingBackend(t *testing.T) {
	reader := NewMinIOParquetReader(nil, "data/events.parquet")

	fileReader, err := reader.Read(context.Background())
	if err == nil {
		t.Fatalf("Read returned a reader with no Parquet backend: %#v", fileReader)
	}
	if fileReader != nil {
		t.Errorf("Read returned a non-nil reader alongside an error: %#v", fileReader)
	}
}

// The driver's public entry point must surface that same error rather than
// panicking on the nil Parquet backend.
func TestMinIOParquetQueryReportsMissingBackend(t *testing.T) {
	driver := &MinIOParquetDriver{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Query panicked instead of returning an error: %v", r)
		}
	}()

	result, err := driver.Query(context.Background(), &MinIOParquetQuery{Key: "data/events.parquet"})
	if err == nil {
		t.Fatalf("Query returned a result for an unimplemented MinIO Parquet backend: %#v", result)
	}
}
