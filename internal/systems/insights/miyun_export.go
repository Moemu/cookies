package insights

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode"
)

type MiyunHandoffPackageKind string

const (
	MiyunHandoffPackageSources MiyunHandoffPackageKind = "sources"
	MiyunHandoffPackageProject MiyunHandoffPackageKind = "project"
)

// MiyunHandoffManifest is frozen lineage retained by the handoff record. The
// upload ZIPs deliberately contain binaries only; metadata never appears as a
// third kind of file in either material package.
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
// metadata only; the package exporter determines the flat ZIP entry name.
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

// ExportMiyunHandoffPackageZIP writes one of the two upload-ready packages.
// The sources package contains only Miyun MP4s; the project package contains
// only frozen project media/documents. Every entry is stored at the ZIP root.
func ExportMiyunHandoffPackageZIP(ctx context.Context, output io.Writer, snapshot MiyunHandoffExportSnapshot, packageKind MiyunHandoffPackageKind, reader MiyunHandoffExportReader) error {
	if output == nil || reader == nil {
		return errors.New("Miyun handoff export output and reader are required")
	}
	if packageKind != MiyunHandoffPackageSources && packageKind != MiyunHandoffPackageProject {
		return fmt.Errorf("unsupported Miyun handoff package %q", packageKind)
	}
	if snapshot.ManifestVersion != MiyunHandoffManifestV1 && snapshot.ManifestVersion != MiyunHandoffManifestV2 && snapshot.ManifestVersion != MiyunHandoffManifestVersion {
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
	sourceNames := miyunHandoffSourceFileNames(snapshot)
	productFiles := append(append([]MiyunHandoffExportFile{}, snapshot.ProductMedia...), snapshot.ProductDocs...)
	productNames := miyunHandoffFileNames(productFiles)
	if packageKind == MiyunHandoffPackageSources {
		for index, source := range snapshot.Sources {
			if err := writeMiyunHandoffFile(ctx, archive, sourceNames[index], source, reader); err != nil {
				return err
			}
		}
	} else {
		for index, file := range productFiles {
			if err := writeMiyunHandoffFile(ctx, archive, productNames[index], file, reader); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return archive.Close()
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

func miyunHandoffSourceFileNames(snapshot MiyunHandoffExportSnapshot) []string {
	files := append([]MiyunHandoffExportFile{}, snapshot.Sources...)
	if len(snapshot.Manifest.SourceMaterialIDs) == len(files) {
		for index := range files {
			files[index].Name = "miyun_" + snapshot.Manifest.SourceMaterialIDs[index] + ".mp4"
		}
	}
	return miyunHandoffFileNames(files)
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
