# Concurrency And Performance

Это один из самых важных разделов для senior Go.

Темы:
- goroutine lifecycle;
- channels, buffering, cancellation;
- worker pools и bounded concurrency;
- mutex, atomic, condition variables;
- race detector и memory visibility;
- allocation hotspots;
- CPU-bound vs IO-bound workloads;
- profiling через `pprof`;
- benchmark methodology;
- latency spikes, GC pauses, queue buildup.

Практические вопросы:
- когда channel хуже mutex;
- как ограничить fan-out;
- как найти goroutine leak;
- как измерять производительность до и после оптимизации;
- почему throughput вырос, а p99 стал хуже.
