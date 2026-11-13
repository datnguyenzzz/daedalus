.PHONY: bench-sine bench-cosine bench-const bench-randomised

bench-sine:
	cd bench/sine_wave && rm -f input.dat
#	cd bench/sine_wave && rm -f static_rl_output.dat
#	cd bench && go test -bench=BenchmarkStaticRLSineWave \
#		-cpuprofile=./sine_wave/cpu_static_rl.prof \
#		-memprofile=./sine_wave/mem_static_rl.prof \
#		-trace=./sine_wave/trace_static_rl.out \
#		-benchtime=1x -timeout=30m
	cd bench/sine_wave && rm -f adaptive_rl_output.dat
	cd bench && go test -bench=BenchmarkAdaptiveRLSineWave -benchtime=1x -timeout=30m

	cd bench/sine_wave && gnuplot -persist plot.gp

bench-cosine:
	cd bench/cosine_wave && rm -f static_rl_output.dat && rm -f adaptive_rl_output.dat && rm -f input.dat
	cd bench && go test -bench=BenchmarkStaticRLCosineWave \
		-cpuprofile=./cosine_wave/cpu_static_rl.prof \
		-memprofile=./cosine_wave/mem_static_rl.prof \
		-trace=./cosine_wave/trace_static_rl.out \
		-benchtime=1x -timeout=30m
	cd bench && go test -bench=BenchmarkAdaptiveRLCosineWave \
		-cpuprofile=./cosine_wave/cpu_adaptive_rl.prof \
		-memprofile=./cosine_wave/mem_adaptive.prof \
		-trace=./cosine_wave/trace_adaptive_rl.out \
		-benchtime=1x -timeout=30m

	cd bench/cosine_wave && gnuplot -persist plot.gp

bench-square:
	cd bench/square_wave && rm -f static_rl_output.dat && rm -f adaptive_rl_output.dat && rm -f input.dat
	cd bench && go test -bench=BenchmarkStaticRLSquareWave \
		-cpuprofile=./square_wave/cpu_static_rl.prof \
		-memprofile=./square_wave/mem_static_rl.prof \
		-trace=./square_wave/trace_static_rl.out \
		-benchtime=1x -timeout=30m
	cd bench && go test -bench=BenchmarkAdaptiveRLSquareWave \
		-cpuprofile=./square_wave/cpu_adaptive_rl.prof \
		-memprofile=./square_wave/mem_adaptive.prof \
		-trace=./square_wave/trace_adaptive_rl.out \
		-benchtime=1x -timeout=30m

	cd bench/square_wave && gnuplot -persist plot.gp

bench-rectangular:
	cd bench/rectangular_wave && rm -f static_rl_output.dat && rm -f adaptive_rl_output.dat && rm -f input.dat
	cd bench && go test -bench=BenchmarkStaticRLRectangularWave \
		-cpuprofile=./rectangular_wave/cpu_static_rl.prof \
		-memprofile=./rectangular_wave/mem_static_rl.prof \
		-trace=./rectangular_wave/trace_static_rl.out \
		-benchtime=1x -timeout=30m
	cd bench && go test -bench=BenchmarkAdaptiveRLRectangularWave \
		-cpuprofile=./square_wave/cpu_adaptive_rl.prof \
		-memprofile=./square_wave/mem_adaptive.prof \
		-trace=./square_wave/trace_adaptive_rl.out \
		-benchtime=1x -timeout=30m

	cd bench/rectangular_wave && gnuplot -persist plot.gp

bench-const:
	cd bench && go test -bench=BenchmarkStaticRLConstFunc -benchtime=1x
	cd bench && go test -bench=BenchmarkAdaptiveRLConstFunc -benchtime=1x

bench-randomised:
	cd bench/randomised && rm -f static_rl_output.dat && rm -f adaptive_rl_output.dat
	cd bench && go test -bench=BenchmarkRandomisedFunc -benchtime=1x \
		-cpuprofile=./randomised/cpu.prof \
		-memprofile=./randomised/mem.prof \
		-trace=./randomised/trace.out \
		-benchtime=1x -timeout=300m

benchmark:
	$(MAKE) bench-sine
	$(MAKE) bench-cosine
