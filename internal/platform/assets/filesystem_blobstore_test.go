package assets

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestFilesystemBlobStorePersistsAcrossInstances(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	data := testPNG(t)
	store, err := NewFilesystemBlobStore(root)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.Put(context.Background(), "assets", "org/project/asset/v1", bytes.NewReader(data), int64(len(data)), "image/png")
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := NewFilesystemBlobStore(root)
	if err != nil {
		t.Fatal(err)
	}
	reader, info, err := reopened.Open(context.Background(), stored.ObjectLocation)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) || info.SizeBytes != int64(len(data)) || info.MIMEType != "image/png" {
		t.Fatalf("unexpected reopened object: size=%d mime=%q", info.SizeBytes, info.MIMEType)
	}
	signed, err := reopened.SignGet(context.Background(), stored.ObjectLocation, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if signed.URL != "" || signed.Method != "GET" {
		t.Fatalf("local signed request=%#v", signed)
	}
}

func TestFilesystemBlobStoreRejectsEscapesAndOverwrite(t *testing.T) {
	t.Parallel()
	store, err := NewFilesystemBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "../outside", "object", bytes.NewReader([]byte("x")), 1, "text/plain"); err == nil {
		t.Fatal("expected escaping bucket to be rejected")
	}
	if _, err := store.Put(context.Background(), "assets", "../outside", bytes.NewReader([]byte("x")), 1, "text/plain"); err == nil {
		t.Fatal("expected escaping key to be rejected")
	}
	if _, err := store.Put(context.Background(), "assets", "stable/key", bytes.NewReader([]byte("one")), 3, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "assets", "stable/key", bytes.NewReader([]byte("one")), 3, "text/plain"); err != nil {
		t.Fatalf("idempotent put failed: %v", err)
	}
	if _, err := store.Put(context.Background(), "assets", "stable/key", bytes.NewReader([]byte("two")), 3, "text/plain"); err == nil {
		t.Fatal("expected immutable overwrite to be rejected")
	}
}
