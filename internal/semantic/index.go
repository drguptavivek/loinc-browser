package semantic

import (
	"runtime"
	"sort"
	"sync"
)

// Index holds every term's unit vector quantized to int8 (x*127), so 109k terms x 1024 dims take
// ~110 MB instead of ~450 MB as float32; the rounding barely moves cosine rankings.
type Index struct {
	ids  []string
	dims int
	data []int8 // len(ids)*dims, row-major
}

// Hit is one search result: a LOINC number and its cosine similarity to the query (-1..1).
type Hit struct {
	LOINCNum string
	Score    float32
}

func quantize(v []float32) []int8 {
	out := make([]int8, len(v))
	for i, x := range v {
		q := x * 127
		switch {
		case q > 127:
			q = 127
		case q < -127:
			q = -127
		}
		if q >= 0 {
			out[i] = int8(q + 0.5)
		} else {
			out[i] = int8(q - 0.5)
		}
	}
	return out
}

// TopK scores every vector against query and returns the k best, highest first. The scan is split
// across CPU cores; at 109k x 1024 it takes a few tens of milliseconds.
func (idx *Index) TopK(query []float32, k int) []Hit {
	n := len(idx.ids)
	if n == 0 || len(query) != idx.dims || k <= 0 {
		return nil
	}
	q := quantize(query)
	scores := make([]int32, n)
	workers := runtime.GOMAXPROCS(0)
	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for start := 0; start < n; start += chunk {
		end := min(start+chunk, n)
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for row := start; row < end; row++ {
				vec := idx.data[row*idx.dims : (row+1)*idx.dims]
				var dot int32
				for i, x := range vec {
					dot += int32(x) * int32(q[i])
				}
				scores[row] = dot
			}
		}(start, end)
	}
	wg.Wait()

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	// ponytail: full sort of 109k ints (~10 ms); switch to a k-sized heap if it shows in profiles.
	sort.Slice(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	if k > n {
		k = n
	}
	hits := make([]Hit, k)
	for i := 0; i < k; i++ {
		row := order[i]
		hits[i] = Hit{LOINCNum: idx.ids[row], Score: float32(scores[row]) / (127 * 127)}
	}
	return hits
}
