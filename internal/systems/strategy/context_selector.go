package strategy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	contextChunkRunes      = 1_200
	contextChunkOverlap    = 120
	contextMaxChunks       = 16
	contextMaxChunksPerRef = 3
	contextMaxTotalRunes   = 24_000
)

// ContextSelector reduces explicitly referenced knowledge into a bounded,
// deterministic set of excerpts suitable for a model prompt.
type ContextSelector interface {
	Select(BriefVersion, []KnowledgeExcerpt) []KnowledgeExcerpt
}

type DeterministicContextSelector struct{}

type scoredKnowledgeChunk struct {
	excerpt        KnowledgeExcerpt
	referenceOrder int
	score          int
}

func (DeterministicContextSelector) Select(brief BriefVersion, references []KnowledgeExcerpt) []KnowledgeExcerpt {
	if len(references) == 0 {
		return []KnowledgeExcerpt{}
	}
	terms := contextQueryTerms(brief.Snapshot)
	grouped := make([][]scoredKnowledgeChunk, 0, len(references))
	for referenceOrder, reference := range references {
		chunks := chunkKnowledgeExcerpt(reference)
		scored := make([]scoredKnowledgeChunk, 0, len(chunks))
		for _, chunk := range chunks {
			score := contextRelevanceScore(chunk, terms)
			chunk.RelevanceScore = score
			scored = append(scored, scoredKnowledgeChunk{
				excerpt: chunk, referenceOrder: referenceOrder, score: score,
			})
		}
		sort.SliceStable(scored, func(i, j int) bool {
			if scored[i].score != scored[j].score {
				return scored[i].score > scored[j].score
			}
			return scored[i].excerpt.ChunkIndex < scored[j].excerpt.ChunkIndex
		})
		if len(scored) > contextMaxChunksPerRef {
			scored = scored[:contextMaxChunksPerRef]
		}
		grouped = append(grouped, scored)
	}

	selected := make([]scoredKnowledgeChunk, 0, contextMaxChunks)
	selectedKeys := map[string]struct{}{}
	totalRunes := 0
	// Preserve coverage first: one best chunk from every referenced document
	// while capacity remains.
	for _, candidates := range grouped {
		if len(candidates) == 0 || len(selected) >= contextMaxChunks {
			continue
		}
		candidate := candidates[0]
		size := len([]rune(candidate.excerpt.Content))
		if totalRunes+size > contextMaxTotalRunes && len(selected) > 0 {
			continue
		}
		selected = append(selected, candidate)
		selectedKeys[candidate.excerpt.ID] = struct{}{}
		totalRunes += size
	}

	extras := make([]scoredKnowledgeChunk, 0)
	for _, candidates := range grouped {
		for _, candidate := range candidates {
			if _, ok := selectedKeys[candidate.excerpt.ID]; !ok {
				extras = append(extras, candidate)
			}
		}
	}
	sort.SliceStable(extras, func(i, j int) bool {
		if extras[i].score != extras[j].score {
			return extras[i].score > extras[j].score
		}
		if extras[i].referenceOrder != extras[j].referenceOrder {
			return extras[i].referenceOrder < extras[j].referenceOrder
		}
		return extras[i].excerpt.ChunkIndex < extras[j].excerpt.ChunkIndex
	})
	for _, candidate := range extras {
		if len(selected) >= contextMaxChunks {
			break
		}
		size := len([]rune(candidate.excerpt.Content))
		if totalRunes+size > contextMaxTotalRunes {
			continue
		}
		selected = append(selected, candidate)
		totalRunes += size
	}

	// Emit chunks in source order so prompt order does not change when two
	// candidates happen to receive the same score.
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].referenceOrder != selected[j].referenceOrder {
			return selected[i].referenceOrder < selected[j].referenceOrder
		}
		return selected[i].excerpt.ChunkIndex < selected[j].excerpt.ChunkIndex
	})
	result := make([]KnowledgeExcerpt, 0, len(selected))
	for _, value := range selected {
		result = append(result, value.excerpt)
	}
	return result
}

func chunkKnowledgeExcerpt(reference KnowledgeExcerpt) []KnowledgeExcerpt {
	content := []rune(strings.TrimSpace(reference.Content))
	if len(content) == 0 {
		return []KnowledgeExcerpt{}
	}
	step := contextChunkRunes - contextChunkOverlap
	chunks := make([]KnowledgeExcerpt, 0, (len(content)+step-1)/step)
	for start, index := 0, 1; start < len(content); start, index = start+step, index+1 {
		end := start + contextChunkRunes
		if end > len(content) {
			end = len(content)
		}
		chunk := reference
		chunk.ReferenceID = reference.ID
		chunk.ID = fmt.Sprintf("%s#chunk-%03d", reference.ID, index)
		chunk.ChunkIndex = index
		chunk.Content = strings.TrimSpace(string(content[start:end]))
		chunk.StartLine = 1 + strings.Count(string(content[:start]), "\n")
		chunk.EndLine = chunk.StartLine + strings.Count(chunk.Content, "\n")
		chunk.Section = contextSectionAt(content, start)
		chunks = append(chunks, chunk)
		if end == len(content) {
			break
		}
	}
	return chunks
}

func contextSectionAt(content []rune, start int) string {
	prefix := string(content[:start])
	lines := strings.Split(prefix, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}

var contextWordPattern = regexp.MustCompile(`[\p{Han}]+|[a-z0-9][a-z0-9_-]*`)

func contextQueryTerms(document BriefDocument) []string {
	values := []string{
		document.Brand.Name,
		document.Product.Name,
		document.Product.Category,
		strings.Join(document.Product.SellingPoints, " "),
		strings.Join(document.Product.Evidence, " "),
		document.Industry,
		document.Region,
		document.Campaign.Objective,
		document.Audience.Primary,
		strings.Join(document.Audience.PainPoints, " "),
		strings.Join(document.Audience.Scenarios, " "),
		document.Proposition,
		strings.Join(document.Channels, " "),
		strings.Join(document.Constraints, " "),
		document.Measurement.PrimaryKPI,
	}
	terms := map[string]struct{}{}
	const maxTerms = 512
	for _, value := range values {
		for _, word := range contextWordPattern.FindAllString(strings.ToLower(value), -1) {
			runes := []rune(word)
			if len(runes) <= 1 {
				continue
			}
			if len(runes) > 64 {
				runes = runes[:64]
				word = string(runes)
			}
			terms[word] = struct{}{}
			if containsHan(runes) {
				for size := 2; size <= 4 && size <= len(runes); size++ {
					for start := 0; start+size <= len(runes); start++ {
						terms[string(runes[start:start+size])] = struct{}{}
						if len(terms) >= maxTerms {
							break
						}
					}
					if len(terms) >= maxTerms {
						break
					}
				}
			}
			if len(terms) >= maxTerms {
				break
			}
		}
		if len(terms) >= maxTerms {
			break
		}
	}
	result := make([]string, 0, len(terms))
	for term := range terms {
		result = append(result, term)
	}
	sort.Strings(result)
	return result
}

func containsHan(value []rune) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func contextRelevanceScore(chunk KnowledgeExcerpt, terms []string) int {
	haystack := strings.ToLower(chunk.Title + "\n" + chunk.Section + "\n" + chunk.Content)
	score := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			weight := len([]rune(term))
			if weight > 8 {
				weight = 8
			}
			score += weight
		}
	}
	return score
}
