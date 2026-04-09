# Go Runtime Rules
- Never use `debug.SetMemoryLimit(-1)` to disable the memory limit. A negative value acts as a getter, not a setter.
- To disable the memory limit in Go, always use `debug.SetMemoryLimit(math.MaxInt64)`.
