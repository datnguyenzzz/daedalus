package bench

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type worker struct {
	fileName string
	mu       sync.Mutex
	ops      int32
}

func newWorker(fileName string) *worker {
	w := &worker{
		fileName: fileName,
		ops:      0,
	}
	return w
}

func (w *worker) scrape(t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ops == 0 {
		return
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(w.fileName)
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	file, err := os.OpenFile(w.fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("Error opening output file: %v\n", err)
		return
	}
	defer file.Close()

	fmt.Fprintf(file, "%d %d\n", t.Unix(), w.ops)

	w.ops = 0
}

func (w *worker) DoWork() {
	time.Sleep(1 * time.Millisecond)
	atomic.AddInt32(&w.ops, 1)
}

func (w *worker) Scrape(quit <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			// Perform final scrape before exiting
			w.scrape(time.Now())
			return
		case t := <-ticker.C:
			w.scrape(t)
		}
	}
}

func writeInputToFile(input [][]int64, dir string) error {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(dir, "input.dat"))
	if err != nil {
		return err
	}
	defer file.Close()

	for i := range len(input[0]) {
		fmt.Fprintf(file, "%d %d\n", input[0][i], input[1][i])
	}

	return nil
}

// sineWaveGenerate F(x) = A*sin(Bx + C) + D
func sineWaveGenerate(entries int, a, b, c, d float64) [][]float64 {
	f := func(x float64) float64 {
		return a*math.Sin(b*x+c) + d
	}

	res := make([][]float64, 2)
	for i := range 2 {
		res[i] = make([]float64, entries)
	}
	for i := range entries {
		res[0][i] = float64(i)
		res[1][i] = f(float64(i))
	}

	return res
}

// cosineWaveGenerate F(x) = A*cos(Bx + C) + D
func cosineWaveGenerate(entries int, a, b, c, d float64) [][]float64 {
	f := func(x float64) float64 {
		return a*math.Cos(b*x+c) + d
	}

	res := make([][]float64, 2)
	for i := range 2 {
		res[i] = make([]float64, entries)
	}
	for i := range entries {
		res[0][i] = float64(i)
		res[1][i] = f(float64(i))
	}

	return res
}

// constantFunctionGenerate F(x) = c
func constantFunctionGenerate(entries int, c float64) [][]float64 {
	res := make([][]float64, 2)
	for i := range 2 {
		res[i] = make([]float64, entries)
	}
	for i := range entries {
		res[0][i] = float64(i)
		res[1][i] = c
	}

	return res
}

func randomFunctionGenerate(entries int, lo, hi int) [][]float64 {
	res := make([][]float64, 2)
	for i := range 2 {
		res[i] = make([]float64, entries)
	}
	counter := 0
	for i := range entries {
		res[0][i] = float64(i)
		src := rand.NewSource(time.Now().UnixNano())
		r := rand.New(src)
		if counter <= 5 {
			// randomise from [lo, hi]
			res[1][i] = float64(lo) + float64(r.Intn(hi-lo))
			if res[1][i] >= float64((lo+hi)>>1) {
				counter = 0
			} else {
				counter += 1
			}
		} else {
			// randomise from (trisection, hi]
			trisection := (hi-lo+1)*2/3 + lo
			res[1][i] = float64(trisection) + float64(r.Intn(hi-trisection))
			counter = 0
		}
	}

	return res
}

// squareWaveGenerate F(x) = c*sgn(sin(b*x)) + d
func squareWaveGenerate(entries int, b, c, d float64) [][]float64 {
	f := func(x float64) float64 {
		v := math.Sin(b * x)
		if v >= 0 {
			return c + d
		}

		return -c + d
	}

	res := make([][]float64, 2)
	for i := range 2 {
		res[i] = make([]float64, entries)
	}
	for i := range entries {
		res[0][i] = float64(i)
		res[1][i] = f(float64(i))
	}

	return res
}

func rectangularWaveGenerate(entries int, frequency, amplitude, dutyCycle float64) [][]float64 {
	f := func(t float64) float64 {
		period := 1.0 / frequency
		timeInPeriod := math.Mod(t, period) // Time elapsed within the current period

		// Determine if the wave is currently "high" or "low" based on the duty cycle
		if timeInPeriod < period*dutyCycle {
			return amplitude
		}
		return 0 // Or 0, depending on whether you want a bipolar or unipolar wave
	}

	res := make([][]float64, 2)
	for i := range 2 {
		res[i] = make([]float64, entries)
	}
	for i := range entries {
		res[0][i] = float64(i)
		res[1][i] = f(float64(i) * 0.1)
	}

	return res
}
