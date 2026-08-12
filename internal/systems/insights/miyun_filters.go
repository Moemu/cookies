package insights

import (
	"fmt"
	"sort"
	"strings"
)

// MiyunMaterialFilterCatalogVersion pins the YouShu filter IDs used by a
// confirmed product profile and its immutable crawl snapshot.
const MiyunMaterialFilterCatalogVersion = "youshu-material-filter/v1"

type miyunFilterOption struct {
	ID      string
	Label   string
	Aliases []string
}

var miyunMTypeOptions = []miyunFilterOption{
	{ID: "100", Label: "纯文案", Aliases: []string{"text", "pure_text"}},
	{ID: "102", Label: "单图", Aliases: []string{"image", "single_image"}},
	{ID: "103", Label: "GIF", Aliases: []string{"gif"}},
	{ID: "104", Label: "组图", Aliases: []string{"gallery", "image_group"}},
	{ID: "201", Label: "视频", Aliases: []string{"video"}},
	{ID: "202", Label: "竖视频", Aliases: []string{"vertical_video"}},
}

var miyunMaterialTagOptions = []miyunFilterOption{
	{ID: "1", Label: "单人口播", Aliases: []string{"single_speaker", "talking_head"}},
	{ID: "2", Label: "多人口播", Aliases: []string{"multiple_speakers", "multi_speaker"}},
	{ID: "6", Label: "真人展示", Aliases: []string{"human_product_demo_no_voice", "human_demo"}},
	{ID: "4", Label: "商品口播", Aliases: []string{"product_voiceover", "product_narration"}},
	{ID: "5", Label: "商品展示", Aliases: []string{"product_demo_no_voice", "product_demo", "product"}},
	{ID: "3", Label: "剧情演绎", Aliases: []string{"story", "story_drama"}},
}

func normalizeMiyunMTypes(values []string) ([]string, error) {
	return normalizeMiyunFilterIDs("material type", values, miyunMTypeOptions)
}

func normalizeMiyunMaterialTags(values []string) ([]string, error) {
	return normalizeMiyunFilterIDs("material content type", values, miyunMaterialTagOptions)
}

func normalizeMiyunFilterIDs(name string, values []string, options []miyunFilterOption) ([]string, error) {
	lookup := make(map[string]string, len(options)*3)
	for _, option := range options {
		lookup[strings.ToLower(option.ID)] = option.ID
		lookup[strings.ToLower(option.Label)] = option.ID
		for _, alias := range option.Aliases {
			lookup[strings.ToLower(alias)] = option.ID
		}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		id, ok := lookup[key]
		if !ok {
			return nil, fmt.Errorf("%w: unsupported Miyun %s %q for catalog %s", ErrInvalidRequest, name, value, MiyunMaterialFilterCatalogVersion)
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func miyunFilterValue(values []string) *string {
	if len(values) == 0 {
		return nil
	}
	value := strings.Join(values, ",")
	return &value
}

func miyunMTypeFromMediaFormat(code string) string {
	switch strings.TrimSpace(code) {
	case "single_image":
		return "102"
	case "gif":
		return "103"
	case "image_group":
		return "104"
	case "video":
		return "201"
	case "vertical_video":
		return "202"
	default:
		return ""
	}
}

func miyunMaterialTagFromContentStyle(code string) string {
	values, err := normalizeMiyunMaterialTags([]string{code})
	if err != nil || len(values) == 0 {
		return ""
	}
	return values[0]
}
