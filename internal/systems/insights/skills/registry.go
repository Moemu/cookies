// Package skills 持有素材洞察的特征提取 Skill 定义。
//
// 与 internal/systems/strategy/skills 同构，是有意的：两处都需要「把提示词
// 当成有版本、可哈希、能被追溯的制品」，而不是散落在代码里的字符串常量。
// 03 §344 验收 12 要求任一分析结论能回溯到 Skill 版本——版本号写在这里，
// 内容哈希由这里算出，两者一起写进 AnalysisRun。
//
// ContentHash 存在的理由：**版本号是人手写的，会忘记改**。改了指令却没动版本号，
// 库里两批结果就会顶着同一个版本号，而它们其实不可比。哈希不会忘。
package skills

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed extraction/*.json
var skillFiles embed.FS

// Skill 是一类素材的提取指令。
//
// 注意这里**没有输出格式**：输出格式由 features.go 的特征体系生成
// （见 extraction_schema.go），不在 Skill 里重复写一份。写两份的话，
// 加一个特征字段就得改两个地方，而漏改的那次不会报错，只会安静地少提一个字段。
type Skill struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	AssetType string `json:"asset_type"`
	// Persona 是给模型的角色设定，进 system 消息。
	Persona string `json:"persona"`
	// Instructions 是提取时要遵守的规则，进 system 消息。
	Instructions []string `json:"instructions"`
	// ReviewFocus 给人看：这类素材的提取结果里，哪几项最容易错、要重点复核。
	// 不进提示词——告诉模型「你容易在这里出错」并不会让它不出错，
	// 但告诉复核的人有用。
	ReviewFocus []string `json:"review_focus"`
}

// Snapshot 是写进 AnalysisRun 的那份留痕。
type Snapshot struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	ContentHash  string   `json:"content_hash"`
	Persona      string   `json:"persona"`
	Instructions []string `json:"instructions"`
	ReviewFocus  []string `json:"review_focus"`
}

type Registry struct {
	byAssetType map[string]Skill
	ordered     []Skill
}

// DefaultRegistry 读入全部内嵌定义。任何一份不合法就整个失败——
// **不跳过坏文件**：跳过意味着那类素材会在运行时才发现没有 Skill，
// 而那时用户已经点了按钮在等结果了。
func DefaultRegistry() (Registry, error) {
	byAssetType := map[string]Skill{}
	var ordered []Skill
	err := fs.WalkDir(skillFiles, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		content, err := skillFiles.ReadFile(path)
		if err != nil {
			return err
		}
		var skill Skill
		if err := json.Unmarshal(content, &skill); err != nil {
			return fmt.Errorf("解析特征提取 Skill %s 失败：%w", path, err)
		}
		if err := skill.Validate(); err != nil {
			return fmt.Errorf("校验特征提取 Skill %s 失败：%w", path, err)
		}
		if existing, duplicated := byAssetType[skill.AssetType]; duplicated {
			// 一类素材两份 Skill，选哪一份取决于文件遍历顺序——那等于结果不可复现。
			return fmt.Errorf("素材类型 %s 有两份 Skill：%s 和 %s", skill.AssetType, existing.Name, skill.Name)
		}
		byAssetType[skill.AssetType] = skill
		ordered = append(ordered, skill)
		return nil
	})
	if err != nil {
		return Registry{}, err
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].AssetType < ordered[j].AssetType })
	return Registry{byAssetType: byAssetType, ordered: ordered}, nil
}

func (s Skill) Validate() error {
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Version) == "" {
		return fmt.Errorf("name 和 version 必填")
	}
	if strings.TrimSpace(s.AssetType) == "" {
		return fmt.Errorf("asset_type 必填：Skill 是按素材类型选的")
	}
	if strings.TrimSpace(s.Persona) == "" {
		return fmt.Errorf("persona 必填")
	}
	if len(s.Instructions) == 0 {
		return fmt.Errorf("instructions 必填")
	}
	if len(s.ReviewFocus) == 0 {
		// 复核重点为空等于对复核的人说「随便看看」，那这一层就白设了。
		return fmt.Errorf("review_focus 必填：人工复核需要知道这类素材哪里最容易错")
	}
	return nil
}

// For 按素材类型取 Skill。
func (r Registry) For(assetType string) (Snapshot, bool) {
	skill, ok := r.byAssetType[assetType]
	if !ok {
		return Snapshot{}, false
	}
	return skill.snapshot(), true
}

// All 按素材类型顺序返回全部，供能力运营的 Skills 视图展示。
func (r Registry) All() []Snapshot {
	snapshots := make([]Snapshot, 0, len(r.ordered))
	for _, skill := range r.ordered {
		snapshots = append(snapshots, skill.snapshot())
	}
	return snapshots
}

func (s Skill) snapshot() Snapshot {
	// 对整个 Skill 结构算哈希：改了 persona、指令或复核重点里的任何一句，
	// 哈希都会变。这正是要的——它们都会影响或解释产出。
	encoded, _ := json.Marshal(s)
	sum := sha256.Sum256(encoded)
	return Snapshot{
		Name: s.Name, Version: s.Version, ContentHash: hex.EncodeToString(sum[:]),
		Persona:      s.Persona,
		Instructions: append([]string(nil), s.Instructions...),
		ReviewFocus:  append([]string(nil), s.ReviewFocus...),
	}
}
