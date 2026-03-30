// Package embedding provides vector embedding generation for text.
// It uses ONNX Runtime to run embedding models locally.
package embedding

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
)

// Vector is a float32 embedding vector.
type Vector = []float32

// maxBatchSize limits the number of texts processed in a single ONNX inference call.
const maxBatchSize = 32

// Provider generates embedding vectors from text.
type Provider interface {
	// Embed generates embeddings for document texts (prefixed with "search_document: ").
	Embed(ctx context.Context, texts []string) ([]Vector, error)

	// EmbedQuery generates an embedding for a search query (prefixed with "search_query: ").
	EmbedQuery(ctx context.Context, query string) (Vector, error)

	// Dimensions returns the embedding vector dimensionality.
	Dimensions() int

	// Close releases resources held by the provider.
	Close() error
}

// ONNXProvider generates embeddings using an ONNX model via ONNX Runtime.
type ONNXProvider struct {
	session    *ort.DynamicAdvancedSession
	tokenizer  *Tokenizer
	dims       int
	modelDir   string
	numInputs  int      // how many model inputs (2 or 3)
	numOutputs int      // how many model outputs
	inputNames []string // cached for diagnostics
}

// NewONNXProvider creates a new embedding provider from an ONNX model directory.
// The directory must contain an ONNX model file and vocab.txt.
func NewONNXProvider(modelDir string) (*ONNXProvider, error) {
	if !ort.IsInitialized() {
		ort.SetSharedLibraryPath(findONNXRuntimeLib())
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("initializing ONNX Runtime: %w", err)
		}
	}

	modelPath, err := findModelFile(modelDir)
	if err != nil {
		return nil, err
	}

	tokenizer, err := NewTokenizer(modelDir)
	if err != nil {
		return nil, fmt.Errorf("loading tokenizer: %w", err)
	}

	// Discover actual input/output names from the model file instead of guessing.
	modelInputs, modelOutputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("inspecting ONNX model: %w", err)
	}

	inputNames := make([]string, len(modelInputs))
	for i, info := range modelInputs {
		inputNames[i] = info.Name
	}
	outputNames := make([]string, len(modelOutputs))
	for i, info := range modelOutputs {
		outputNames[i] = info.Name
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("creating ONNX session (inputs=%v, outputs=%v): %w",
			inputNames, outputNames, err)
	}

	// Determine dimensions from a test inference.
	dims, err := probeDimensions(session, tokenizer, len(inputNames))
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("probing embedding dimensions: %w", err)
	}

	return &ONNXProvider{
		session:    session,
		tokenizer:  tokenizer,
		dims:       dims,
		modelDir:   modelDir,
		numInputs:  len(inputNames),
		numOutputs: len(outputNames),
		inputNames: inputNames,
	}, nil
}

func (p *ONNXProvider) Dimensions() int {
	return p.dims
}

func (p *ONNXProvider) Close() error {
	if p.session != nil {
		return p.session.Destroy()
	}
	return nil
}

// Embed generates document embeddings (prefixed with "search_document: ").
func (p *ONNXProvider) Embed(ctx context.Context, texts []string) ([]Vector, error) {
	prefixed := make([]string, len(texts))
	for i, t := range texts {
		prefixed[i] = "search_document: " + t
	}
	return p.embedBatch(ctx, prefixed)
}

// EmbedQuery generates a query embedding (prefixed with "search_query: ").
func (p *ONNXProvider) EmbedQuery(ctx context.Context, query string) (Vector, error) {
	vecs, err := p.embedBatch(ctx, []string{"search_query: " + query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	return vecs[0], nil
}

// embedBatch processes texts in batches of maxBatchSize.
func (p *ONNXProvider) embedBatch(ctx context.Context, texts []string) ([]Vector, error) {
	results := make([]Vector, 0, len(texts))

	for i := 0; i < len(texts); i += maxBatchSize {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		end := i + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		vecs, err := p.embedSingle(texts[i:end])
		if err != nil {
			return nil, fmt.Errorf("embedding batch starting at %d: %w", i, err)
		}
		results = append(results, vecs...)
	}

	return results, nil
}

// embedSingle runs inference on a single batch.
func (p *ONNXProvider) embedSingle(texts []string) ([]Vector, error) {
	encoded := p.tokenizer.EncodeBatch(texts)
	batchSize := len(texts)
	seqLen := encoded.MaxLen

	shape := ort.NewShape(int64(batchSize), int64(seqLen))

	inputIDs, err := ort.NewTensor(shape, encoded.InputIDs)
	if err != nil {
		return nil, fmt.Errorf("creating input_ids tensor: %w", err)
	}
	defer inputIDs.Destroy()

	attMask, err := ort.NewTensor(shape, encoded.AttentionMask)
	if err != nil {
		return nil, fmt.Errorf("creating attention_mask tensor: %w", err)
	}
	defer attMask.Destroy()

	// Build inputs list — some models have 2 inputs (no token_type_ids), others 3.
	inputs := []ort.Value{inputIDs, attMask}
	if p.numInputs >= 3 {
		tokenTypeIDs, err := ort.NewTensor(shape, encoded.TokenTypeIDs)
		if err != nil {
			return nil, fmt.Errorf("creating token_type_ids tensor: %w", err)
		}
		defer tokenTypeIDs.Destroy()
		inputs = append(inputs, tokenTypeIDs)
	}

	// Pre-allocate outputs: nil = let ONNX Runtime allocate each.
	outputs := make([]ort.Value, p.numOutputs)

	err = p.session.Run(inputs, outputs)
	if err != nil {
		return nil, fmt.Errorf("running inference: %w", err)
	}
	// Destroy all outputs when done.
	for _, o := range outputs {
		if o != nil {
			defer o.Destroy()
		}
	}

	// Use the first output (primary embedding output).
	if outputs[0] == nil {
		return nil, fmt.Errorf("output tensor is nil")
	}

	outputShape := outputs[0].GetShape()
	if len(outputShape) < 2 {
		return nil, fmt.Errorf("unexpected output shape: %v", outputShape)
	}

	// Get raw float32 data from the output tensor.
	outputTensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("output tensor is not float32")
	}
	outputData := outputTensor.GetData()

	vectors := make([]Vector, batchSize)

	if len(outputShape) == 2 {
		// Output is [batch_size, hidden_size] — already pooled by the model.
		hiddenSize := int(outputShape[1])
		for b := 0; b < batchSize; b++ {
			vec := make(Vector, hiddenSize)
			copy(vec, outputData[b*hiddenSize:(b+1)*hiddenSize])
			vectors[b] = l2Normalize(vec)
		}
	} else {
		// Output is [batch_size, seq_len, hidden_size] — apply mean pooling.
		hiddenSize := int(outputShape[2])
		for b := 0; b < batchSize; b++ {
			vec := make(Vector, hiddenSize)
			tokenCount := float32(0)

			for t := 0; t < seqLen; t++ {
				maskIdx := b*seqLen + t
				if encoded.AttentionMask[maskIdx] == 0 {
					continue
				}
				tokenCount++

				offset := (b*seqLen + t) * hiddenSize
				for d := 0; d < hiddenSize; d++ {
					vec[d] += outputData[offset+d]
				}
			}

			if tokenCount > 0 {
				for d := range vec {
					vec[d] /= tokenCount
				}
			}

			vectors[b] = l2Normalize(vec)
		}
	}

	return vectors, nil
}

// l2Normalize normalizes a vector to unit length.
func l2Normalize(v Vector) Vector {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	out := make(Vector, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

// probeDimensions runs a single inference to determine the output dimensions.
func probeDimensions(session *ort.DynamicAdvancedSession, tokenizer *Tokenizer, numInputs int) (int, error) {
	encoded := tokenizer.EncodeBatch([]string{"test"})
	shape := ort.NewShape(1, int64(encoded.MaxLen))

	inputIDs, _ := ort.NewTensor(shape, encoded.InputIDs)
	defer inputIDs.Destroy()
	attMask, _ := ort.NewTensor(shape, encoded.AttentionMask)
	defer attMask.Destroy()

	inputs := []ort.Value{inputIDs, attMask}
	if numInputs >= 3 {
		tokenTypeIDs, _ := ort.NewTensor(shape, encoded.TokenTypeIDs)
		defer tokenTypeIDs.Destroy()
		inputs = append(inputs, tokenTypeIDs)
	}

	outputs := []ort.Value{nil}
	err := session.Run(inputs, outputs)
	if err != nil {
		return 0, err
	}
	defer outputs[0].Destroy()

	outShape := outputs[0].GetShape()
	if len(outShape) < 2 {
		return 0, fmt.Errorf("unexpected output shape: %v (need at least 2 dims)", outShape)
	}

	// Output can be [batch, hidden] or [batch, seq_len, hidden].
	return int(outShape[len(outShape)-1]), nil
}

// findModelFile locates the ONNX model file in the given directory.
func findModelFile(dir string) (string, error) {
	candidates := []string{"model_quantized.onnx", "model.onnx", "onnx/model_quantized.onnx", "onnx/model.onnx"}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no ONNX model found in %s", dir)
}

// findONNXRuntimeLib returns the path to the ONNX Runtime shared library.
func findONNXRuntimeLib() string {
	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/usr/local/lib/libonnxruntime.dylib",
			"/opt/homebrew/lib/libonnxruntime.dylib",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		return "libonnxruntime.dylib"
	default:
		candidates := []string{
			"/usr/lib/libonnxruntime.so",
			"/usr/local/lib/libonnxruntime.so",
			"/usr/lib/x86_64-linux-gnu/libonnxruntime.so",
			"/usr/lib64/libonnxruntime.so",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		return "libonnxruntime.so"
	}
}
