// pagination_bounds.go：写实分页安全——页数推导纯函数与四道闸上限常量。
//
// 页数推导提取动因（2026-09-26 架构核实）：下界推导在 submitted.go 与
// raw_json.go 三处逐字重复（各 4 行），limit 路径的 endPage 收敛又各写一遍。
// 四道闸常量集中动因（2026-09-27）：此前散落 raw_json.go（页数/字节/维度）
// 与 submitted.go 内联（条数），新取数路径要考古四处才能装配。
//
// 抽出为纯函数/常量的原因：这些规则是分页安全的不变量（v1.6.4 / 84982af
// 确立），必须只有一处定义，且可被行为矩阵独立验证——它们不依赖
// Client 状态、不发请求，纯 CPU 计算。
//
// 注意：这些函数只决定「要抓几页」，不得改写返回给调用方的 PageBean。
// 服务端声明的 totalPage/totalNum 必须原样透传（raw_json_limit_pages_test.go
// 锁定该契约），推导值仅用于内部翻页决策。
package client

import "math"

// ─── 分页安全上限（四道闸）───
//
// 四条内存安全闸的上限常量集中于此，让「防服务端声明驱动 make 分配 OOM」
// 的知识单点定义。此前散落 raw_json.go（页数/字节/维度）与 submitted.go
// 内联（条数），新取数路径要考古四处才能装配。各闸语义：
//
//   - maxTotalPage（页数闸）：TotalPage 来自服务端单字段声明，恶意值直接
//     驱动 make 分配会 OOM；10000 页 × pageSize=500 ≈ 500 万条，远超任何
//     真实业务数据量。
//   - maxAssembleBuffer（字节闸）：多页合并的原始字节预算。len(raw1)×totalPage
//     可达 4MB×10000=40GB 单次 make，64MB 足够覆盖任何真实拼接输出。
//   - maxSubmittedRecords（条数闸）：结构化路径的容量上界，与字节闸解耦
//     （页长可被 WithSubmittedPageSize 调大，仅页数×页长的钳制会膨胀）。
//   - maxFetchTasksDims（维度闸）：任务维度数上界，恶意值驱动全维度并发
//     拉取 × 单页 4MB 累积无预算；128 维远超任何真实学校维度集。
//
// 页数推导与钳制纯函数（derivePageBounds / clampPage / limitEndPage）同文件承载；
// 字节闸的配套纯函数（estimatePagesBudgeted / capAssembledSlice /
// cumulativeSliceBytes / budgetTruncatePage）只被透传路径消费，见 raw_json.go。
const (
	maxTotalPage        = 10000
	maxAssembleBuffer   = 64 << 20
	maxSubmittedRecords = 100_000
	maxFetchTasksDims   = 128
)

// maxSubmittedCapacityCeiling 返回「钳制页数 × pageSize」这一容量上界，
// 在乘法会溢出 int 时退回到 math.MaxInt。
//
// 调用方以除法形态比较（capacity/maxTotalPage > pageSize）判定越界，
// 因此本函数只需在乘法安全时给出精确值、溢出时给出一个必然大于任何
// capacity 的饱和值——后者随即会被下游 maxSubmittedRecords 条数闸拦下。
func maxSubmittedCapacityCeiling(pageSize int) int {
	if pageSize > 0 && maxTotalPage > math.MaxInt/pageSize {
		return math.MaxInt
	}
	return maxTotalPage * pageSize
}

// 规则：取 max(totalPage, ceil(totalNum/pageSize)) 作为下界，
// 再钳制到 [1, maxTotalPage]。
//
// 为什么用 max：服务端 totalPage 来自单一字段声明，可能虚低或为 0
// （v1.6.4 事故）。只信 totalPage 会静默漏数据，因此 totalNum 推导值
// 作为安全下界。
//
// 为什么钳制上界：totalPage 来自服务端声明，恶意/异常值（如 1e9）直接
// 驱动 make 分配会导致单请求 OOM。上界 maxTotalPage 防的是「放大」，
// 不是「翻页完整性」——超界时截断而非继续翻。
//
// 为什么下界为 1：results 索引槽以页号下标存放，首页固定在 results[1]。
func derivePageBounds(totalNum, totalPage, pageSize int) int {
	return clampPage(derivePageBoundsUnclamped(totalNum, totalPage, pageSize))
}

// derivePageBoundsUnclamped 返回 max(totalPage, ceil(totalNum/pageSize))，
// 不做上界钳制，只保留下界 1。
//
// 单独提供是因为 limit 路径的收敛顺序不同：那里需要「声明页数」的原始值
// 先与 limit 派生值做 min，收敛后才判断是否超上界。若先钳到 maxTotalPage
// 再收敛，虚高的 totalNum（如 1e9）会被当成合法 10000 页而真的去翻页，
// 违反 TestGetCirclesLimitJSON_HugeLimitClamped 锁定的
// 「超限退回首页、不再请求 page2」。
func derivePageBoundsUnclamped(totalNum, totalPage, pageSize int) int {
	if pageSize <= 0 {
		// pageSize 非法时只信任服务端声明。
		if totalPage < 1 {
			return 1
		}
		return totalPage
	}
	derivedPages := (totalNum + pageSize - 1) / pageSize
	pages := totalPage
	if pages < derivedPages {
		pages = derivedPages
	}
	if pages < 1 {
		return 1
	}
	return pages
}

// clampPage 把页数钳制到 [1, maxTotalPage]。
func clampPage(pages int) int {
	if pages > maxTotalPage {
		return maxTotalPage
	}
	if pages < 1 {
		return 1
	}
	return pages
}

// limitEndPage 返回 limit 路径实际需要抓取的最后一页页号。
//
// 规则：endPage = min(ceil((offset+limit)/pageSize), declaredPages)；
// declaredPages 必须是**未钳制**的声明值（见 derivePageBoundsUnclamped）。
//
// 收敛后若 endPage 超 maxTotalPage，直接退回 1（首页快照）而不是钳到上界——
// ：endPage 来自调用方 limit 与服务端 totalNum 声明的组合，虚高时
// 可达百万，make([]rawResult, endPage+1) 一次预分配几十 MB。防放大优先于
// 分页完整性。
//
// 为什么不按 totalPage 翻页再截断：那样会为用不到的数据发出请求
// （raw_json_limit_pages_test.go 锁定「只请求覆盖到的页」）。
func limitEndPage(offset, limit, pageSize, declaredPages int) int {
	if pageSize <= 0 {
		return clampPage(declaredPages)
	}
	need := offset + limit
	endPage := (need + pageSize - 1) / pageSize
	if endPage > declaredPages {
		endPage = declaredPages
	}
	if endPage > maxTotalPage {
		return 1
	}
	if endPage < 1 {
		return 1
	}
	return endPage
}
