package insights

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestExportMiyunHandoffPackageZIPUsesTwoFlatSafePackages(t *testing.T) {
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
	if err := ExportMiyunHandoffPackageZIP(context.Background(), &output, snapshot, MiyunHandoffPackageSources, &reader); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"miyun_m_1.mp4", "miyun_m_2.mp4"}
	if len(archive.File) != len(wantNames) {
		t.Fatalf("entries = %d, want %d", len(archive.File), len(wantNames))
	}
	for index, want := range wantNames {
		if got := archive.File[index].Name; got != want {
			t.Errorf("entry %d = %q, want %q", index, got, want)
		}
	}
	if got := string(readZIPFile(t, archive, "miyun_m_1.mp4")); got != "video" {
		t.Errorf("source = %q", got)
	}
	if got := string(readZIPFile(t, archive, "miyun_m_2.mp4")); got != "video2" {
		t.Errorf("second source = %q", got)
	}

	output.Reset()
	if err := ExportMiyunHandoffPackageZIP(context.Background(), &output, snapshot, MiyunHandoffPackageProject, &reader); err != nil {
		t.Fatal(err)
	}
	projectArchive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	projectNames := []string{"商品,图.png", "商品,图 (2).png", "报价-单.pdf"}
	if len(projectArchive.File) != len(projectNames) {
		t.Fatalf("project entries = %d, want %d", len(projectArchive.File), len(projectNames))
	}
	for index, want := range projectNames {
		if got := projectArchive.File[index].Name; got != want || strings.Contains(got, "/") {
			t.Errorf("project entry %d = %q, want flat %q", index, got, want)
		}
	}
}

func TestExportMiyunHandoffPackageZIPStreamsLargeContentAndPropagatesCloseError(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 1024*128)
	reader := exportReader{content: map[string][]byte{"source": data}}
	snapshot := testMiyunExportSnapshot()
	var output bytes.Buffer
	if err := ExportMiyunHandoffPackageZIP(context.Background(), &output, snapshot, MiyunHandoffPackageSources, &reader); err != nil {
		t.Fatal(err)
	}
	if reader.opens != 1 || reader.maxRead > 64*1024 {
		t.Fatalf("opens/max read = %d/%d; expected a bounded streaming read", reader.opens, reader.maxRead)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := readZIPFile(t, archive, "miyun_source_1.mp4")
	if sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatal("streamed content hash differs")
	}
	reader.closeErr = errors.New("close failed")
	if err := ExportMiyunHandoffPackageZIP(context.Background(), io.Discard, snapshot, MiyunHandoffPackageSources, &reader); !errors.Is(err, reader.closeErr) {
		t.Fatalf("error = %v, want close error", err)
	}
}

func TestExportMiyunHandoffPackageZIPRetainsV1SnapshotCompatibility(t *testing.T) {
	snapshot := testMiyunExportSnapshot()
	snapshot.ManifestVersion = MiyunHandoffManifestV1
	reader := exportReader{content: map[string][]byte{"source": []byte("video")}}
	var output bytes.Buffer
	if err := ExportMiyunHandoffPackageZIP(context.Background(), &output, snapshot, MiyunHandoffPackageSources, &reader); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 1 || archive.File[0].Name != "miyun_source_1.mp4" {
		t.Fatalf("v1 archive entries = %#v", archive.File)
	}
}

func TestExportMiyunHandoffPackageZIPRejectsUnknownSchemaPackageAndCancellation(t *testing.T) {
	snapshot := testMiyunExportSnapshot()
	snapshot.ManifestVersion = "unknown"
	if err := ExportMiyunHandoffPackageZIP(context.Background(), io.Discard, snapshot, MiyunHandoffPackageSources, &exportReader{}); err == nil {
		t.Fatal("unknown schema succeeded")
	}
	snapshot.ManifestVersion = MiyunHandoffManifestVersion
	if err := ExportMiyunHandoffPackageZIP(context.Background(), io.Discard, snapshot, "nested", &exportReader{}); err == nil {
		t.Fatal("unknown package succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ExportMiyunHandoffPackageZIP(ctx, io.Discard, snapshot, MiyunHandoffPackageSources, &exportReader{}); !errors.Is(err, context.Canceled) {
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
