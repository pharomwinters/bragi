package embedding

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	maxSeqLen     = 512
	clsToken      = "[CLS]"
	sepToken      = "[SEP]"
	unkToken      = "[UNK]"
	padToken      = "[PAD]"
	wordPiecePrefix = "##"
)

// EncodedBatch holds tokenized inputs for a batch of texts.
type EncodedBatch struct {
	InputIDs      []int64
	AttentionMask []int64
	TokenTypeIDs  []int64
	MaxLen        int // padded sequence length
}

// Tokenizer is a basic BERT WordPiece tokenizer.
type Tokenizer struct {
	vocab    map[string]int64
	clsID    int64
	sepID    int64
	unkID    int64
	padID    int64
}

// NewTokenizer loads a BERT tokenizer from a model directory.
// It looks for vocab.txt in the directory.
func NewTokenizer(modelDir string) (*Tokenizer, error) {
	vocabPath := filepath.Join(modelDir, "vocab.txt")

	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("opening vocab.txt: %w", err)
	}
	defer f.Close()

	vocab := make(map[string]int64)
	scanner := bufio.NewScanner(f)
	var idx int64
	for scanner.Scan() {
		token := scanner.Text()
		vocab[token] = idx
		idx++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading vocab.txt: %w", err)
	}

	getID := func(token string) int64 {
		if id, ok := vocab[token]; ok {
			return id
		}
		return 0
	}

	return &Tokenizer{
		vocab: vocab,
		clsID: getID(clsToken),
		sepID: getID(sepToken),
		unkID: getID(unkToken),
		padID: getID(padToken),
	}, nil
}

// EncodeBatch tokenizes a batch of texts with padding to uniform length.
func (t *Tokenizer) EncodeBatch(texts []string) EncodedBatch {
	batchSize := len(texts)

	// Tokenize each text and find max length.
	allIDs := make([][]int64, batchSize)
	maxLen := 0

	for i, text := range texts {
		ids := t.encode(text)
		allIDs[i] = ids
		if len(ids) > maxLen {
			maxLen = len(ids)
		}
	}

	// Build padded tensors.
	inputIDs := make([]int64, batchSize*maxLen)
	attMask := make([]int64, batchSize*maxLen)
	tokenTypeIDs := make([]int64, batchSize*maxLen) // all zeros for single-sequence

	for i := 0; i < batchSize; i++ {
		for j := 0; j < maxLen; j++ {
			offset := i*maxLen + j
			if j < len(allIDs[i]) {
				inputIDs[offset] = allIDs[i][j]
				attMask[offset] = 1
			} else {
				inputIDs[offset] = t.padID
				attMask[offset] = 0
			}
		}
	}

	return EncodedBatch{
		InputIDs:      inputIDs,
		AttentionMask: attMask,
		TokenTypeIDs:  tokenTypeIDs,
		MaxLen:        maxLen,
	}
}

// encode tokenizes a single text into token IDs.
func (t *Tokenizer) encode(text string) []int64 {
	// Basic normalization: lowercase and clean whitespace.
	text = strings.ToLower(strings.TrimSpace(text))

	// Tokenize into words.
	words := basicTokenize(text)

	// WordPiece each word.
	var ids []int64
	ids = append(ids, t.clsID)

	for _, word := range words {
		wpIDs := t.wordPiece(word)
		ids = append(ids, wpIDs...)

		// Truncate if we've hit max length (leave room for [SEP]).
		if len(ids) >= maxSeqLen-1 {
			ids = ids[:maxSeqLen-1]
			break
		}
	}

	ids = append(ids, t.sepID)
	return ids
}

// wordPiece splits a word into subword tokens using the WordPiece algorithm.
func (t *Tokenizer) wordPiece(word string) []int64 {
	if _, ok := t.vocab[word]; ok {
		return []int64{t.vocab[word]}
	}

	var ids []int64
	remaining := word

	for len(remaining) > 0 {
		// Find the longest matching prefix in vocab.
		matched := false
		for end := len(remaining); end > 0; end-- {
			sub := remaining[:end]
			if len(ids) > 0 {
				sub = wordPiecePrefix + sub
			}
			if id, ok := t.vocab[sub]; ok {
				ids = append(ids, id)
				remaining = remaining[end:]
				matched = true
				break
			}
		}
		if !matched {
			ids = append(ids, t.unkID)
			break
		}
	}

	return ids
}

// basicTokenize splits text on whitespace and punctuation boundaries.
func basicTokenize(text string) []string {
	var tokens []string
	var buf strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
		} else if unicode.IsPunct(r) || isChinesePunct(r) {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			buf.WriteRune(r)
		}
	}

	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}

	return tokens
}

// isChinesePunct checks for CJK punctuation that unicode.IsPunct might miss.
func isChinesePunct(r rune) bool {
	return (r >= 0x3000 && r <= 0x303F) ||
		(r >= 0xFF00 && r <= 0xFFEF)
}

// ApproxTokenCount gives a rough estimate of the number of tokens in a text.
// Useful for pre-checking chunk sizes before actual tokenization.
func ApproxTokenCount(text string) int {
	words := len(strings.Fields(text))
	// Average ~1.3 tokens per whitespace-separated word for English.
	return int(float64(words) * 1.3)
}
