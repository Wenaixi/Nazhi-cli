package client

import (
	"context"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// ParallelDimsResult 是并行维度查询的聚合结果。
type ParallelDimsResult[T any] struct {
	Items          []T     // 所有成功维度的 item（按维度声明顺序拼接）
	BizErrors      []error // 非 context 取消的业务错误
	ContextErrors  []error // context 取消/超时错误
	CancelledCount int     // 因 ctx 取消/超时而失败的维度数
	FailedCount    int     // 因业务错误而失败的维度数
}

// collectDims 是两条取数路径共用的并发收集内核。
//
// 此前结构化路径走 ParallelDims、透传路径在 FetchTasksJSON 里自建了一套
// fan-out 与错误聚合，两者靠注释约定「改动一条必须同步核对另一条」维持
// 一致——该约定已在事实上破裂（透传路径手写了 id==0 跳过、取消检测与两段
// 几乎逐字相同的错误三分类循环）。
//
// 此前判定无法复用的理由是产出形状不同（记录切片 vs 待拼接字节片段）。
// 本内核把产出单位放宽为「任意可放入槽位的值」，该理由不再成立：两条
// 路径都只是产出一个分片，区别仅在分片如何合并，而合并不属于收集职责。
//
// 收集器只负责四件事，调用方只负责「怎么把分片并起来」：
//   - 跳过 id=0 的汇总维度
//   - 按维度声明顺序落槽，输出与 goroutine 调度顺序解耦
//   - 单维度业务错误不中断其他维度
//   - context 取消向 errgroup 传播，且预取消的维度也计入错误列表
//
// collectDimsOpts 是两条取数路径在取消语义上的差异点。
type collectDimsOpts struct {
	// maxDims 是维度数上界，<=0 表示不钳制。
	maxDims int
	// propagateDimCancel 为真时，fn 返回的 context 错误向 errgroup 传播
	//（透传路径需要：让 g.Wait 走 cancel 分支以保住 ErrRetryable 可重试
	// 语义，否则 cancel 被错误列表吞掉后统一包装为 ErrBusinessRejected）。
	// 为假时（结构化路径）fn 的错误一律只记录、不传播——「单个维度失败
	// 不中断其他维度」是该路径的既有契约。
	propagateDimCancel bool
	// recordCancelled 为真时，被传播的取消错误同时记入错误列表，
	// 供调用方的 partial 分支拼出可重试语义。
	recordCancelled bool
}

func collectDims[T any](
	ctx context.Context,
	dims []types.Dimension,
	limit int,
	opts collectDimsOpts,
	fn func(context.Context, types.Dimension) (T, error),
) (batches []T, errs []error, egErr error) {
	lim := limit
	if lim <= 0 {
		lim = 1
	}

	active := make([]types.Dimension, 0, len(dims))
	for _, dim := range dims {
		if dim.ID == 0 {
			continue // 跳过汇总维度
		}
		active = append(active, dim)
	}
	// 维度数上界钳制：恶意服务端可声明极大维度数，驱动全维度并发拉取
	// 而无预算。截断保留前 maxDims 维（真实学校维度集通常 <10）。
	if opts.maxDims > 0 && len(active) > opts.maxDims {
		active = active[:opts.maxDims]
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(lim)

	var mu sync.Mutex
	batches = make([]T, len(active))
	for idx, dim := range active {
		i, d := idx, dim
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				// 预取消的维度也计入 errs：混合失败场景下调用方的
				// egErr 分支仍能拿到已收集的业务错误诊断（不吞错）。
				appendLocked(&mu, &errs, err)
				return err
			}
			v, err := fn(gctx, d)
			if err != nil {
				// 透传路径需要让 context 错误向 errgroup 传播，走 cancel
				// 分支以保住 ErrRetryable 可重试语义；结构化路径则一律只
				// 记录不传播，「单个维度失败不中断其他维度」是其既有契约。
				if opts.propagateDimCancel && isContextError(err) {
					if opts.recordCancelled {
						appendLocked(&mu, &errs, err)
					}
					return err
				}
				appendLocked(&mu, &errs, err)
				return nil // 业务错误记录到 errs，不取消其他维度
			}
			batches[i] = v // 仅写自己的槽位，无需锁
			return nil
		})
	}
	return batches, errs, g.Wait()
}

// classifyDimErrors 把收集到的错误按 context 取消与业务错误两分，
// 两条取数路径共用同一口径。
func classifyDimErrors(errs []error) (bizErrs, ctxErrs []error, cancelledCount int) {
	for _, e := range errs {
		if isContextError(e) {
			cancelledCount++
			ctxErrs = append(ctxErrs, e)
			continue
		}
		bizErrs = append(bizErrs, e)
	}
	return bizErrs, ctxErrs, cancelledCount
}

// ParallelDims 对维度列表并发执行 fn，聚合结果并自动分类错误。
//
// 行为：
//   - 跳过 id=0 的汇总维度
//   - 并发上限 = limit；limit<=0 时按 1（串行）执行
//   - fn 接收含 errgroup 取消传播的 ctx 和单个 dimension，返回该维度的 items 和 error
//   - 单个维度失败不中断其他维度的执行
//   - Items 按维度声明顺序拼接（与完成顺序无关），保证同输入同输出
//
// 返回的 ParallelDimsResult 包含聚合后的 items、分类后的错误列表和计数。
// egErr 是 errgroup.Wait() 返回的错误（当 goroutine 直接 return err 时触发，
// 通常只传递 context 取消信号）。
//
// fan-out 与保序落槽由 collectDims 内核承担（透传路径 FetchTasksJSON 共用
// 同一内核），本函数只负责展平分片与按 ClassifyError 口径分类错误。
func ParallelDims[T any](ctx context.Context, dims []types.Dimension, limit int, fn func(context.Context, types.Dimension) ([]T, error)) (result *ParallelDimsResult[T], egErr error) {
	batches, allErrs, egErr := collectDims(ctx, dims, limit, collectDimsOpts{}, fn)

	// 容量钳制：维度数 * 10 的预分配由服务端 getDimensions 响应驱动——
	// 恶意/异常服务端返回 1e5 维度 → 预分配 1e6 槽位 × Task
	// (30+ 字符串字段) 可达数百 MB OOM。与分页四道闸同纪律：容量上界
	// 钳到 1000（超出后 append 自动扩容，只是少一次预分配收益，语义不变）。
	const preallocCap = 1000
	capHint := len(batches) * 10
	if capHint > preallocCap {
		capHint = preallocCap
	}
	allItems := make([]T, 0, capHint)
	for i := range batches {
		allItems = append(allItems, batches[i]...)
	}
	result = &ParallelDimsResult[T]{Items: allItems}

	for _, e := range allErrs {
		switch ClassifyError(e) { //nolint:exhaustive
		case CategoryContextCancel, CategoryContextTimeout:
			result.CancelledCount++
			result.ContextErrors = append(result.ContextErrors, e)
		case CategoryNetworkTimeout, CategoryBusinessError:
			result.FailedCount++
			result.BizErrors = append(result.BizErrors, e)
		default:
			result.FailedCount++
			result.BizErrors = append(result.BizErrors, e)
		}
	}
	return result, egErr
}
