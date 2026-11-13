package bench

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	go_adaptive_rate_limiter "github.com/datnguyenzzz/nogodb/lib/go-adaptive-rate-limiter"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

var (
	sineWaveDir        = "sine_wave"
	cosineWaveDir      = "cosine_wave"
	squareWaveDir      = "square_wave"
	constFuncDir       = "const"
	randomisedFuncDir  = "randomised"
	rectangularFuncDir = "rectangular_wave"
)

const (
	benchEntries = 400
	concurrency  = 100

	randomisedTestMin   = 700
	randomisedTestCases = 3

	// randomisedTestEntries = 5
	// randomisedTestCases   = 3
)

var sharedInputForRandomisedTest [][][]int64

func getOrSetSharedInputforRandomisedTest() [][][]int64 {
	if len(sharedInputForRandomisedTest) > 0 {
		return sharedInputForRandomisedTest
	}

	sharedInputForRandomisedTest = make([][][]int64, randomisedTestCases)
	for i := range sharedInputForRandomisedTest {
		inputCount := randomisedTestMin + 100*i
		sharedInputForRandomisedTest[i] = generateRandomisedFunction(inputCount)
	}

	return sharedInputForRandomisedTest
}

const (
	minLimit = 500
	maxLimit = 5000
	burst    = 1
)

// Static Rate limiter -- Wave function

func BenchmarkStaticRLSineWave(b *testing.B) {
	fmt.Println("Start bench static rate limiter - Sine Wave ...")
	input := generateSineWave()

	rl := rate.NewLimiter(maxLimit, burst)
	sharedWorker := newWorker(filepath.Join(sineWaveDir, "static_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		time.Sleep(1 * time.Second)
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()

	// Cleanse up
	quit <- struct{}{}
}

func BenchmarkStaticRLCosineWave(b *testing.B) {
	fmt.Println("Start bench static rate limiter - Cosine Wave ...")
	input := generateCosineWave()

	rl := rate.NewLimiter(maxLimit, burst)
	sharedWorker := newWorker(filepath.Join(cosineWaveDir, "static_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		time.Sleep(1 * time.Second)
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()

	// Cleanse up
	quit <- struct{}{}
}

func BenchmarkStaticRLConstFunc(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Const Function ...")
	_ = generateConstFunction()
}

func BenchmarkStaticRLSquareWave(b *testing.B) {
	fmt.Println("Start bench static rate limiter - Square Wave ...")
	input := generateSquareWave()

	rl := rate.NewLimiter(maxLimit, burst)
	sharedWorker := newWorker(filepath.Join(squareWaveDir, "static_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		time.Sleep(1 * time.Second)
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()

	// Cleanse up
	quit <- struct{}{}
}

func BenchmarkStaticRLRectangularWave(b *testing.B) {
	fmt.Println("Start bench static rate limiter - Rectangular Wave ...")
	input := generateRectangularFunction(benchEntries)

	rl := rate.NewLimiter(maxLimit, burst)
	sharedWorker := newWorker(filepath.Join(rectangularFuncDir, "static_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()

	// Cleanse up
	quit <- struct{}{}
}

// Adaptive Rate limiter -- Wave function

func BenchmarkAdaptiveRLSineWave(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Sine Wave ...")
	input := generateSineWave()

	rl := go_adaptive_rate_limiter.NewAdaptiveRateLimiter(
		go_adaptive_rate_limiter.WithLimit(minLimit, maxLimit),
	)
	sharedWorker := newWorker(filepath.Join(sineWaveDir, "adaptive_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		time.Sleep(1 * time.Second)
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()
	// Cleanse up
	quit <- struct{}{}
}

func BenchmarkAdaptiveRLCosineWave(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Cosine Wave ...")
	input := generateCosineWave()

	rl := go_adaptive_rate_limiter.NewAdaptiveRateLimiter(
		go_adaptive_rate_limiter.WithLimit(minLimit, maxLimit),
	)
	sharedWorker := newWorker(filepath.Join(cosineWaveDir, "adaptive_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		time.Sleep(1 * time.Second)
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()
	// Cleanse up
	quit <- struct{}{}
}

func BenchmarkAdaptiveRLConstFunc(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Const Function ...")
	_ = generateConstFunction()
}

func BenchmarkAdaptiveRLRectangularWave(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Rectangular Wave ...")
	input := generateRectangularFunction(benchEntries)

	rl := go_adaptive_rate_limiter.NewAdaptiveRateLimiter(
		go_adaptive_rate_limiter.WithLimit(minLimit, maxLimit),
	)
	sharedWorker := newWorker(filepath.Join(rectangularFuncDir, "adaptive_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()

	// Cleanse up
	quit <- struct{}{}
}

func BenchmarkAdaptiveRLSquareWave(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Square Wave ...")
	input := generateSquareWave()

	rl := go_adaptive_rate_limiter.NewAdaptiveRateLimiter(
		go_adaptive_rate_limiter.WithLimit(minLimit, maxLimit),
	)
	sharedWorker := newWorker(filepath.Join(squareWaveDir, "adaptive_rl_output.dat"))
	quit := make(chan struct{})

	// Start scraping metrics
	go func() {
		sharedWorker.Scrape(quit)
	}()

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		time.Sleep(1 * time.Second)
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				sharedWorker.DoWork()
				return nil
			})
		}
	}

	_ = eg.Wait()
	// Cleanse up
	quit <- struct{}{}
}

// Randomised bench test

type iLimiter interface {
	Wait(ctx context.Context) error
}

func getAdaptiveRL() iLimiter {
	return go_adaptive_rate_limiter.NewAdaptiveRateLimiter(
		go_adaptive_rate_limiter.WithLimit(minLimit, maxLimit),
	)
}

func getStaticRL() iLimiter {
	return rate.NewLimiter(maxLimit, burst)
}

func randomisedBench(input [][]int64, isAdaptive bool) int64 {
	startTime := time.Now().Unix()
	var rl iLimiter

	if !isAdaptive {
		rl = getStaticRL()
	} else {
		rl = getAdaptiveRL()
	}

	// Do work
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(concurrency)
	for _, opCount := range input[1] {
		for i := 0; int64(i) < opCount; i++ {
			eg.Go(func() error {
				if err := rl.Wait(ctx); err != nil {
					panic(err)
				}
				time.Sleep(1 * time.Millisecond)
				return nil
			})
		}
	}
	_ = eg.Wait()

	completedTime := time.Now().Unix() - startTime
	return completedTime
}

func BenchmarkRandomisedFunc(b *testing.B) {
	fmt.Println("Start bench adaptive rate limiter - Randomised Function ...")
	inputPerTests := getOrSetSharedInputforRandomisedTest()
	// fmt.Printf("%+v", inputPerTests)

	adaptiveResult := make([]int64, 0, len(inputPerTests))
	staticResult := make([]int64, 0, len(inputPerTests))

	for tc, input := range inputPerTests {
		b.Run(fmt.Sprintf("bench-%v", tc), func(b *testing.B) {
			b.ResetTimer()
			res := randomisedBench(input, false)
			staticResult = append(staticResult, res)
			fmt.Printf("[INFO] staticRLBench-%v result: %v\n", tc, res)

			b.ResetTimer()
			res = randomisedBench(input, true)
			adaptiveResult = append(adaptiveResult, res)
			fmt.Printf("[INFO] adaptiveRLBench-%v result: %v\n", tc, res)
		})
	}

	// report result
	fileName := filepath.Join(randomisedFuncDir, "adaptive_rl_output.dat")
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("Error opening output file: %v\n", err)
		return
	}
	defer file.Close()

	for i, t := range adaptiveResult {
		s := int64(0)
		for _, sz := range inputPerTests[i][1] {
			s += sz
		}
		fmt.Fprintf(file, "%v %v\n", s, t)
	}

	fileName = filepath.Join(randomisedFuncDir, "static_rl_output.dat")
	file, err = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("Error opening output file: %v\n", err)
		return
	}
	defer file.Close()

	for i, t := range staticResult {
		s := int64(0)
		for _, sz := range inputPerTests[i][1] {
			s += sz
		}
		fmt.Fprintf(file, "%v %v\n", s, t)
	}
}

// hard code for now
func generateSineWave() [][]int64 {
	input := sineWaveGenerate(benchEntries, 7000, math.Pi/100, 3*math.Pi/2, 7010)
	res := make([][]int64, 2)
	for i := range 2 {
		res[i] = make([]int64, 0, benchEntries)
		for _, v := range input[i] {
			res[i] = append(res[i], int64(v))
		}
	}

	err := writeInputToFile(res, sineWaveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing sine wave input: %v\n", err)
	}
	return res
}

func generateCosineWave() [][]int64 {
	input := cosineWaveGenerate(benchEntries, 7000, math.Pi/100, 2*math.Pi, 7010)
	res := make([][]int64, 2)
	for i := range 2 {
		res[i] = make([]int64, 0, benchEntries)
		for _, v := range input[i] {
			res[i] = append(res[i], int64(v))
		}
	}
	err := writeInputToFile(res, cosineWaveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing cosine wave input: %v\n", err)
	}
	return res
}

func generateSquareWave() [][]int64 {
	input := squareWaveGenerate(benchEntries, math.Pi/100, -7000, 7010)
	res := make([][]int64, 2)
	for i := range 2 {
		res[i] = make([]int64, 0, benchEntries)
		for _, v := range input[i] {
			res[i] = append(res[i], int64(v))
		}
	}
	err := writeInputToFile(res, squareWaveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing square wave input: %v\n", err)
	}
	return res
}

func generateConstFunction() [][]int64 {
	input := constantFunctionGenerate(benchEntries, 300)
	res := make([][]int64, 2)
	for i := range 2 {
		res[i] = make([]int64, 0, benchEntries)
		for _, v := range input[i] {
			res[i] = append(res[i], int64(v))
		}
	}
	err := writeInputToFile(res, constFuncDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing constant function input: %v\n", err)
	}
	return res
}

func generateRandomisedFunction(randomisedTestEntries int) [][]int64 {
	randomisedInput := randomFunctionGenerate(randomisedTestEntries, 100, 14100)
	res := make([][]int64, 2)
	for i := range 2 {
		res[i] = make([]int64, 0, len(randomisedInput[i]))
		for _, v := range randomisedInput[i] {
			res[i] = append(res[i], int64(v))
		}
	}
	//err := writeInputToFile(res, randomisedFuncDir)
	//if err != nil {
	//fmt.Fprintf(os.Stderr, "Error writing randomised function input: %v\n", err)
	//}
	return res
}

func generateRectangularFunction(benchEntries int) [][]int64 {
	input := rectangularWaveGenerate(benchEntries, 0.1, 3000, 0.15)
	res := make([][]int64, 2)
	for i := range 2 {
		res[i] = make([]int64, 0, len(input[i]))
		for _, v := range input[i] {
			res[i] = append(res[i], int64(v))
		}
	}
	err := writeInputToFile(res, rectangularFuncDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing randomised function input: %v\n", err)
	}
	return res
}
