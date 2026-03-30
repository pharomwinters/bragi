// Package search provides document chunking, indexing, and semantic search.
package search

import (
	"strings"
	"unicode"
)

const (
	// minChunkWords is the minimum word count for a standalone chunk.
	// Paragraphs below this threshold are merged with the next paragraph.
	minChunkWords = 50

	// maxChunkChars is the maximum character count before a chunk is split
	// at a sentence boundary.
	maxChunkChars = 2000
)

// Chunk represents a fragment of a document suitable for embedding.
type Chunk struct {
	Index     int    // 0-based position within the document
	Content   string // the chunk text
	Heading   string // nearest heading above this chunk (empty if none)
	StartLine int    // 1-based start line in the original document
	EndLine   int    // 1-based end line in the original document
}

// ChunkDocument splits a markdown body into chunks using the given strategy.
// Strategy must be "paragraph" or "heading". Defaults to "paragraph" if unrecognized.
func ChunkDocument(body string, strategy string) []Chunk {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	switch strategy {
	case "heading":
		return chunkByHeading(body)
	default:
		return chunkByParagraph(body)
	}
}

// chunkByParagraph splits on double newlines, merges small adjacent chunks,
// and splits oversized chunks at sentence boundaries.
func chunkByParagraph(body string) []Chunk {
	lines := strings.Split(body, "\n")

	// First pass: identify paragraph boundaries (blank-line separated).
	type rawChunk struct {
		lines     []string
		startLine int // 1-based
	}

	var raws []rawChunk
	var current []string
	startLine := 1

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				raws = append(raws, rawChunk{lines: current, startLine: startLine})
				current = nil
			}
			startLine = i + 2 // next line is 1-based
		} else {
			if len(current) == 0 {
				startLine = i + 1
			}
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		raws = append(raws, rawChunk{lines: current, startLine: startLine})
	}

	if len(raws) == 0 {
		return nil
	}

	// Second pass: merge small chunks (but never across heading boundaries).
	var merged []rawChunk

	for i := 0; i < len(raws); i++ {
		text := strings.Join(raws[i].lines, "\n")

		// Don't merge if next chunk starts with a heading.
		nextIsHeading := false
		if i+1 < len(raws) && len(raws[i+1].lines) > 0 {
			nextIsHeading = extractHeading(raws[i+1].lines[0]) != ""
		}

		if wordCount(text) < minChunkWords && i+1 < len(raws) && !nextIsHeading {
			nextLines := make([]string, len(raws[i+1].lines))
			copy(nextLines, raws[i+1].lines)
			combined := rawChunk{
				lines:     append(append(append([]string{}, raws[i].lines...), ""), nextLines...),
				startLine: raws[i].startLine,
			}
			raws[i+1] = combined
			continue
		}

		merged = append(merged, raws[i])
	}

	// Third pass: split oversized chunks, assign headings.
	var chunks []Chunk
	currentHeading := ""

	for _, raw := range merged {
		text := strings.Join(raw.lines, "\n")

		// Scan all lines for headings to track context.
		for _, line := range raw.lines {
			if h := extractHeading(line); h != "" {
				currentHeading = h
			}
		}

		endLine := raw.startLine + len(raw.lines) - 1

		if len(text) <= maxChunkChars {
			chunks = append(chunks, Chunk{
				Index:     len(chunks),
				Content:   text,
				Heading:   currentHeading,
				StartLine: raw.startLine,
				EndLine:   endLine,
			})
		} else {
			// Split at sentence boundaries.
			sentences := splitSentences(text)
			var buf strings.Builder
			chunkStart := raw.startLine

			for _, sent := range sentences {
				if buf.Len()+len(sent) > maxChunkChars && buf.Len() > 0 {
					content := buf.String()
					lineCount := strings.Count(content, "\n")
					chunks = append(chunks, Chunk{
						Index:     len(chunks),
						Content:   content,
						Heading:   currentHeading,
						StartLine: chunkStart,
						EndLine:   chunkStart + lineCount,
					})
					chunkStart = chunkStart + lineCount + 1
					buf.Reset()
				}
				buf.WriteString(sent)
			}

			if buf.Len() > 0 {
				content := buf.String()
				lineCount := strings.Count(content, "\n")
				chunks = append(chunks, Chunk{
					Index:     len(chunks),
					Content:   content,
					Heading:   currentHeading,
					StartLine: chunkStart,
					EndLine:   chunkStart + lineCount,
				})
			}
		}
	}

	return chunks
}

// chunkByHeading splits on ATX headings (# through ######).
// Each heading starts a new chunk. Content before the first heading is chunk 0.
func chunkByHeading(body string) []Chunk {
	lines := strings.Split(body, "\n")

	type section struct {
		heading   string
		lines     []string
		startLine int // 1-based
	}

	var sections []section
	var current section
	current.startLine = 1

	for i, line := range lines {
		if h := extractHeading(line); h != "" {
			// Save current section if it has content.
			if len(current.lines) > 0 || current.heading != "" {
				sections = append(sections, current)
			}
			current = section{
				heading:   h,
				lines:     []string{line},
				startLine: i + 1,
			}
		} else {
			current.lines = append(current.lines, line)
		}
	}

	// Save final section.
	if len(current.lines) > 0 || current.heading != "" {
		sections = append(sections, current)
	}

	// Convert to chunks, skipping empty sections.
	var chunks []Chunk
	for _, sec := range sections {
		text := strings.TrimSpace(strings.Join(sec.lines, "\n"))
		if text == "" {
			continue
		}

		chunks = append(chunks, Chunk{
			Index:     len(chunks),
			Content:   text,
			Heading:   sec.heading,
			StartLine: sec.startLine,
			EndLine:   sec.startLine + len(sec.lines) - 1,
		})
	}

	return chunks
}

// extractHeading returns the heading text if the line is an ATX heading, or "".
func extractHeading(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return ""
	}

	// Count leading # characters.
	level := 0
	for _, r := range trimmed {
		if r == '#' {
			level++
		} else {
			break
		}
	}

	if level < 1 || level > 6 {
		return ""
	}

	rest := strings.TrimSpace(trimmed[level:])
	if rest == "" && level > 0 {
		return "" // lone "#" with no text
	}

	// Heading must have space after #.
	if len(trimmed) > level && trimmed[level] != ' ' {
		return ""
	}

	return rest
}

// splitSentences splits text into sentence-like fragments.
func splitSentences(text string) []string {
	var sentences []string
	var buf strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		buf.WriteRune(runes[i])

		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Look ahead for whitespace or end of text.
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				sentences = append(sentences, buf.String())
				buf.Reset()
			}
		}
	}

	if buf.Len() > 0 {
		sentences = append(sentences, buf.String())
	}

	return sentences
}

// wordCount counts whitespace-separated words.
func wordCount(s string) int {
	return len(strings.Fields(s))
}
