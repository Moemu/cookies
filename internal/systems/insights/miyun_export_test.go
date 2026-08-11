package insights

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestExportMiyunHandoffZIPUsesFrozenDraftManifestAndSafeLayout(t *testing.T) {
	snapshot := MiyunHandoffExportSnapshot{
		ManifestVersion: MiyunHandoffManifestVersion,
		Manifest:        MiyunHandoffManifest{HandoffID: "handoff_1", HandoffVersion: "7", SourceMaterialName: "中文,\"换行\n素材", MiyunMaterialID: "m_1", SourceURL: "https://example.test/source", Source: "miyun", DeliveryDays: "12", CumulativeImpressions: "unknown", RelatedAds: "", RelatedCreators: "3", TargetProduct: "产品", TargetCategory: "品类", Notes: "a,\"b\"\nnext", ParameterVersion: "params-v1", InputHash: "sha256:input"},
		Sources:         []MiyunHandoffExportFile{{Reference: "source", Name: "../../NUL.mp4"}, {Reference: "source2", Name: "../../NUL.mp4"}},
		ProductMedia:    []MiyunHandoffExportFile{{Reference: "media1", Name: "商品,图.png"}, {Reference: "media2", Name: "商品,图.png"}},
		ProductDocs:     []MiyunHandoffExportFile{{Reference: "doc1", Name: "报价\x00单.pdf"}},
	}
	snapshot.Manifest.SourceMaterialIDs = []string{"m_1", "m_2"}
	reader := exportReader{content: map[string][]byte{"source": []byte("video"), "source2": []byte("video2"), "media1": []byte("one"), "media2": []byte("two"), "doc1": []byte("doc")}}
	var output bytes.Buffer
	if err := ExportMiyunHandoffZIP(context.Background(), &output, snapshot, &reader); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"viral/source/", "product/media/", "product/docs/", "manifest.csv", "viral/source/file.mp4", "viral/source/file (2).mp4", "product/media/商品,图.png", "product/media/商品,图 (2).png", "product/docs/报价-单.pdf"}
	if len(archive.File) != len(wantNames) {
		t.Fatalf("entries = %d, want %d", len(archive.File), len(wantNames))
	}
	for index, want := range wantNames {
		if got := archive.File[index].Name; got != want {
			t.Errorf("entry %d = %q, want %q", index, got, want)
		}
	}
	manifest := readZIPFile(t, archive, "manifest.csv")
	if !bytes.HasPrefix(manifest, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("manifest has no UTF-8 BOM")
	}
	records, err := csv.NewReader(bytes.NewReader(manifest[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(records[0], ","); got != strings.Join(miyunHandoffManifestColumns, ",") {
		t.Fatalf("columns = %q", got)
	}
	if len(records) != 3 || records[1][3] != "m_1" || records[2][3] != "m_2" {
		t.Fatalf("source manifest records = %#v", records)
	}
	if got, want := records[1][4], "viral/source/file.mp4"; got != want {
		t.Errorf("source path = %q, want %q", got, want)
	}
	if got, want := records[2][4], "viral/source/file (2).mp4"; got != want {
		t.Errorf("second source path = %q, want %q", got, want)
	}
	if got := string(readZIPFile(t, archive, "viral/source/file.mp4")); got != "video" {
		t.Errorf("source = %q", got)
	}
	if got := string(readZIPFile(t, archive, "viral/source/file (2).mp4")); got != "video2" {
		t.Errorf("second source = %q", got)
	}
}

func TestExportMiyunHandoffZIPStreamsLargeContentAndPropagatesCloseError(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 1024*128)
	reader := exportReader{content: map[string][]byte{"source": data}}
	snapshot := testMiyunExportSnapshot()
	var output bytes.Buffer
	if err := ExportMiyunHandoffZIP(context.Background(), &output, snapshot, &reader); err != nil {
		t.Fatal(err)
	}
	if reader.opens != 1 || reader.maxRead > 64*1024 {
		t.Fatalf("opens/max read = %d/%d; expected a bounded streaming read", reader.opens, reader.maxRead)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := readZIPFile(t, archive, "viral/source/source.mp4")
	if sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatal("streamed content hash differs")
	}
	reader.closeErr = errors.New("close failed")
	if err := ExportMiyunHandoffZIP(context.Background(), io.Discard, snapshot, &reader); !errors.Is(err, reader.closeErr) {
		t.Fatalf("error = %v, want close error", err)
	}
}

func TestExportMiyunHandoffZIPRetainsV1ManifestCompatibility(t *testing.T) {
	snapshot := testMiyunExportSnapshot()
	snapshot.ManifestVersion = MiyunHandoffManifestV1
	reader := exportReader{content: map[string][]byte{"source": []byte("video")}}
	var output bytes.Buffer
	if err := ExportMiyunHandoffZIP(context.Background(), &output, snapshot, &reader); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	manifest := readZIPFile(t, archive, "manifest.csv")
	records, err := csv.NewReader(bytes.NewReader(manifest[3:])).ReadAll()
	if err != nil || len(records) != 2 || !reflect.DeepEqual(records[0], miyunHandoffManifestV1Columns) || records[1][0] != MiyunHandoffManifestV1 {
		t.Fatalf("v1 manifest records = %#v, %v", records, err)
	}
}

func TestExportMiyunHandoffZIPRejectsUnknownSchemaAndCancellation(t *testing.T) {
	snapshot := testMiyunExportSnapshot()
	snapshot.ManifestVersion = "unknown"
	if err := ExportMiyunHandoffZIP(context.Background(), io.Discard, snapshot, &exportReader{}); err == nil {
		t.Fatal("unknown schema succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot.ManifestVersion = MiyunHandoffManifestVersion
	if err := ExportMiyunHandoffZIP(ctx, io.Discard, snapshot, &exportReader{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestCleanMiyunHandoffFileNameBlocksSeparatorsControlsAndReservedNames(t *testing.T) {
	cases := map[string]string{
		`..\\folder/CON.txt`: "file.txt",
		"AUX":                "file",
		"lpt9.pdf":           "file.pdf",
		"\x00 . ":            "-",
		"normal 中文.mp4":      "normal 中文.mp4",
	}
	for input, want := range cases {
		if got := cleanMiyunHandoffFileName(input); got != want {
			t.Errorf("cleanMiyunHandoffFileName(%q) = %q, want %q", input, got, want)
		}
	}
}

func testMiyunExportSnapshot() MiyunHandoffExportSnapshot {
	return MiyunHandoffExportSnapshot{ManifestVersion: MiyunHandoffManifestVersion, Manifest: MiyunHandoffManifest{HandoffID: "h", HandoffVersion: "1", SourceMaterialIDs: []string{"source_1"}}, Sources: []MiyunHandoffExportFile{{Reference: "source", Name: "source.mp4"}}}
}

type exportReader struct {
	content        map[string][]byte
	closeErr       error
	opens, maxRead int
}

func (r *exportReader) OpenMiyunHandoffExportFile(_ context.Context, file MiyunHandoffExportFile) (io.ReadCloser, error) {
	r.opens++
	return &countingReadCloser{Reader: bytes.NewReader(r.content[file.Reference]), owner: r, closeErr: r.closeErr}, nil
}

type countingReadCloser struct {
	io.Reader
	owner    *exportReader
	closeErr error
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > r.owner.maxRead {
		r.owner.maxRead = n
	}
	return n, err
}
func (r *countingReadCloser) Close() error { return r.closeErr }
func readZIPFile(t *testing.T, archive *zip.Reader, name string) []byte {
	t.Helper()
	for _, file := range archive.File {
		if file.Name == name {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			value, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
	}
	t.Fatalf("missing ZIP entry %q", name)
	return nil
}
