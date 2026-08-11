package insights

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode"
)

var miyunHandoffManifestV1Columns = []string{
	"manifest_version", "handoff_id", "handoff_version", "source_material_name", "source_file",
	"miyun_material_id", "source_url", "source", "delivery_days", "cumulative_impressions",
	"related_ads", "related_creators", "target_product", "target_category", "product_media_files",
	"product_document_files", "notes", "juliang_spend", "parameter_version", "input_hash",
}

var miyunHandoffManifestColumns = []string{
	"manifest_version", "handoff_id", "handoff_version", "source_material_id", "source_file",
	"product_media_files", "product_document_files", "parameter_version", "input_hash",
}

// MiyunHandoffManifest is the frozen, draft schema rendered in manifest.csv.
// All fields are strings so an absent value is represented by the CSV empty
// value and never by an invented numeric zero.
type MiyunHandoffManifest struct {
	HandoffID             string
	HandoffVersion        string
	SourceMaterialIDs     []string
	SourceMaterialName    string
	MiyunMaterialID       string
	SourceURL             string
	Source                string
	DeliveryDays          string
	CumulativeImpressions string
	RelatedAds            string
	RelatedCreators       string
	TargetProduct         string
	TargetCategory        string
	Notes                 string
	ParameterVersion      string
	InputHash             string
}

// MiyunHandoffExportFile is a frozen file reference. Name is presentation
// metadata only; ExportMiyunHandoffZIP determines the ZIP path itself.
type MiyunHandoffExportFile struct {
	Reference string
	Name      string
	SizeBytes int64
	SHA256    string
}

// MiyunHandoffExportSnapshot contains only frozen handoff data. Callers must
// not populate it from live product/profile state at export time.
type MiyunHandoffExportSnapshot struct {
	ManifestVersion string
	Manifest        MiyunHandoffManifest
	Sources         []MiyunHandoffExportFile
	ProductMedia    []MiyunHandoffExportFile
	ProductDocs     []MiyunHandoffExportFile
}

// MiyunHandoffExportReader is the authorized, project-scoped file boundary.
// It deliberately exposes neither a BlobStore nor storage paths.
type MiyunHandoffExportReader interface {
	OpenMiyunHandoffExportFile(context.Context, MiyunHandoffExportFile) (io.ReadCloser, error)
}

// ExportMiyunHandoffZIP writes a complete ZIP incrementally. A successful
// return means all readers, ZIP entries, and the ZIP central directory closed
// successfully; callers can then safely transition an export to exported.
func ExportMiyunHandoffZIP(ctx context.Context, output io.Writer, snapshot MiyunHandoffExportSnapshot, reader MiyunHandoffExportReader) error {
	if output == nil || reader == nil {
		return errors.New("Miyun handoff export output and reader are required")
	}
	if snapshot.ManifestVersion != MiyunHandoffManifestV1 && snapshot.ManifestVersion != MiyunHandoffManifestVersion {
		return fmt.Errorf("unsupported Miyun handoff manifest version %q", snapshot.ManifestVersion)
	}
	if len(snapshot.Sources) == 0 {
		return errors.New("Miyun handoff export source files are required")
	}
	for _, source := range snapshot.Sources {
		if strings.TrimSpace(source.Reference) == "" {
			return errors.New("Miyun handoff export source file is required")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	archive := zip.NewWriter(output)
	if err := writeMiyunHandoffDirectory(archive, "viral/source/"); err != nil {
		return err
	}
	if err := writeMiyunHandoffDirectory(archive, "product/media/"); err != nil {
		return err
	}
	if err := writeMiyunHandoffDirectory(archive, "product/docs/"); err != nil {
		return err
	}

	sourceNames := miyunHandoffFileNames(snapshot.Sources)
	mediaNames := miyunHandoffFileNames(snapshot.ProductMedia)
	docNames := miyunHandoffFileNames(snapshot.ProductDocs)
	if err := writeMiyunHandoffManifest(archive, snapshot.ManifestVersion, snapshot.Manifest, sourceNames, mediaNames, docNames); err != nil {
		return err
	}
	for index, source := range snapshot.Sources {
		if err := writeMiyunHandoffFile(ctx, archive, "viral/source/"+sourceNames[index], source, reader); err != nil {
			return err
		}
	}
	for index, file := range snapshot.ProductMedia {
		if err := writeMiyunHandoffFile(ctx, archive, "product/media/"+mediaNames[index], file, reader); err != nil {
			return err
		}
	}
	for index, file := range snapshot.ProductDocs {
		if err := writeMiyunHandoffFile(ctx, archive, "product/docs/"+docNames[index], file, reader); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return archive.Close()
}

func writeMiyunHandoffDirectory(archive *zip.Writer, name string) error {
	_, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	return err
}

func writeMiyunHandoffManifest(archive *zip.Writer, manifestVersion string, manifest MiyunHandoffManifest, sourceNames, mediaNames, docNames []string) error {
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: "manifest.csv", Method: zip.Deflate})
	if err != nil {
		return err
	}
	if _, err := entry.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	writer := csv.NewWriter(entry)
	if manifestVersion == MiyunHandoffManifestV1 {
		if err := writer.Write(miyunHandoffManifestV1Columns); err != nil {
			return err
		}
		if err := writer.Write([]string{
			manifestVersion, manifest.HandoffID, manifest.HandoffVersion, manifest.SourceMaterialName,
			joinMiyunHandoffPaths("viral/source/", sourceNames), manifest.MiyunMaterialID, manifest.SourceURL, manifest.Source,
			manifest.DeliveryDays, manifest.CumulativeImpressions, manifest.RelatedAds, manifest.RelatedCreators,
			manifest.TargetProduct, manifest.TargetCategory, joinMiyunHandoffPaths("product/media/", mediaNames),
			joinMiyunHandoffPaths("product/docs/", docNames), manifest.Notes, "", manifest.ParameterVersion, manifest.InputHash,
		}); err != nil {
			return err
		}
	} else {
		if len(manifest.SourceMaterialIDs) != len(sourceNames) {
			return fmt.Errorf("Miyun handoff manifest source identity mismatch")
		}
		if err := writer.Write(miyunHandoffManifestColumns); err != nil {
			return err
		}
		for index, sourceName := range sourceNames {
			if err := writer.Write([]string{
				manifestVersion, manifest.HandoffID, manifest.HandoffVersion, manifest.SourceMaterialIDs[index],
				"viral/source/" + sourceName, joinMiyunHandoffPaths("product/media/", mediaNames),
				joinMiyunHandoffPaths("product/docs/", docNames), manifest.ParameterVersion, manifest.InputHash,
			}); err != nil {
				return err
			}
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeMiyunHandoffFile(ctx context.Context, archive *zip.Writer, entryName string, file MiyunHandoffExportFile, source MiyunHandoffExportReader) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	entry, err := archive.CreateHeader(&zip.FileHeader{Name: entryName, Method: zip.Deflate})
	if err != nil {
		return err
	}
	content, err := source.OpenMiyunHandoffExportFile(ctx, file)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := content.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	hash := sha256.New()
	count, copyErr := io.Copy(entry, io.TeeReader(&miyunHandoffContextReader{ctx: ctx, reader: content}, hash))
	if copyErr != nil {
		return copyErr
	}
	if file.SizeBytes > 0 && count != file.SizeBytes {
		return fmt.Errorf("Miyun handoff file size mismatch")
	}
	if file.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), file.SHA256) {
		return fmt.Errorf("Miyun handoff file hash mismatch")
	}
	err = nil
	return err
}

type miyunHandoffContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *miyunHandoffContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func miyunHandoffFileNames(files []MiyunHandoffExportFile) []string {
	seen := make(map[string]int, len(files))
	names := make([]string, len(files))
	for index, file := range files {
		base := cleanMiyunHandoffFileName(file.Name)
		key := strings.ToLower(base)
		seen[key]++
		if seen[key] == 1 {
			names[index] = base
			continue
		}
		extension := path.Ext(base)
		stem := strings.TrimSuffix(base, extension)
		candidate := fmt.Sprintf("%s (%d)%s", stem, seen[key], extension)
		for {
			candidateKey := strings.ToLower(candidate)
			if seen[candidateKey] == 0 {
				seen[candidateKey] = 1
				names[index] = candidate
				break
			}
			seen[key]++
			candidate = fmt.Sprintf("%s (%d)%s", stem, seen[key], extension)
		}
	}
	return names
}

func cleanMiyunHandoffFileName(value string) string {
	value = path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	var result strings.Builder
	for _, character := range value {
		switch {
		case unicode.IsControl(character), strings.ContainsRune("\\/:*?\"<>|", character):
			result.WriteByte('-')
		default:
			result.WriteRune(character)
		}
	}
	name := strings.Trim(strings.TrimSpace(result.String()), ". ")
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	if isMiyunHandoffWindowsReservedName(name) {
		return "file" + path.Ext(name)
	}
	return name
}

func isMiyunHandoffWindowsReservedName(name string) bool {
	base := strings.ToUpper(strings.TrimSuffix(name, path.Ext(name)))
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

func joinMiyunHandoffPaths(directory string, names []string) string {
	paths := make([]string, len(names))
	for index, name := range names {
		paths[index] = directory + name
	}
	return strings.Join(paths, ";")
}
