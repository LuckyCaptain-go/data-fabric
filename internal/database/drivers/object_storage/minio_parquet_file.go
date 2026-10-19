package object_storage

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/xitongsys/parquet-go/source"
)

// MinIOParquetFile adapts a MinIO object to parquet-go's seekable file API.
// It fetches byte ranges on demand, so reading the Parquet footer does not
// require downloading the whole object.
type MinIOParquetFile struct {
	client    *MinIOClient
	ctx       context.Context
	key       string
	size      int64
	position  int64
	buffer    []byte
	readBytes *atomic.Int64
}

var _ source.ParquetFile = (*MinIOParquetFile)(nil)

// NewMinIOParquetFile stats the object and prepares a seekable reader.
func NewMinIOParquetFile(ctx context.Context, client *MinIOClient, key string) (*MinIOParquetFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		return nil, fmt.Errorf("MinIO client is required")
	}
	if key == "" {
		return nil, fmt.Errorf("Parquet object key is required")
	}
	metadata, err := client.GetObjectMetadata(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to stat MinIO object %q: %w", key, err)
	}
	if metadata.ContentLength < 0 {
		return nil, fmt.Errorf("MinIO object %q has an invalid size", key)
	}
	return &MinIOParquetFile{
		client:    client,
		ctx:       ctx,
		key:       key,
		size:      metadata.ContentLength,
		readBytes: &atomic.Int64{},
	}, nil
}

func (f *MinIOParquetFile) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if f.position >= f.size {
		return 0, io.EOF
	}
	if len(f.buffer) == 0 {
		readSize := int64(1024 * 1024)
		if int64(len(p)) > readSize {
			readSize = int64(len(p))
		}
		if remaining := f.size - f.position; readSize > remaining {
			readSize = remaining
		}
		data, err := f.client.GetObjectRange(f.ctx, f.key, f.position, readSize)
		if err != nil {
			return 0, fmt.Errorf("failed to read MinIO object %q at byte %d: %w", f.key, f.position, err)
		}
		f.buffer = data
		if f.readBytes != nil {
			f.readBytes.Add(int64(len(data)))
		}
		if len(f.buffer) == 0 {
			return 0, io.EOF
		}
	}
	n := copy(p, f.buffer)
	f.buffer = f.buffer[n:]
	f.position += int64(n)
	return n, nil
}

func (f *MinIOParquetFile) Seek(offset int64, whence int) (int64, error) {
	var position int64
	switch whence {
	case io.SeekStart:
		position = offset
	case io.SeekCurrent:
		position = f.position + offset
	case io.SeekEnd:
		position = f.size + offset
	default:
		return f.position, fmt.Errorf("invalid seek origin %d", whence)
	}
	if position < 0 {
		return f.position, fmt.Errorf("seek before start of MinIO object")
	}
	if position > f.size {
		position = f.size
	}
	f.position = position
	f.buffer = nil
	return f.position, nil
}

func (f *MinIOParquetFile) Write([]byte) (int, error) {
	return 0, fmt.Errorf("writing to MinIO Parquet files is not supported")
}

func (f *MinIOParquetFile) Close() error {
	f.buffer = nil
	return nil
}

func (f *MinIOParquetFile) Open(name string) (source.ParquetFile, error) {
	if name != "" && name != f.key {
		return nil, fmt.Errorf("external Parquet column file %q is not supported", name)
	}
	return &MinIOParquetFile{
		client:    f.client,
		ctx:       f.ctx,
		key:       f.key,
		size:      f.size,
		readBytes: f.readBytes,
	}, nil
}

func (f *MinIOParquetFile) BytesRead() int64 {
	if f == nil || f.readBytes == nil {
		return 0
	}
	return f.readBytes.Load()
}

func (f *MinIOParquetFile) Create(string) (source.ParquetFile, error) {
	return nil, fmt.Errorf("creating MinIO Parquet files is not supported")
}
