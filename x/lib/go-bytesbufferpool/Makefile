benchmark:
	@go test -bench=. -count=1 \
	 -benchmem -memprofile results/mem.out \
	 -cpuprofile results/cpu.out \
	 -trace results/trace.out \
	 | tee results/bench-results.txt

pprof-cpu:
	@go tool pprof -http :8080 results/cpu.out

pprof-mem:
	@go tool pprof -http :8081 results/mem.out

pprof-trace:
	@go tool trace trace.out