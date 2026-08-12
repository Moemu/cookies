package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestExtractTXT(t *testing.T) {
	text, mime, err := extractDocument(".txt", []byte("\xef\xbb\xbfhello\nworld"))
	if err != nil || text != "hello\nworld" || mime != "text/plain" {
		t.Fatalf("extract txt = %q, %q, %v", text, mime, err)
	}
	if _, _, err := extractDocument(".txt", []byte{0xff}); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("invalid utf-8 error = %v", err)
	}
}

func TestSupportedDocumentFormatMatrixIsExplicit(t *testing.T) {
	for _, extension := range []string{".md", ".txt", ".docx", ".xlsx", ".pdf"} {
		if !supportedDocumentExtension(extension) {
			t.Errorf("expected %s to be supported", extension)
		}
	}
	for _, extension := range []string{".psd", ".ai", ".sketch", ".doc", ".xls", ".zip"} {
		if supportedDocumentExtension(extension) {
			t.Errorf("expected %s to be rejected", extension)
		}
	}
}

func TestExtractXLSXVisibleSheetsInWorkbookOrder(t *testing.T) {
	content := buildXLSX(t, map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0"?><Types><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/></Types>`,
		"_rels/.rels":                `<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns:r="urn:r"><sheets><sheet name="Second" sheetId="2" r:id="r2"/><sheet name="Hidden" state="hidden" sheetId="3" r:id="r3"/><sheet name="First" sheetId="1" r:id="r1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/one.xml"/><Relationship Id="r2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/two.xml"/><Relationship Id="r3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/hidden.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst><si><t>shared</t></si></sst>`,
		"xl/worksheets/one.xml":      `<worksheet><cols><col min="3" max="3" hidden="1"/></cols><sheetData><row><c r="A1" t="inlineStr"><is><t>first</t></is></c><c r="B1" t="s"><v>0</v></c><c r="C1" t="inlineStr"><is><t>hidden column</t></is></c></row><row hidden="1"><c r="A2" t="inlineStr"><is><t>hidden row</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/two.xml":      `<worksheet><sheetData><row><c r="A1"><v>2</v></c></row></sheetData></worksheet>`,
		"xl/worksheets/hidden.xml":   `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>secret</t></is></c></row></sheetData></worksheet>`,
	})
	text, mime, err := extractDocument(".xlsx", content)
	if err != nil || text != "2\nfirst\tshared" || mime != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("extract xlsx = %q, %q, %v", text, mime, err)
	}
}

func TestXLSXAdmissionAndUnsafeParts(t *testing.T) {
	if !allowedMIME(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") ||
		!allowedMIME(".xlsx", "application/octet-stream") || allowedMIME(".xlsx", "application/zip-compressed") {
		t.Fatal("xlsx MIME admission does not match the supported contract")
	}
	parts := minimalXLSXParts()
	parts["xl/vbaProject.bin"] = "macro payload"
	content := buildXLSX(t, parts)
	if _, _, err := extractDocument(".xlsx", content); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("macro-enabled archive error = %v", err)
	}
}

func TestXLSXRejectsExternalRelationshipsEncryptionAndColumnOverflow(t *testing.T) {
	parts := minimalXLSXParts()
	parts["xl/worksheets/_rels/one.xml.rels"] = `<Relationships><Relationship Id="external" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.test" TargetMode="External"/></Relationships>`
	if _, _, err := extractDocument(".xlsx", buildXLSX(t, parts)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("external relationship error = %v", err)
	}

	if _, _, err := extractDocument(".xlsx", markZIPEncrypted(buildXLSX(t, minimalXLSXParts()))); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("encrypted entry error = %v", err)
	}

	parts = minimalXLSXParts()
	parts["xl/worksheets/one.xml"] = `<worksheet><sheetData><row><c r="IW1"><v>257</v></c></row></sheetData></worksheet>`
	if _, _, err := extractDocument(".xlsx", buildXLSX(t, parts)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("column overflow error = %v", err)
	}
}

func TestExtractXLSXRejectsRenamedCorruptAndOversizedZIP(t *testing.T) {
	if _, _, err := extractDocument(".xlsx", []byte("not a zip")); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("corrupt xlsx error = %v", err)
	}
	renamed := buildXLSX(t, map[string]string{"note.txt": "plain zip"})
	if _, _, err := extractDocument(".xlsx", renamed); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("renamed zip error = %v", err)
	}
	over := buildXLSX(t, map[string]string{
		"[Content_Types].xml": string(bytes.Repeat([]byte("x"), int(maxXLSXEntryBytes+1))),
	})
	if _, _, err := extractDocument(".xlsx", over); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("oversized zip entry error = %v", err)
	}
}

func buildXLSX(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range parts {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func minimalXLSXParts() map[string]string {
	return map[string]string{
		"[Content_Types].xml":        `<Types><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/></Types>`,
		"_rels/.rels":                `<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns:r="urn:r"><sheets><sheet name="Sheet" sheetId="1" r:id="r1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/one.xml"/></Relationships>`,
		"xl/worksheets/one.xml":      `<worksheet><sheetData><row><c r="A1"><v>1</v></c></row></sheetData></worksheet>`,
	}
}

func markZIPEncrypted(content []byte) []byte {
	result := append([]byte(nil), content...)
	for offset := 0; offset+10 <= len(result); offset++ {
		switch binary.LittleEndian.Uint32(result[offset:]) {
		case 0x04034b50:
			binary.LittleEndian.PutUint16(result[offset+6:], binary.LittleEndian.Uint16(result[offset+6:])|0x1)
		case 0x02014b50:
			binary.LittleEndian.PutUint16(result[offset+8:], binary.LittleEndian.Uint16(result[offset+8:])|0x1)
		}
	}
	return result
}
