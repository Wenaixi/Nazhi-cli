package client

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// TestParallelDims_ClampsDimensionCount 锁定导出层 ParallelDims 的维度数上界。
//
// 背景：collectDims 内核的钳制由 collectDimsOpts.maxDims 驱动，零值表示
// 不钳制（TestCollectDims_NoClampWhenUnset 锁定该语义为显式契约）。但
// ParallelDims 作为面向调用方的导出入口此前传零值，导致直接调用它的调用方
// 可以用任意维度数驱动无界并发与累积——维度闸的防护被移到 seam 之外，
// 只靠「内部调用方恰好自己预钳了一次」维持。
//
// 本测试锁定：ParallelDims 自己持有维度上界，与内部两条取数路径同源。
func TestParallelDims_ClampsDimensionCount(t *testing.T) {
	const total = 300
	dims := make([]types.Dimension, 0, total)
	for i := 1; i <= total; i++ {
		dims = append(dims, types.Dimension{ID: int64(i)})
	}

	var fanout atomic.Int64
	result, egErr := ParallelDims[types.Task](
		context.Background(), dims, 8,
		func(_ context.Context, d types.Dimension) ([]types.Task, error) {
			fanout.Add(1)
			return []types.Task{{ID: d.ID}}, nil
		},
	)
	if egErr != nil {
		t.Fatalf("ParallelDims 返回错误: %v", egErr)
	}

	if got := len(result.Items); got != maxFetchTasksDims {
		t.Errorf("维度数应被钳制到 %d，实际聚合 %d 个 item", maxFetchTasksDims, got)
	}
	if got := fanout.Load(); got > maxFetchTasksDims {
		t.Errorf("扇出次数应 ≤ %d，实际 %d 次", maxFetchTasksDims, got)
	}
}
