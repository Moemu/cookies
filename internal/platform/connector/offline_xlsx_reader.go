package connector

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	offlineXLSXMaxFileBytes  = 8 * 1024 * 1024
	offlineXLSXMaxEntryBytes = 8 * 1024 * 1024
	offlineXLSXMaxTotalBytes = 32 * 1024 * 1024
	offlineXLSXMaxEntries    = 128
	offlineXLSXMaxRows       = 20_000
	offlineXLSXMaxColumns    = 64
	offlineXLSXMaxCellChars  = 4_096
)

type offlineXLSXRelationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

type offlineXLSXRelationships struct {
	Items []offlineXLSXRelationship `xml:"Relationship"`
}

type offlineXLSXWorkbook struct {
	Sheets []struct {
		Name  string `xml:"name,attr"`
		State string `xml:"state,attr"`
		RID   string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

func readOfflineXLSX(content []byte) ([][]string, error) {
	if len(content) == 0 || len(content) > offlineXLSXMaxFileBytes {
		return nil, fmt.Errorf("%w: offline workbook size is invalid", ErrInvalidFact)
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(reader.File) == 0 || len(reader.File) > offlineXLSXMaxEntries {
		return nil, fmt.Errorf("%w: offline workbook container is invalid", ErrInvalidFact)
	}
	files := make(map[string]*zip.File, len(reader.File))
	var total uint64
	for _, file := range reader.File {
		lower := strings.ToLower(file.Name)
		if !safeOfflineXLSXName(file.Name) || file.Flags&0x1 != 0 || file.UncompressedSize64 > offlineXLSXMaxEntryBytes ||
			strings.HasPrefix(lower, "xl/embeddings/") || strings.HasPrefix(lower, "xl/oleobjects/") ||
			strings.HasPrefix(lower, "xl/externallinks/") || strings.HasPrefix(lower, "xl/macrosheets/") || lower == "xl/vbaproject.bin" {
			return nil, fmt.Errorf("%w: offline workbook contains an unsupported part", ErrInvalidFact)
		}
		total += file.UncompressedSize64
		if total > offlineXLSXMaxTotalBytes || files[file.Name] != nil {
			return nil, fmt.Errorf("%w: offline workbook expansion is invalid", ErrInvalidFact)
		}
		files[file.Name] = file
	}
	for name := range files {
		if !strings.HasSuffix(strings.ToLower(name), ".rels") {
			continue
		}
		relationships, relErr := readOfflineXLSXRelationships(files, name)
		if relErr != nil {
			return nil, relErr
		}
		for _, relationship := range relationships.Items {
			if relationship.TargetMode != "" {
				return nil, fmt.Errorf("%w: external workbook relationships are not permitted", ErrInvalidFact)
			}
		}
	}
	rootRelationships, err := readOfflineXLSXRelationships(files, "_rels/.rels")
	if err != nil {
		return nil, err
	}
	workbookPath := ""
	for _, relationship := range rootRelationships.Items {
		if strings.HasSuffix(relationship.Type, "/officeDocument") {
			workbookPath, err = resolveOfflineXLSXTarget("", relationship.Target)
			if err != nil || workbookPath != "xl/workbook.xml" {
				return nil, fmt.Errorf("%w: workbook relationship is invalid", ErrInvalidFact)
			}
		}
	}
	if workbookPath == "" {
		return nil, fmt.Errorf("%w: workbook part is missing", ErrInvalidFact)
	}
	workbookData, err := readOfflineXLSXPart(files, workbookPath)
	if err != nil {
		return nil, err
	}
	var workbook offlineXLSXWorkbook
	if xml.Unmarshal(workbookData, &workbook) != nil || len(workbook.Sheets) != 1 || workbook.Sheets[0].State == "hidden" || workbook.Sheets[0].State == "veryHidden" {
		return nil, fmt.Errorf("%w: one visible worksheet is required", ErrInvalidFact)
	}
	relationshipsPath := path.Join(path.Dir(workbookPath), "_rels", path.Base(workbookPath)+".rels")
	workbookRelationships, err := readOfflineXLSXRelationships(files, relationshipsPath)
	if err != nil {
		return nil, err
	}
	var worksheetPath string
	for _, relationship := range workbookRelationships.Items {
		if relationship.ID == workbook.Sheets[0].RID && strings.HasSuffix(relationship.Type, "/worksheet") {
			worksheetPath, err = resolveOfflineXLSXTarget(workbookPath, relationship.Target)
			if err != nil || !strings.HasPrefix(worksheetPath, "xl/worksheets/") {
				return nil, fmt.Errorf("%w: worksheet relationship is invalid", ErrInvalidFact)
			}
		}
	}
	if worksheetPath == "" {
		return nil, fmt.Errorf("%w: worksheet part is missing", ErrInvalidFact)
	}
	shared, err := parseOfflineXLSXSharedStrings(files)
	if err != nil {
		return nil, err
	}
	worksheet, err := readOfflineXLSXPart(files, worksheetPath)
	if err != nil {
		return nil, err
	}
	return parseOfflineXLSXWorksheet(worksheet, shared)
}

func safeOfflineXLSXName(name string) bool {
	return name != "" && !strings.Contains(name, "\\") && !strings.HasPrefix(name, "/") && !strings.Contains(name, "..") && path.Clean(name) == name
}

func resolveOfflineXLSXTarget(base, target string) (string, error) {
	if target == "" || strings.ContainsAny(target, "\\?#") || strings.HasPrefix(target, "/") {
		return "", ErrInvalidFact
	}
	resolved := path.Clean(path.Join(path.Dir(base), target))
	if !safeOfflineXLSXName(resolved) {
		return "", ErrInvalidFact
	}
	return resolved, nil
}

func readOfflineXLSXPart(files map[string]*zip.File, name string) ([]byte, error) {
	file := files[name]
	if file == nil || file.UncompressedSize64 > offlineXLSXMaxEntryBytes {
		return nil, fmt.Errorf("%w: workbook part is invalid", ErrInvalidFact)
	}
	stream, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, offlineXLSXMaxEntryBytes+1))
	if err != nil || len(data) > offlineXLSXMaxEntryBytes {
		return nil, fmt.Errorf("%w: workbook part is too large", ErrInvalidFact)
	}
	return data, nil
}

func readOfflineXLSXRelationships(files map[string]*zip.File, name string) (offlineXLSXRelationships, error) {
	data, err := readOfflineXLSXPart(files, name)
	if err != nil {
		return offlineXLSXRelationships{}, err
	}
	var relationships offlineXLSXRelationships
	if xml.Unmarshal(data, &relationships) != nil {
		return offlineXLSXRelationships{}, fmt.Errorf("%w: workbook relationships are invalid", ErrInvalidFact)
	}
	return relationships, nil
}

func parseOfflineXLSXSharedStrings(files map[string]*zip.File) ([]string, error) {
	if files["xl/sharedStrings.xml"] == nil {
		return nil, nil
	}
	data, err := readOfflineXLSXPart(files, "xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	values := []string{}
	for {
		token, tokenErr := decoder.Token()
		if tokenErr == io.EOF {
			return values, nil
		}
		if tokenErr != nil {
			return nil, tokenErr
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "si" {
			continue
		}
		value, decodeErr := decodeOfflineXLSXText(decoder, start)
		if decodeErr != nil {
			return nil, decodeErr
		}
		values = append(values, value)
		if len(values) > offlineXLSXMaxRows*offlineXLSXMaxColumns {
			return nil, fmt.Errorf("%w: shared string count is too large", ErrInvalidFact)
		}
	}
}

func parseOfflineXLSXWorksheet(data []byte, shared []string) ([][]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	rows := [][]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if len(rows) < 2 {
				return nil, fmt.Errorf("%w: workbook requires a header and data", ErrInvalidFact)
			}
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "col" && offlineXLSXAttributeTrue(start, "hidden") {
			return nil, fmt.Errorf("%w: hidden columns are not permitted", ErrInvalidFact)
		}
		if start.Name.Local != "row" {
			continue
		}
		if offlineXLSXAttributeTrue(start, "hidden") {
			return nil, fmt.Errorf("%w: hidden rows are not permitted", ErrInvalidFact)
		}
		row, rowErr := parseOfflineXLSXRow(decoder, start, shared)
		if rowErr != nil {
			return nil, rowErr
		}
		if len(row) > 0 {
			rows = append(rows, row)
			if len(rows) > offlineXLSXMaxRows {
				return nil, fmt.Errorf("%w: workbook row count is too large", ErrInvalidFact)
			}
		}
	}
}

func parseOfflineXLSXRow(decoder *xml.Decoder, row xml.StartElement, shared []string) ([]string, error) {
	values := []string{}
	lastColumn := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local != "c" {
				continue
			}
			column, columnErr := offlineXLSXCellColumn(value)
			if columnErr != nil || column <= lastColumn || column > offlineXLSXMaxColumns {
				return nil, fmt.Errorf("%w: workbook cell reference is invalid", ErrInvalidFact)
			}
			lastColumn = column
			cell, cellErr := parseOfflineXLSXCell(decoder, value, shared)
			if cellErr != nil {
				return nil, cellErr
			}
			for len(values) < column-1 {
				values = append(values, "")
			}
			values = append(values, cell)
		case xml.EndElement:
			if value.Name.Local == "row" {
				return values, nil
			}
		}
	}
}

func offlineXLSXAttributeTrue(element xml.StartElement, name string) bool {
	for _, attribute := range element.Attr {
		if attribute.Name.Local == name {
			return attribute.Value == "1" || strings.EqualFold(attribute.Value, "true")
		}
	}
	return false
}

func offlineXLSXCellColumn(cell xml.StartElement) (int, error) {
	reference := ""
	for _, attribute := range cell.Attr {
		if attribute.Name.Local == "r" {
			reference = attribute.Value
		}
	}
	column := 0
	for _, character := range reference {
		if character < 'A' || character > 'Z' {
			break
		}
		column = column*26 + int(character-'A'+1)
	}
	if column == 0 {
		return 0, ErrInvalidFact
	}
	return column, nil
}

func parseOfflineXLSXCell(decoder *xml.Decoder, cell xml.StartElement, shared []string) (string, error) {
	typ := ""
	for _, attribute := range cell.Attr {
		if attribute.Name.Local == "t" {
			typ = attribute.Value
		}
	}
	value := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch item := token.(type) {
		case xml.StartElement:
			if item.Name.Local == "f" {
				return "", fmt.Errorf("%w: formulas are not permitted in offline exports", ErrInvalidFact)
			}
			if item.Name.Local == "v" || item.Name.Local == "is" {
				decoded, decodeErr := decodeOfflineXLSXText(decoder, item)
				if decodeErr != nil {
					return "", decodeErr
				}
				value = decoded
			}
		case xml.EndElement:
			if item.Name.Local != "c" {
				continue
			}
			if typ == "s" {
				index, indexErr := strconv.Atoi(strings.TrimSpace(value))
				if indexErr != nil || index < 0 || index >= len(shared) {
					return "", fmt.Errorf("%w: shared string reference is invalid", ErrInvalidFact)
				}
				value = shared[index]
			}
			if !utf8.ValidString(value) || len([]rune(value)) > offlineXLSXMaxCellChars {
				return "", fmt.Errorf("%w: workbook cell text is invalid", ErrInvalidFact)
			}
			return strings.TrimSpace(value), nil
		}
	}
}

func decodeOfflineXLSXText(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var builder strings.Builder
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if value.Name.Local == "t" && depth > 1 {
				var text string
				if decodeErr := decoder.DecodeElement(&text, &value); decodeErr != nil {
					return "", decodeErr
				}
				builder.WriteString(text)
				depth--
			}
		case xml.CharData:
			if start.Name.Local == "v" {
				builder.Write([]byte(value))
			}
		case xml.EndElement:
			depth--
		}
	}
	return builder.String(), nil
}
