package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// These limits deliberately apply before XML parsing.  XLSX is a ZIP container,
// so limiting only the final extracted text would still permit zip bombs.
const (
	maxXLSXEntries             = 128
	maxXLSXSheets              = 32
	maxXLSXRowsPerSheet        = 10000
	maxXLSXColumnsPerRow       = 256
	maxXLSXEntryBytes    int64 = 2 * 1024 * 1024
	maxXLSXTotalBytes    int64 = 20 * 1024 * 1024
	maxXLSXOutputChars         = 1_000_000
)

type xlsxRelationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

type xlsxRelationships struct {
	Items []xlsxRelationship `xml:"Relationship"`
}

type xlsxWorkbook struct {
	Sheets []xlsxSheet `xml:"sheets>sheet"`
}

type xlsxContentTypes struct {
	Overrides []struct {
		PartName    string `xml:"PartName,attr"`
		ContentType string `xml:"ContentType,attr"`
	} `xml:"Override"`
}

type xlsxSheet struct {
	Name  string `xml:"name,attr"`
	State string `xml:"state,attr"`
	RID   string `xml:"id,attr"`
}

func extractXLSX(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(reader.File) == 0 || len(reader.File) > maxXLSXEntries {
		return "", ErrInvalidDocument
	}
	files := make(map[string]*zip.File, len(reader.File))
	var total uint64
	for _, file := range reader.File {
		if !safeZIPName(file.Name) || file.Flags&0x1 != 0 || file.UncompressedSize64 > uint64(maxXLSXEntryBytes) {
			return "", ErrInvalidDocument
		}
		lowerName := strings.ToLower(file.Name)
		if strings.HasPrefix(lowerName, "xl/embeddings/") ||
			strings.HasPrefix(lowerName, "xl/oleobjects/") ||
			strings.HasPrefix(lowerName, "xl/externallinks/") ||
			strings.HasPrefix(lowerName, "xl/macrosheets/") ||
			lowerName == "xl/vbaproject.bin" {
			return "", ErrInvalidDocument
		}
		total += file.UncompressedSize64
		if total > uint64(maxXLSXTotalBytes) || files[file.Name] != nil {
			return "", ErrInvalidDocument
		}
		files[file.Name] = file
	}
	contentTypes, err := readXLSXPart(files, "[Content_Types].xml")
	if err != nil || !validXLSXContentTypes(contentTypes) {
		return "", ErrInvalidDocument
	}
	for name := range files {
		if !strings.HasSuffix(strings.ToLower(name), ".rels") {
			continue
		}
		relationships, relErr := readRelationships(files, name)
		if relErr != nil {
			return "", ErrInvalidDocument
		}
		for _, rel := range relationships.Items {
			if rel.TargetMode != "" {
				return "", ErrInvalidDocument
			}
		}
	}
	rootRels, err := readRelationships(files, "_rels/.rels")
	if err != nil {
		return "", ErrInvalidDocument
	}
	workbookPath := ""
	for _, rel := range rootRels.Items {
		if rel.TargetMode != "" {
			return "", ErrInvalidDocument
		}
		if strings.HasSuffix(rel.Type, "/officeDocument") && rel.TargetMode == "" {
			if workbookPath != "" {
				return "", ErrInvalidDocument
			}
			workbookPath, err = resolveXLSXTarget("", rel.Target)
			if err != nil || workbookPath != "xl/workbook.xml" {
				return "", ErrInvalidDocument
			}
		}
	}
	if workbookPath == "" {
		return "", ErrInvalidDocument
	}
	workbookData, err := readXLSXPart(files, workbookPath)
	if err != nil {
		return "", ErrInvalidDocument
	}
	var workbook xlsxWorkbook
	if xml.Unmarshal(workbookData, &workbook) != nil || len(workbook.Sheets) == 0 || len(workbook.Sheets) > maxXLSXSheets {
		return "", ErrInvalidDocument
	}
	relsPath := path.Join(path.Dir(workbookPath), "_rels", path.Base(workbookPath)+".rels")
	workbookRels, err := readRelationships(files, relsPath)
	if err != nil {
		return "", ErrInvalidDocument
	}
	relByID := make(map[string]xlsxRelationship, len(workbookRels.Items))
	for _, rel := range workbookRels.Items {
		if rel.ID == "" || rel.TargetMode != "" || relByID[rel.ID].ID != "" {
			return "", ErrInvalidDocument
		}
		relByID[rel.ID] = rel
	}
	shared, err := parseSharedStrings(files)
	if err != nil {
		return "", ErrInvalidDocument
	}
	var output xlsxTextBuilder
	for _, sheet := range workbook.Sheets {
		if sheet.State == "hidden" || sheet.State == "veryHidden" {
			continue
		}
		rel, ok := relByID[sheet.RID]
		if !ok || !strings.HasSuffix(rel.Type, "/worksheet") || rel.TargetMode != "" {
			return "", ErrInvalidDocument
		}
		sheetPath, err := resolveXLSXTarget(workbookPath, rel.Target)
		if err != nil || !strings.HasPrefix(sheetPath, "xl/worksheets/") {
			return "", ErrInvalidDocument
		}
		part, err := readXLSXPart(files, sheetPath)
		if err != nil || parseWorksheet(part, shared, &output) != nil {
			return "", ErrInvalidDocument
		}
	}
	return strings.TrimSpace(output.String()), nil
}

func validXLSXContentTypes(data []byte) bool {
	var contentTypes xlsxContentTypes
	if xml.Unmarshal(data, &contentTypes) != nil {
		return false
	}
	foundWorkbook := false
	for _, override := range contentTypes.Overrides {
		if override.PartName != "/xl/workbook.xml" {
			continue
		}
		if foundWorkbook || override.ContentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" {
			return false
		}
		foundWorkbook = true
	}
	return foundWorkbook
}

func safeZIPName(name string) bool {
	return name != "" && !strings.Contains(name, "\\") && !strings.HasPrefix(name, "/") &&
		!strings.Contains(name, "..") && path.Clean(name) == name
}

func resolveXLSXTarget(base, target string) (string, error) {
	if target == "" || strings.ContainsAny(target, "\\?#") || strings.HasPrefix(target, "/") {
		return "", ErrInvalidDocument
	}
	resolved := path.Clean(path.Join(path.Dir(base), target))
	if !safeZIPName(resolved) {
		return "", ErrInvalidDocument
	}
	return resolved, nil
}

func readXLSXPart(files map[string]*zip.File, name string) ([]byte, error) {
	file := files[name]
	if file == nil || file.UncompressedSize64 > uint64(maxXLSXEntryBytes) {
		return nil, ErrInvalidDocument
	}
	stream, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, maxXLSXEntryBytes+1))
	if err != nil || int64(len(data)) > maxXLSXEntryBytes {
		return nil, ErrInvalidDocument
	}
	return data, nil
}

func readRelationships(files map[string]*zip.File, name string) (xlsxRelationships, error) {
	data, err := readXLSXPart(files, name)
	if err != nil {
		return xlsxRelationships{}, err
	}
	var relationships xlsxRelationships
	if xml.Unmarshal(data, &relationships) != nil {
		return xlsxRelationships{}, ErrInvalidDocument
	}
	return relationships, nil
}

func parseSharedStrings(files map[string]*zip.File) ([]string, error) {
	file := files["xl/sharedStrings.xml"]
	if file == nil {
		return nil, nil
	}
	data, err := readXLSXPart(files, file.Name)
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	values := []string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return values, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "si" {
			continue
		}
		text, err := decodeXLSXText(decoder, start)
		if err != nil {
			return nil, err
		}
		values = append(values, text)
		if len(values) > maxXLSXRowsPerSheet*maxXLSXSheets {
			return nil, ErrInvalidDocument
		}
	}
}

func parseWorksheet(data []byte, shared []string, output *xlsxTextBuilder) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	rows := 0
	hiddenColumns := make([]bool, maxXLSXColumnsPerRow+1)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "col" {
			if err := recordHiddenXLSXColumns(start, hiddenColumns); err != nil {
				return err
			}
			continue
		}
		if start.Name.Local != "row" {
			continue
		}
		rows++
		if rows > maxXLSXRowsPerSheet {
			return ErrInvalidDocument
		}
		if xlsxAttributeTrue(start, "hidden") {
			if err := decoder.Skip(); err != nil {
				return err
			}
			continue
		}
		cells, err := parseXLSXRow(decoder, start, shared, hiddenColumns)
		if err != nil {
			return err
		}
		if len(cells) > 0 {
			if err := output.add(strings.Join(cells, "\t") + "\n"); err != nil {
				return err
			}
		}
	}
}

func parseXLSXRow(decoder *xml.Decoder, row xml.StartElement, shared []string, hiddenColumns []bool) ([]string, error) {
	cells := []string{}
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
			column, err := xlsxCellColumn(value)
			if err != nil || column <= lastColumn || column > maxXLSXColumnsPerRow {
				return nil, ErrInvalidDocument
			}
			lastColumn = column
			cell, err := parseXLSXCell(decoder, value, shared)
			if err != nil {
				return nil, err
			}
			if hiddenColumns[column] {
				continue
			}
			position := visibleXLSXColumnPosition(column, hiddenColumns)
			for len(cells) < position-1 {
				cells = append(cells, "")
			}
			cells = append(cells, cell)
		case xml.EndElement:
			if value.Name.Local == "row" {
				return cells, nil
			}
		}
	}
}

func visibleXLSXColumnPosition(column int, hidden []bool) int {
	position := 0
	for index := 1; index <= column; index++ {
		if !hidden[index] {
			position++
		}
	}
	return position
}

func recordHiddenXLSXColumns(column xml.StartElement, hidden []bool) error {
	if !xlsxAttributeTrue(column, "hidden") {
		return nil
	}
	minimum, maximum := 0, 0
	for _, attr := range column.Attr {
		var err error
		switch attr.Name.Local {
		case "min":
			minimum, err = strconv.Atoi(attr.Value)
		case "max":
			maximum, err = strconv.Atoi(attr.Value)
		}
		if err != nil {
			return ErrInvalidDocument
		}
	}
	if minimum < 1 || maximum < minimum || maximum > 16384 {
		return ErrInvalidDocument
	}
	if minimum >= len(hidden) {
		return nil
	}
	if maximum >= len(hidden) {
		maximum = len(hidden) - 1
	}
	for index := minimum; index <= maximum; index++ {
		hidden[index] = true
	}
	return nil
}

func xlsxAttributeTrue(element xml.StartElement, name string) bool {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value == "1" || strings.EqualFold(attr.Value, "true")
		}
	}
	return false
}

func xlsxCellColumn(cell xml.StartElement) (int, error) {
	reference := ""
	for _, attr := range cell.Attr {
		if attr.Name.Local == "r" {
			reference = attr.Value
			break
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
		return 0, ErrInvalidDocument
	}
	return column, nil
}

func parseXLSXCell(decoder *xml.Decoder, cell xml.StartElement, shared []string) (string, error) {
	typ := ""
	for _, attr := range cell.Attr {
		if attr.Name.Local == "t" {
			typ = attr.Value
		}
	}
	value := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch valueToken := token.(type) {
		case xml.StartElement:
			if valueToken.Name.Local == "v" || valueToken.Name.Local == "is" {
				text, err := decodeXLSXText(decoder, valueToken)
				if err != nil {
					return "", err
				}
				value = text
			}
		case xml.EndElement:
			if valueToken.Name.Local == "c" {
				if typ == "s" {
					index, err := strconv.Atoi(strings.TrimSpace(value))
					if err != nil || index < 0 || index >= len(shared) {
						return "", ErrInvalidDocument
					}
					return shared[index], nil
				}
				return value, nil
			}
		}
	}
}

func decodeXLSXText(decoder *xml.Decoder, start xml.StartElement) (string, error) {
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
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return "", err
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
	text := builder.String()
	if !utf8.ValidString(text) || len([]rune(text)) > maxXLSXOutputChars {
		return "", ErrInvalidDocument
	}
	return text, nil
}

type xlsxTextBuilder struct {
	builder strings.Builder
	chars   int
}

func (b *xlsxTextBuilder) add(value string) error {
	b.chars += len([]rune(value))
	if b.chars > maxXLSXOutputChars {
		return ErrInvalidDocument
	}
	b.builder.WriteString(value)
	return nil
}

func (b *xlsxTextBuilder) String() string { return b.builder.String() }
