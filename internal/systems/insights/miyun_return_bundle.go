package insights

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	MiyunReturnBundleMaxBytes   = int64(500 * 1024 * 1024)
	miyunReturnBundleMaxFiles   = 20
	miyunReturnManifestMaxBytes = int64(2 * 1024 * 1024)
)

type ImportMiyunHandoffReturnBundleRequest struct {
	ExpectedVersion   int64
	Filename          string
	DeclaredSizeBytes int64
	Content           io.ReaderAt
}

type MiyunHandoffReturnBundleResult struct {
	Status          string               `json:"status"`
	Returns         []MiyunHandoffReturn `json:"returns"`
	FailedFilenames []string             `json:"failed_filenames,omitempty"`
}

type miyunReturnBundleFile struct {
	name              string
	size              int64
	open              func() (io.ReadCloser, error)
	association       MiyunReturnAssociationSource
	sourceMaterialID  string
	containerFilename string
}

func (s Service) ImportMiyunHandoffReturnBundle(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, handoffID string, key contract.IdempotencyKey, request ImportMiyunHandoffReturnBundleRequest) (MiyunHandoffReturnBundleResult, error) {
	if key.Validate() != nil || request.ExpectedVersion < 1 || request.Content == nil || request.DeclaredSizeBytes < 1 || request.DeclaredSizeBytes > MiyunReturnBundleMaxBytes {
		return MiyunHandoffReturnBundleResult{}, ErrInvalidRequest
	}
	handoff, err := s.GetMiyunHandoff(ctx, actor, projectID, handoffID)
	if err != nil {
		return MiyunHandoffReturnBundleResult{}, err
	}
	if handoff.Version != request.ExpectedVersion {
		return MiyunHandoffReturnBundleResult{}, ErrVersionConflict
	}
	files, err := readMiyunReturnBundle(request, handoff)
	if err != nil {
		return MiyunHandoffReturnBundleResult{}, err
	}
	result := MiyunHandoffReturnBundleResult{Status: "succeeded", Returns: make([]MiyunHandoffReturn, 0, len(files))}
	for _, file := range files {
		current, getErr := s.GetMiyunHandoff(ctx, actor, projectID, handoffID)
		if getErr != nil {
			return MiyunHandoffReturnBundleResult{}, getErr
		}
		fileKey := miyunReturnBundleKey(string(key), file.name)
		created, createErr := s.CreateMiyunHandoffReturn(ctx, actor, projectID, handoffID, contract.IdempotencyKey(fileKey+"-create"), CreateMiyunHandoffReturnRequest{ExpectedVersion: current.Version})
		if createErr != nil {
			result.FailedFilenames = append(result.FailedFilenames, file.name)
			continue
		}
		if created.Status == MiyunHandoffReturnReturned {
			result.Returns = append(result.Returns, created)
			continue
		}
		reader, openErr := file.open()
		if openErr != nil {
			result.FailedFilenames = append(result.FailedFilenames, file.name)
			continue
		}
		uploaded, uploadErr := s.UploadMiyunHandoffReturn(ctx, actor, projectID, handoffID, created.ID, contract.IdempotencyKey(fileKey+"-upload"), UploadMiyunHandoffReturnRequest{
			ExpectedVersion: current.Version, Filename: file.name, DeclaredMIMEType: MiyunReturnImportMIMEType,
			DeclaredSizeBytes: file.size, SourceMaterialID: file.sourceMaterialID, AssociationSource: file.association,
			ContainerFilename: file.containerFilename, Content: reader,
		})
		_ = reader.Close()
		if uploadErr != nil {
			result.FailedFilenames = append(result.FailedFilenames, file.name)
			continue
		}
		updatedHandoff, returned, markErr := s.MarkMiyunHandoffReturned(ctx, actor, projectID, handoffID, uploaded.ID, contract.IdempotencyKey(fileKey+"-mark"), current.Version)
		if markErr != nil {
			result.FailedFilenames = append(result.FailedFilenames, file.name)
			continue
		}
		_ = updatedHandoff
		result.Returns = append(result.Returns, returned)
	}
	if len(result.FailedFilenames) > 0 {
		result.Status = "partial"
		if len(result.Returns) == 0 {
			result.Status = "failed"
		}
	}
	return result, nil
}

func readMiyunReturnBundle(request ImportMiyunHandoffReturnBundleRequest, handoff MiyunHandoff) ([]miyunReturnBundleFile, error) {
	filename := filepath.Base(strings.TrimSpace(request.Filename))
	if strings.EqualFold(filepath.Ext(filename), ".mp4") {
		association, sourceID, err := resolveMiyunReturnAssociation(handoff, filename, "", "")
		if err != nil {
			return nil, err
		}
		return []miyunReturnBundleFile{{name: filename, size: request.DeclaredSizeBytes, association: association, sourceMaterialID: sourceID, open: func() (io.ReadCloser, error) {
			return io.NopCloser(io.NewSectionReader(request.Content, 0, request.DeclaredSizeBytes)), nil
		}}}, nil
	}
	if !strings.EqualFold(filepath.Ext(filename), ".zip") {
		return nil, ErrInvalidRequest
	}
	archive, err := zip.NewReader(request.Content, request.DeclaredSizeBytes)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	var videos []*zip.File
	var manifest *zip.File
	var total int64
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || entry.Name != path.Base(entry.Name) || strings.Contains(entry.Name, "\\") {
			return nil, fmt.Errorf("%w: return ZIP must be flat", ErrInvalidRequest)
		}
		size := int64(entry.UncompressedSize64)
		if size < 1 {
			return nil, ErrInvalidRequest
		}
		total += size
		if total > MiyunReturnBundleMaxBytes {
			return nil, ErrInvalidRequest
		}
		switch strings.ToLower(filepath.Ext(entry.Name)) {
		case ".mp4":
			if size > 200*1024*1024 {
				return nil, ErrInvalidRequest
			}
			videos = append(videos, entry)
		case ".xlsx":
			if !strings.EqualFold(entry.Name, "manifest.xlsx") || manifest != nil || size > miyunReturnManifestMaxBytes {
				return nil, ErrInvalidRequest
			}
			manifest = entry
		default:
			return nil, fmt.Errorf("%w: return ZIP accepts MP4 files and optional manifest.xlsx only", ErrInvalidRequest)
		}
	}
	if len(videos) < 1 || len(videos) > miyunReturnBundleMaxFiles {
		return nil, ErrInvalidRequest
	}
	mapping := map[string]string{}
	if manifest != nil {
		reader, openErr := manifest.Open()
		if openErr != nil {
			return nil, ErrInvalidRequest
		}
		manifestBytes, readErr := io.ReadAll(io.LimitReader(reader, miyunReturnManifestMaxBytes+1))
		_ = reader.Close()
		if readErr != nil || int64(len(manifestBytes)) > miyunReturnManifestMaxBytes {
			return nil, ErrInvalidRequest
		}
		mapping, err = parseMiyunReturnManifestXLSX(manifestBytes)
		if err != nil {
			return nil, err
		}
	}
	seen := map[string]struct{}{}
	files := make([]miyunReturnBundleFile, 0, len(videos))
	for _, entry := range videos {
		if _, exists := seen[strings.ToLower(entry.Name)]; exists {
			return nil, ErrInvalidRequest
		}
		seen[strings.ToLower(entry.Name)] = struct{}{}
		association, sourceID := MiyunReturnAssociationSource(""), ""
		if mappedID := strings.TrimSpace(mapping[entry.Name]); mappedID != "" {
			association, sourceID = MiyunReturnAssociationManifestXLSX, mappedID
		}
		association, sourceID, err = resolveMiyunReturnAssociation(handoff, entry.Name, association, sourceID)
		if err != nil {
			return nil, err
		}
		zipEntry := entry
		files = append(files, miyunReturnBundleFile{name: entry.Name, size: int64(entry.UncompressedSize64), association: association, sourceMaterialID: sourceID, containerFilename: filename, open: zipEntry.Open})
	}
	for output := range mapping {
		if _, ok := seen[strings.ToLower(output)]; !ok {
			return nil, fmt.Errorf("%w: manifest output file is absent from ZIP", ErrInvalidRequest)
		}
	}
	return files, nil
}

func miyunReturnBundleKey(parent, filename string) string {
	digest := sha256.Sum256([]byte(parent + "\x00" + strings.ToLower(filename)))
	return "mrb-" + hex.EncodeToString(digest[:16])
}

func parseMiyunReturnManifestXLSX(content []byte) (map[string]string, error) {
	archive, err := zip.NewReader(strings.NewReader(string(content)), int64(len(content)))
	if err != nil {
		return nil, ErrInvalidRequest
	}
	parts := map[string][]byte{}
	for _, entry := range archive.File {
		if entry.Name != "xl/sharedStrings.xml" && entry.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			return nil, ErrInvalidRequest
		}
		parts[entry.Name], err = io.ReadAll(io.LimitReader(reader, miyunReturnManifestMaxBytes+1))
		_ = reader.Close()
		if err != nil {
			return nil, ErrInvalidRequest
		}
	}
	if len(parts["xl/worksheets/sheet1.xml"]) == 0 {
		return nil, ErrInvalidRequest
	}
	shared := []string{}
	if data := parts["xl/sharedStrings.xml"]; len(data) > 0 {
		var table struct {
			Items []struct {
				Texts []string `xml:"t"`
			} `xml:"si"`
		}
		if xml.Unmarshal(data, &table) != nil {
			return nil, ErrInvalidRequest
		}
		for _, item := range table.Items {
			shared = append(shared, strings.Join(item.Texts, ""))
		}
	}
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Ref    string `xml:"r,attr"`
				Type   string `xml:"t,attr"`
				Value  string `xml:"v"`
				Inline string `xml:"is>t"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if xml.Unmarshal(parts["xl/worksheets/sheet1.xml"], &sheet) != nil {
		return nil, ErrInvalidRequest
	}
	rows := make([]map[string]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		values := map[string]string{}
		for _, cell := range row.Cells {
			value := cell.Value
			if cell.Type == "s" {
				index, parseErr := strconv.Atoi(value)
				if parseErr != nil || index < 0 || index >= len(shared) {
					return nil, ErrInvalidRequest
				}
				value = shared[index]
			} else if cell.Type == "inlineStr" {
				value = cell.Inline
			}
			values[xlsxColumn(cell.Ref)] = strings.TrimSpace(value)
		}
		rows = append(rows, values)
	}
	headerRow := -1
	columns := map[string]string{}
	for index, row := range rows {
		for column, value := range row {
			columns[strings.ToLower(value)] = column
		}
		if columns["output_filename"] != "" && columns["source_material_id"] != "" {
			headerRow = index
			break
		}
		columns = map[string]string{}
	}
	if headerRow < 0 {
		return nil, fmt.Errorf("%w: manifest.xlsx requires output_filename and source_material_id columns", ErrInvalidRequest)
	}
	result := map[string]string{}
	for _, row := range rows[headerRow+1:] {
		filename, sourceID := strings.TrimSpace(row[columns["output_filename"]]), strings.TrimSpace(row[columns["source_material_id"]])
		if filename == "" && sourceID == "" {
			continue
		}
		if filename == "" || sourceID == "" || !strings.EqualFold(filepath.Ext(filename), ".mp4") {
			return nil, ErrInvalidRequest
		}
		if _, exists := result[filename]; exists {
			return nil, ErrInvalidRequest
		}
		result[filename] = sourceID
	}
	if len(result) == 0 {
		return nil, ErrInvalidRequest
	}
	return result, nil
}

func xlsxColumn(reference string) string {
	for index, character := range reference {
		if character >= '0' && character <= '9' {
			return reference[:index]
		}
	}
	return reference
}
