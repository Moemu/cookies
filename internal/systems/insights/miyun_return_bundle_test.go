package insights

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func TestReadMiyunReturnBundleSupportsMP4FilenameAndManifestAssociations(t *testing.T) {
	handoff := MiyunHandoff{SourceMaterialIDs: []string{"material_1", "material_2"}}
	direct := []byte("video")
	files, err := readMiyunReturnBundle(ImportMiyunHandoffReturnBundleRequest{Filename: "material_1__result.mp4", DeclaredSizeBytes: int64(len(direct)), Content: bytes.NewReader(direct)}, handoff)
	if err != nil || len(files) != 1 || files[0].association != MiyunReturnAssociationFilename || files[0].sourceMaterialID != "material_1" {
		t.Fatalf("direct association = %#v, err=%v", files, err)
	}

	manifest := minimalMiyunReturnManifestXLSX(t, "returned.mp4", "material_2")
	var bundle bytes.Buffer
	archive := zip.NewWriter(&bundle)
	videoEntry, _ := archive.Create("returned.mp4")
	_, _ = videoEntry.Write([]byte("returned-video"))
	manifestEntry, _ := archive.Create("manifest.xlsx")
	_, _ = manifestEntry.Write(manifest)
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	files, err = readMiyunReturnBundle(ImportMiyunHandoffReturnBundleRequest{Filename: "returns.zip", DeclaredSizeBytes: int64(bundle.Len()), Content: bytes.NewReader(bundle.Bytes())}, handoff)
	if err != nil || len(files) != 1 || files[0].association != MiyunReturnAssociationManifestXLSX || files[0].sourceMaterialID != "material_2" {
		t.Fatalf("manifest association = %#v, err=%v", files, err)
	}
	reader, err := files[0].open()
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(reader)
	_ = reader.Close()
	if string(content) != "returned-video" {
		t.Fatalf("video content = %q", content)
	}
}

func TestReadMiyunReturnBundleRejectsNestedZIP(t *testing.T) {
	var bundle bytes.Buffer
	archive := zip.NewWriter(&bundle)
	entry, _ := archive.Create("nested/result.mp4")
	_, _ = entry.Write([]byte("video"))
	_ = archive.Close()
	_, err := readMiyunReturnBundle(ImportMiyunHandoffReturnBundleRequest{Filename: "returns.zip", DeclaredSizeBytes: int64(bundle.Len()), Content: bytes.NewReader(bundle.Bytes())}, MiyunHandoff{SourceMaterialIDs: []string{"material_1"}})
	if err == nil {
		t.Fatal("nested ZIP succeeded")
	}
}

func minimalMiyunReturnManifestXLSX(t *testing.T, filename, sourceID string) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	parts := map[string]string{
		"[Content_Types].xml":      `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"xl/workbook.xml":          `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>output_filename</t></is></c><c r="B1" t="inlineStr"><is><t>source_material_id</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>` + filename + `</t></is></c><c r="B2" t="inlineStr"><is><t>` + sourceID + `</t></is></c></row></sheetData></worksheet>`,
	}
	for name, content := range parts {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
