package skilldelta

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// BuildBenchmark constructs a benchmark document from metadata and pair results.
func BuildBenchmark(metadata BenchmarkMetadata, pairs []PairResult) Benchmark {
	sorted := append([]PairResult(nil), pairs...)
	sortPairResults(sorted)
	return Benchmark{
		Metadata: metadata,
		Pairs:    sorted,
		Summary:  Aggregate(sorted),
	}
}

// LoadPairs walks a benchmark directory and loads all pair.json files.
func LoadPairs(root string) ([]PairResult, error) {
	var pairs []PairResult
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "pair.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var pair PairResult
		if err := json.Unmarshal(data, &pair); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		pairs = append(pairs, pair)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sortPairResults(pairs)
	return pairs, nil
}

// WritePairJSON writes one pair result to disk.
func WritePairJSON(path string, pair PairResult) error {
	return writeJSON(path, pair)
}

// WriteBenchmarkJSON writes benchmark.json to disk.
func WriteBenchmarkJSON(path string, benchmark Benchmark) error {
	return writeJSON(path, benchmark)
}

// ReadBenchmarkJSON reads a benchmark document from disk.
func ReadBenchmarkJSON(path string) (Benchmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Benchmark{}, err
	}
	var benchmark Benchmark
	if err := json.Unmarshal(data, &benchmark); err != nil {
		return Benchmark{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return benchmark, nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func sortPairResults(pairs []PairResult) {
	sort.Slice(pairs, func(i, j int) bool {
		a, b := pairs[i], pairs[j]
		if a.ScenarioID != b.ScenarioID {
			return a.ScenarioID < b.ScenarioID
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		return a.Repeat < b.Repeat
	})
}
