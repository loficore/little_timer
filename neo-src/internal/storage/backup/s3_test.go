package backup

// S3Adapter 的测试，使用 mock S3APIClient。
//
// mock 在内存中存对象并记录全部 API 调用，无需真实 S3 端点或
// testcontainers 即可完成完整的行为测试。

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// mockS3Client 以内存方式实现 S3APIClient。每个方法把入参记录进 calls
// slice，返回值由 struct 字段驱动。mockObj 字段为 nil 时返回零值 output，
// 调用方只需设置自己关心的字段。
type mockS3Client struct {
	store map[string][]byte // 按 key 存放的对象

	// 记录调用（每次调用追加）。
	putCalls        []*s3.PutObjectInput
	getCalls        []*s3.GetObjectInput
	deleteCalls     []*s3.DeleteObjectInput
	headBucketCalls []*s3.HeadBucketInput
	listCalls       []*s3.ListObjectsV2Input

	// 各方法的错误覆盖（nil = 成功）。
	headBucketErr    error
	putObjectErr     error
	getObjectErr     error
	deleteObjectErr  error
	listObjectsV2Err error

	// 覆盖 ListObjectsV2 的 output（nil = 空 output）。
	listObjectsV2Out *s3.ListObjectsV2Output

	// 覆盖 GetObject 的 output（nil = 查 store / 空 body）。
	getObjectOut *s3.GetObjectOutput
}

func (m *mockS3Client) HeadBucket(_ context.Context, params *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	m.headBucketCalls = append(m.headBucketCalls, params)
	if m.headBucketErr != nil {
		return nil, m.headBucketErr
	}
	return &s3.HeadBucketOutput{}, nil
}

func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putCalls = append(m.putCalls, params)
	if m.putObjectErr != nil {
		return nil, m.putObjectErr
	}
	if m.store != nil && params.Key != nil && params.Body != nil {
		b, err := io.ReadAll(params.Body)
		if err == nil {
			m.store[*params.Key] = b
		}
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getCalls = append(m.getCalls, params)
	if m.getObjectErr != nil {
		return nil, m.getObjectErr
	}
	if m.getObjectOut != nil {
		return m.getObjectOut, nil
	}
	if m.store != nil && params.Key != nil {
		b, ok := m.store[*params.Key]
		if ok {
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
		}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.deleteCalls = append(m.deleteCalls, params)
	if m.deleteObjectErr != nil {
		return nil, m.deleteObjectErr
	}
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(_ context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.listCalls = append(m.listCalls, params)
	if m.listObjectsV2Err != nil {
		return nil, m.listObjectsV2Err
	}
	if m.listObjectsV2Out != nil {
		return m.listObjectsV2Out, nil
	}
	return &s3.ListObjectsV2Output{}, nil
}

// newS3AdapterWithMock 返回接到 mock client 的 S3Adapter。adapter 直接
// 构造（不走 NewS3Adapter），测试因此不需要真实 AWS 凭据。
func newS3AdapterWithMock(t *testing.T, mock *mockS3Client) *S3Adapter {
	t.Helper()
	return &S3Adapter{
		cfg: S3Config{
			Bucket:     "test-bucket",
			Region:     "us-east-1",
			PathPrefix: "little_timer/",
		},
		client: mock,
	}
}

// writeTempFile 创建带给定内容的临时文件并返回路径。清理由调用方负责。
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// readTempFile 读取文件内容；出错则测试失败。
func readTempFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return b
}

// TestS3BackupRestoreRoundTrip 验证 Backup 写入的字节被 Restore 原样
// 取回 —— mock 存储 PutObject 的字节并从 GetObject 返回。
func TestS3BackupRestoreRoundTrip(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	original := []byte("hello S3 backup round-trip")
	srcPath := writeTempFile(t, original)
	destPath := filepath.Join(t.TempDir(), "restored.db")

	if err := adapter.Backup(srcPath, "presets_backup_1234567890.db"); err != nil {
		t.Fatalf("Backup: unexpected error: %v", err)
	}

	if err := adapter.Restore("presets_backup_1234567890.db", destPath); err != nil {
		t.Fatalf("Restore: unexpected error: %v", err)
	}

	if got := readTempFile(t, destPath); !bytes.Equal(got, original) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, original)
	}
}

// TestS3TestConnection_WriteProbe 验证 TestConnection 依次调用
// HeadBucket、PutObject、GetObject、DeleteObject，且用的是 probe 文件名。
func TestS3TestConnection_WriteProbe(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	if err := adapter.TestConnection(); err != nil {
		t.Fatalf("TestConnection: unexpected error: %v", err)
	}

	// HeadBucket 必须被调用一次。
	if n := len(mock.headBucketCalls); n != 1 {
		t.Fatalf("HeadBucket: want 1 call, got %d", n)
	}

	// PutObject 必须用带 probe 的 key 调用。
	if n := len(mock.putCalls); n != 1 {
		t.Fatalf("PutObject: want 1 call, got %d", n)
	}
	key := aws.ToString(mock.putCalls[0].Key)
	if !strings.Contains(key, "lt_probe_") {
		t.Fatalf("PutObject key %q does not contain lt_probe_", key)
	}

	// GetObject 必须用同一个 key 调用。
	if n := len(mock.getCalls); n != 1 {
		t.Fatalf("GetObject: want 1 call, got %d", n)
	}
	if aws.ToString(mock.getCalls[0].Key) != key {
		t.Fatalf("GetObject key mismatch: got %q, want %q",
			aws.ToString(mock.getCalls[0].Key), key)
	}

	// DeleteObject 必须用同一个 key 调用。
	if n := len(mock.deleteCalls); n != 1 {
		t.Fatalf("DeleteObject: want 1 call, got %d", n)
	}
	if aws.ToString(mock.deleteCalls[0].Key) != key {
		t.Fatalf("DeleteObject key mismatch: got %q, want %q",
			aws.ToString(mock.deleteCalls[0].Key), key)
	}
}

// TestS3TestConnection_NoSuchBucket 验证 HeadBucket 的 NoSuchBucket 错误
// 被分类为 ErrFileNotFound。
func TestS3TestConnection_NoSuchBucket(t *testing.T) {
	mock := &mockS3Client{
		store:         make(map[string][]byte),
		headBucketErr: &types.NoSuchBucket{Message: aws.String("test")},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got %v", err)
	}
}

// TestS3TestConnection_AccessDenied 验证 PutObject 的 AccessDenied 错误
// 被分类为 ErrPermissionDenied。
func TestS3TestConnection_AccessDenied(t *testing.T) {
	mock := &mockS3Client{
		store:        make(map[string][]byte),
		putObjectErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "test"},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

// TestS3TestConnection_InvalidAccessKeyId 验证 code 为
// "InvalidAccessKeyId" 的 GenericAPIError 被分类为 ErrAuthenticationFail。
func TestS3TestConnection_InvalidAccessKeyId(t *testing.T) {
	mock := &mockS3Client{
		store: make(map[string][]byte),
		headBucketErr: &smithy.GenericAPIError{
			Code:    "InvalidAccessKeyId",
			Message: "test",
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Fatalf("expected ErrAuthenticationFail, got %v", err)
	}
}

// TestS3TestConnection_SignatureDoesNotMatch 验证 code 为
// "SignatureDoesNotMatch" 的 GenericAPIError 被分类为 ErrAuthenticationFail。
func TestS3TestConnection_SignatureDoesNotMatch(t *testing.T) {
	mock := &mockS3Client{
		store: make(map[string][]byte),
		headBucketErr: &smithy.GenericAPIError{
			Code:    "SignatureDoesNotMatch",
			Message: "test",
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Fatalf("expected ErrAuthenticationFail, got %v", err)
	}
}

// TestS3TestConnection_NetworkError 验证 *url.Error 被分类为 ErrNetworkError。
func TestS3TestConnection_NetworkError(t *testing.T) {
	mock := &mockS3Client{
		store: make(map[string][]byte),
		headBucketErr: &url.Error{
			Op:  "Get",
			URL: "https://s3.example.com",
			Err: errors.New("dial tcp: connection refused"),
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrNetworkError) {
		t.Fatalf("expected ErrNetworkError, got %v", err)
	}
}

// TestS3TestConnection_NilBody 验证 GetObject 返回 nil Body 时不会在
// io.ReadAll 处 panic：TestConnection 必须返回 ErrConnectionFailed，
// 并且仍尝试 DeleteObject probe 清理。
func TestS3TestConnection_NilBody(t *testing.T) {
	mock := &mockS3Client{
		store:        make(map[string][]byte),
		getObjectOut: &s3.GetObjectOutput{},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for nil GetObject body, got nil")
	}
	if !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("expected ErrConnectionFailed, got %v", err)
	}
	if n := len(mock.deleteCalls); n != 1 {
		t.Fatalf("DeleteObject probe cleanup: want 1 call, got %d", n)
	}
	if key := aws.ToString(mock.deleteCalls[0].Key); !strings.Contains(key, "lt_probe_") {
		t.Errorf("DeleteObject key %q does not contain lt_probe_", key)
	}
}

// TestS3Backup_AccessDenied 验证 Backup 期间 PutObject 的 AccessDenied
// 错误被分类为 ErrPermissionDenied（而非笼统的 ErrBackupFailed）。
func TestS3Backup_AccessDenied(t *testing.T) {
	mock := &mockS3Client{
		store:        make(map[string][]byte),
		putObjectErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "test"},
	}
	adapter := newS3AdapterWithMock(t, mock)
	srcPath := writeTempFile(t, []byte("test"))

	err := adapter.Backup(srcPath, "presets_backup_1234567890.db")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

// TestS3List 验证 ListObjectsV2 的结果被解析为 BackupInfo 条目，
// 且剥掉了前缀。
func TestS3List(t *testing.T) {
	now := time.Now()
	mock := &mockS3Client{
		store: make(map[string][]byte),
		listObjectsV2Out: &s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key:          aws.String("little_timer/presets_backup_1000.db"),
					LastModified: aws.Time(now),
					Size:         aws.Int64(2048),
				},
				{
					Key:          aws.String("little_timer/presets_backup_2000.db"),
					LastModified: aws.Time(now.Add(time.Hour)),
					Size:         aws.Int64(4096),
				},
			},
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	results, err := adapter.List()
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].Name != "presets_backup_1000.db" {
		t.Fatalf("first name: got %q, want presets_backup_1000.db", results[0].Name)
	}
	if results[0].SizeBytes != 2048 {
		t.Fatalf("first size: got %d, want 2048", results[0].SizeBytes)
	}
	if results[1].Name != "presets_backup_2000.db" {
		t.Fatalf("second name: got %q, want presets_backup_2000.db", results[1].Name)
	}
	if results[1].SizeBytes != 4096 {
		t.Fatalf("second size: got %d, want 4096", results[1].SizeBytes)
	}

	// 校验 ListObjectsV2 调用使用了正确的前缀。
	if n := len(mock.listCalls); n != 1 {
		t.Fatalf("ListObjectsV2: want 1 call, got %d", n)
	}
	if aws.ToString(mock.listCalls[0].Prefix) != "little_timer/" {
		t.Fatalf("prefix: got %q, want little_timer/", aws.ToString(mock.listCalls[0].Prefix))
	}
}

// TestS3List_TimestampFromName 验证备份时间戳解析自 key 名
// （与 Local/WebDAV 一致），且名字可解析时忽略 LastModified。
func TestS3List_TimestampFromName(t *testing.T) {
	lastModified := time.Unix(1800000000, 0)
	mock := &mockS3Client{
		store: make(map[string][]byte),
		listObjectsV2Out: &s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key:          aws.String("little_timer/presets_backup_1700000000.db"),
					LastModified: aws.Time(lastModified),
					Size:         aws.Int64(100),
				},
				{
					Key:          aws.String("little_timer/presets_backup_1700000100.db"),
					LastModified: aws.Time(lastModified),
					Size:         aws.Int64(200),
				},
			},
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	results, err := adapter.List()
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}

	// 时间戳必须取自文件名，而不是 LastModified。
	if got := results[0].Timestamp; got != 1700000000 {
		t.Errorf("first timestamp: got %d, want 1700000000 (name-parsed, LastModified=%d)", got, lastModified.Unix())
	}
	if got := results[1].Timestamp; got != 1700000100 {
		t.Errorf("second timestamp: got %d, want 1700000100 (name-parsed, LastModified=%d)", got, lastModified.Unix())
	}
}

// TestS3Delete 验证 Delete 用正确的 key（PathPrefix + "/" + backupName）
// 调用 DeleteObject 且不返回错误。
func TestS3Delete(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	if err := adapter.Delete("presets_backup_1234567890.db"); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	if n := len(mock.deleteCalls); n != 1 {
		t.Fatalf("DeleteObject: want 1 call, got %d", n)
	}
	wantKey := "little_timer/presets_backup_1234567890.db"
	if got := aws.ToString(mock.deleteCalls[0].Key); got != wantKey {
		t.Fatalf("DeleteObject key: got %q, want %q", got, wantKey)
	}
}

// TestS3WriteManifest 验证 WriteManifest 用 manifest.json key 调用
// PutObject，且 Content-Type 头为 application/json。
func TestS3WriteManifest(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	manifestData := `{"version":1,"backups":[]}`
	if err := adapter.WriteManifest(manifestData); err != nil {
		t.Fatalf("WriteManifest: unexpected error: %v", err)
	}
	if n := len(mock.putCalls); n != 1 {
		t.Fatalf("PutObject: want 1 call, got %d", n)
	}
	call := mock.putCalls[0]
	if aws.ToString(call.Key) != "little_timer/manifest.json" {
		t.Fatalf("key: got %q, want little_timer/manifest.json", aws.ToString(call.Key))
	}
	if aws.ToString(call.ContentType) != "application/json" {
		t.Fatalf("ContentType: got %q, want application/json", aws.ToString(call.ContentType))
	}

	// 验证 body 已存进 mock。
	b, ok := mock.store["little_timer/manifest.json"]
	if !ok {
		t.Fatal("manifest.json not stored in mock")
	}
	if string(b) != manifestData {
		t.Fatalf("body: got %q, want %q", string(b), manifestData)
	}
}
