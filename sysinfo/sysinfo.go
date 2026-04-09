package sysinfo

import (
	"context"
	"log/slog"
	"math"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/jaypipes/ghw"
	"github.com/mongodb-labs/migration-tools/humantools"
	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/mem"
)

// LogSystemInfo logs system specs useful for gauging vertical scale.
func LogSystemInfo(ctx context.Context, logger *slog.Logger) {
	// NB: Per docs, negative values cause this function not to
	// alter anything.
	memlimitBytes := debug.SetMemoryLimit(-1)

	memlimitStr := lo.Ternary(
		memlimitBytes == math.MaxInt64,
		"none",
		humantools.FmtBytes(memlimitBytes),
	)

	attrs := []slog.Attr{
		slog.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		slog.String("gomemlimit", memlimitStr),
	}

	attrs = append(attrs, slog.Group(
		"cpu",
		lo.ToAnySlice(getCPUAttrs(ctx))...,
	))

	attrs = append(attrs, slog.Group(
		"memory",
		lo.ToAnySlice(getMemoryAttrs(ctx))...,
	))

	logger.InfoContext(ctx, "System info", lo.ToAnySlice(attrs)...)
}

func getCPUAttrs(ctx context.Context) []slog.Attr {
	attrs := []slog.Attr{
		slog.Int("totalLogicalCPUs", runtime.NumCPU()),
	}

	cpu, err := ghw.CPU(ctx)
	if err != nil {
		attrs = append(
			attrs,
			slog.Any("cpuInfoErr", err),
		)

		return attrs
	}

	attrs = append(
		attrs,
		slog.Uint64("totalCores", uint64(cpu.TotalCores)),
	)

	// TotalHardwareThreads is the same data point as
	// runtime.NumCPU() above, so we skip it.

	// Log all processor details
	for i, proc := range cpu.Processors {
		groupAttrs := []slog.Attr{
			slog.Int64("id", int64(proc.ID)),
		}

		if proc.Vendor != "" {
			groupAttrs = append(groupAttrs, slog.String("vendor", proc.Vendor))
		}
		if proc.Model != "" {
			groupAttrs = append(groupAttrs, slog.String("model", proc.Model))
		}
		if proc.NumCores > 0 {
			groupAttrs = append(groupAttrs, slog.Int64("cores", int64(proc.NumCores)))
		}
		if proc.NumThreads > 0 {
			groupAttrs = append(groupAttrs, slog.Int64("threads", int64(proc.NumThreads)))
		}

		// Skip Capabilities since it’s long & esoteric.

		attrs = append(
			attrs,
			slog.Group(
				strconv.Itoa(i),
				lo.ToAnySlice(groupAttrs)...,
			),
		)
	}

	return attrs
}

func getMemoryAttrs(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr

	ghwMem, err := ghw.Memory(ctx)
	if err != nil {
		// Memory() doesn’t work on macOS, so don’t bother
		// logging the error here; instead just try another library.
		return getSimpleMemoryAttrs(ctx)
	}

	if ghwMem.TotalPhysicalBytes > 0 {
		attrs = append(
			attrs,
			slog.String("physical", humantools.FmtBytes(ghwMem.TotalPhysicalBytes)),
		)
	}
	if ghwMem.TotalUsableBytes > 0 {
		attrs = append(attrs, slog.String("usable", humantools.FmtBytes(ghwMem.TotalUsableBytes)))
	}
	if len(ghwMem.SupportedPageSizes) > 0 {
		pageSizes := make([]string, len(ghwMem.SupportedPageSizes))
		for i, size := range ghwMem.SupportedPageSizes {
			pageSizes[i] = humantools.FmtBytes(size)
		}
		attrs = append(attrs, slog.Any("pageSizes", pageSizes))
	}

	if len(attrs) == 0 {
		return getSimpleMemoryAttrs(ctx)
	}
	return attrs
}

func getSimpleMemoryAttrs(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr

	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		attrs = append(attrs, slog.Any("gopsutilErr", err))
	} else {
		if vmem.Total > 0 {
			attrs = append(attrs, slog.String("total", humantools.FmtBytes(vmem.Total)))
		}
		if vmem.Available > 0 {
			attrs = append(attrs, slog.String("available", humantools.FmtBytes(vmem.Available)))
		}
	}

	return attrs
}
