package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// TestCollectDims_PreservesOrder 锁定并发收集内核的保序契约：
// 输出必须按维度声明顺序，与 goroutine 完成顺序无关。
//
// 夹具让首个维度睡得最久，并记录实际完成顺序作为自检——若完成顺序与声明
// 顺序相同，本测试就无法区分「按声明序落槽」与「按完成序落槽」，会恒绿。
func TestCollectDims_PreservesOrder(t *testing.T) {
	dims := []types.Dimension{
		{ID: 11, Name: "慢"},
		{ID: 21, Name: "中"},
		{ID: 22, Name: "快"},
	}

	var mu sync.Mutex
	var completionOrder []int64
	record := func(id int64) {
		mu.Lock()
		completionOrder = append(completionOrder, id)
		mu.Unlock()
	}

	batches, errs, egErr := collectDims(context.Background(), dims, 3, collectDimsOpts{},
		func(_ context.Context, d types.Dimension) (int64, error) {
			switch d.ID {
			case 11:
				time.Sleep(80 * time.Millisecond)
			case 21:
				time.Sleep(40 * time.Millisecond)
			}
			record(d.ID)
			return d.ID, nil
		})
	if egErr != nil {
		t.Fatalf("collectDims 返回 errgroup 错误: %v", egErr)
	}
	if len(errs) != 0 {
		t.Fatalf("期望无错误，得到 %v", errs)
	}

	want := []int64{11, 21, 22}
	if len(batches) != len(want) {
		t.Fatalf("期望 %d 个分片，得到 %d", len(want), len(batches))
	}

	// 夹具自检：首维度必须最后完成，否则本测试无法证伪乱序实现。
	if len(completionOrder) != len(want) {
		t.Fatalf("完成顺序记录不完整: %v", completionOrder)
	}
	if completionOrder[len(completionOrder)-1] != want[0] {
		t.Fatalf("夹具前提不成立：首维度并非最后完成（完成顺序=%v），本测试无法证伪乱序实现", completionOrder)
	}

	for i, w := range want {
		if batches[i] != w {
			t.Fatalf("保序失败：位置 %d 期望维度 %d，得到 %d（完整分片=%v）", i, w, batches[i], batches)
		}
	}
}

// TestCollectDims_SkipsZeroID 锁定跳过 id=0 汇总维度的语义。
func TestCollectDims_SkipsZeroID(t *testing.T) {
	dims := []types.Dimension{{ID: 0, Name: "汇总"}, {ID: 7, Name: "真实"}}
	batches, _, egErr := collectDims(context.Background(), dims, 2, collectDimsOpts{},
		func(_ context.Context, d types.Dimension) (int64, error) { return d.ID, nil })
	if egErr != nil {
		t.Fatalf("collectDims 返回错误: %v", egErr)
	}
	if len(batches) != 1 {
		t.Fatalf("id=0 的汇总维度应被跳过，期望 1 个分片，得到 %d", len(batches))
	}
	if batches[0] != 7 {
		t.Errorf("期望保留真实维度 7，得到 %d", batches[0])
	}
}

// TestCollectDims_BizErrorDoesNotInterruptSiblings 锁定「单个维度业务失败
// 不中断其他维度」——这是结构化路径的既有契约。
func TestCollectDims_BizErrorDoesNotInterruptSiblings(t *testing.T) {
	dims := []types.Dimension{{ID: 1}, {ID: 2}, {ID: 3}}
	batches, errs, egErr := collectDims(context.Background(), dims, 3, collectDimsOpts{},
		func(_ context.Context, d types.Dimension) (int64, error) {
			if d.ID == 2 {
				return 0, errors.New("维度 2 业务失败")
			}
			return d.ID, nil
		})
	if egErr != nil {
		t.Fatalf("业务错误不应中断其他维度，errgroup 不应返回错误，得到 %v", egErr)
	}
	if len(errs) != 1 {
		t.Fatalf("期望记录 1 个错误，得到 %d: %v", len(errs), errs)
	}
	if len(batches) != 3 {
		t.Fatalf("期望保留 3 个槽位，得到 %d", len(batches))
	}
	if batches[0] != 1 || batches[2] != 3 {
		t.Errorf("成功维度的分片应保留原值，得到 %v", batches)
	}
}

// TestCollectDims_PropagateDimCancelOptIn 锁定两条路径的取消语义差异
// 只能由显式开关控制。
//
// 该差异若在泛化时被抹平，结构化路径的混合失败场景会从 ErrBusinessRejected
// 退化为 ErrRetryable——TestFetchTasks_MixedBizAndCancel_FailedCountAccurate
// 守住该行为，本测试守住开关本身。
func TestCollectDims_PropagateDimCancelOptIn(t *testing.T) {
	t.Run("默认不传播", func(t *testing.T) {
		dims := []types.Dimension{{ID: 1}, {ID: 2}}
		_, errs, egErr := collectDims(context.Background(), dims, 2, collectDimsOpts{},
			func(_ context.Context, _ types.Dimension) (int64, error) {
				return 0, context.DeadlineExceeded
			})
		if egErr != nil {
			t.Errorf("默认策略下维度级 context 错误不应向 errgroup 传播，得到 %v", egErr)
		}
		if len(errs) != 2 {
			t.Errorf("默认策略下 context 错误仍应逐个记入错误列表，期望 2 个，得到 %d", len(errs))
		}
	})

	t.Run("开启后传播并记录", func(t *testing.T) {
		dims := []types.Dimension{{ID: 1}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 预取消
		_, errs, egErr := collectDims(ctx, dims, 1, collectDimsOpts{
			propagateDimCancel: true,
			recordCancelled:    true,
		}, func(gctx context.Context, _ types.Dimension) (int64, error) {
			return 0, gctx.Err()
		})
		if !errors.Is(egErr, context.Canceled) {
			t.Errorf("开启传播后期望 egErr 为 context.Canceled，得到 %v", egErr)
		}
		if len(errs) == 0 {
			t.Error("开启 recordCancelled 后取消错误应记入错误列表")
		}
	})

	t.Run("开启传播但不记录维度级", func(t *testing.T) {
		dims := []types.Dimension{{ID: 1}}
		_, errs, egErr := collectDims(context.Background(), dims, 1, collectDimsOpts{
			propagateDimCancel: true,
		}, func(_ context.Context, _ types.Dimension) (int64, error) {
			return 0, context.DeadlineExceeded
		})
		if !errors.Is(egErr, context.DeadlineExceeded) {
			t.Errorf("开启传播后期望 egErr 为 context.DeadlineExceeded，得到 %v", egErr)
		}
		if len(errs) != 0 {
			t.Errorf("未开启 recordCancelled 时不应记入错误列表，得到 %v", errs)
		}
	})
}

// TestCollectDims_ClampsDimensionCount 锁定维度数上界钳制：服务端声明的
// 维度数不得驱动无界并发与累积。
func TestCollectDims_ClampsDimensionCount(t *testing.T) {
	dims := make([]types.Dimension, 0, 300)
	for i := 1; i <= 300; i++ {
		dims = append(dims, types.Dimension{ID: int64(i)})
	}
	batches, _, egErr := collectDims(context.Background(), dims, 8, collectDimsOpts{maxDims: 128},
		func(_ context.Context, d types.Dimension) (int64, error) { return d.ID, nil })
	if egErr != nil {
		t.Fatalf("collectDims 返回错误: %v", egErr)
	}
	if len(batches) != 128 {
		t.Fatalf("维度数应被钳制到 128，得到 %d", len(batches))
	}
	if batches[127] != 128 {
		t.Errorf("应保留前 128 维，末位期望维度 128，得到 %d", batches[127])
	}
}

// TestCollectDims_NoClampWhenUnset 确认未设上界时不钳制。
func TestCollectDims_NoClampWhenUnset(t *testing.T) {
	dims := make([]types.Dimension, 0, 50)
	for i := 1; i <= 50; i++ {
		dims = append(dims, types.Dimension{ID: int64(i)})
	}
	batches, _, egErr := collectDims(context.Background(), dims, 8, collectDimsOpts{},
		func(_ context.Context, d types.Dimension) (int64, error) { return d.ID, nil })
	if egErr != nil {
		t.Fatalf("collectDims 返回错误: %v", egErr)
	}
	if len(batches) != 50 {
		t.Fatalf("未设上界时不应钳制，期望 50 个分片，得到 %d", len(batches))
	}
}

// TestClassifyDimErrors_SplitsByCancel 锁定错误分类口径：context 取消与
// 业务错误两分，且取消计数正确。
func TestClassifyDimErrors_SplitsByCancel(t *testing.T) {
	bizErr := errors.New("业务失败")
	cancelErr := context.DeadlineExceeded
	bizErrs, ctxErrs, cancelled := classifyDimErrors([]error{bizErr, cancelErr, bizErr})
	if len(bizErrs) != 2 {
		t.Errorf("期望 2 个业务错误，得到 %d: %v", len(bizErrs), bizErrs)
	}
	if len(ctxErrs) != 1 {
		t.Errorf("期望 1 个取消错误，得到 %d: %v", len(ctxErrs), ctxErrs)
	}
	if cancelled != 1 {
		t.Errorf("取消计数期望 1，得到 %d", cancelled)
	}
}

// TestClassifyDimErrors_EmptyInput 确认空输入得到全零结果。
func TestClassifyDimErrors_EmptyInput(t *testing.T) {
	bizErrs, ctxErrs, cancelled := classifyDimErrors(nil)
	if len(bizErrs) != 0 || len(ctxErrs) != 0 || cancelled != 0 {
		t.Errorf("空输入应得到全零结果，得到 biz=%v ctx=%v cancelled=%d", bizErrs, ctxErrs, cancelled)
	}
}
