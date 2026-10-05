// pkg/client 包内 1:1 透传业务 JSON 的方法族（*JSON 后缀）。
//
// 设计目标：CLI 输出 envelope.data 与平台原始 JSON byte-for-byte 一致。
// 原方法（FetchTasks / GetMyInfo / GetSubmittedCircles / QuerySelfEvaluation /
// GetHonorTypes / GetHonorList / ActivateSession）反序列化进强类型 struct，
// 存在字段裁剪 / 命名转换 / 字段顺序稳定性的差异；本文件的方法直接返回
// 服务端原始 JSON（json.RawMessage），让调用方（CLI、第三方 SDK 消费者）
// 拿到与平台一致的数据。
//
// 设计要点：
//   - 自动分页/跨维度合并仍在 SDK 内部完成，调用方仍只需传 token
//   - 自动 fallback：dataList → returnData / dataMap（按方法实际通道）
//   - GetMyInfoJSON 内部调用 GetMyInfo()；ActivateSessionJSON 走 sessionManager
//     激活后 Marshal 其返回的 UserInfo。两者均含学校信息 SSO 降级补全与班级名清理后处理
//   - 失败/取消语义与原方法一致，错误链不变

// 内存安全四道闸的上限常量集中在 pagination_bounds.go（页数/字节/条数/维度），
// 本文件只消费，不再各自定义。字节闸配套纯函数（estimatePagesBudgeted /
// capAssembledSlice / cumulativeSliceBytes / budgetTruncatePage）由本文件
// 定义，只被透传路径消费。

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// rawListBytes 返回 dataList 的原始字节。dataList 缺失时返回 nil。
// 返回 []byte 而非 RawMessage 让 bytes.Buffer 直接 append，避免反复拷贝。

// capAssembledSlice 对已累积的 rawResult 切片做总量预算截断。
// getCirclesJSON/getCirclesLimitJSON 翻页时把每页原始字节累积进 results，
// 已由 estimatePagesBudgeted（页数×首页字节）预翻页预算 + 本函数合并前
// 复核截断双层覆盖——服务端报 10000 页×4MB≈40GB 渐进填充时在真实翻页
// 前即被预预算截断，残余超预算由本函数兜底。超出预算返回 true，
// 调用方截断到已合并合法前缀。
func capAssembledSlice(raw1 []byte, results []rawResult, untilPage int) bool {
	if untilPage < 1 {
		return false
	}
	return cumulativeSliceBytes(raw1, results, untilPage) > maxAssembleBuffer
}

// cumulativeSliceBytes 统计 raw1 到 untilPage 的原始字节累积总量。
func cumulativeSliceBytes(raw1 []byte, results []rawResult, untilPage int) int {
	total := len(raw1)
	for pn := 2; pn <= untilPage && pn < len(results); pn++ {
		total += len(results[pn].raw)
	}
	return total
}

// estimatePagesBudgeted 以「页数 × 首页字节」估算多页累积量上界。
// 翻页前 results 里 pn≥2 尚未拉取，无法按实际字节统计；真实每页 ≤ 首页
// 字节（分页语义），故 endPage×len(raw1) 是安全上界。
func estimatePagesBudgeted(pageCount, firstPageLen int) int {
	if pageCount < 1 || firstPageLen < 1 {
		return 0
	}
	return pageCount * firstPageLen
}

// budgetTruncatePage 页面累积字节越过 maxAssembleBuffer 预算时，线性回退
// 找到「累积不超过预算」的最大页号。命中预算返回 (回退后页号, true)；
// 未命中返回 (declaredPages, false)。getCirclesJSON 与 getCirclesLimitJSON
// 两处共用（此前逐字重复的 9 行回退循环收敛到本函数单点）。
//
// 线性回退而非二分：页量已由 maxTotalPage 钳制到 ≤10000，且预算命中是
// 服务端异常分页的罕见路径，线性递减在真实场景至多几十次迭代，无性能压力。
// 注：旧注释称「二分找」与实现不符（实为线性递减），此处据实描述。
func budgetTruncatePage(raw1 []byte, results []rawResult, declaredPages int) (int, bool) {
	if !capAssembledSlice(raw1, results, declaredPages) {
		return declaredPages, false
	}
	budgetPage := declaredPages
	for budgetPage > 1 && cumulativeSliceBytes(raw1, results, budgetPage) > maxAssembleBuffer {
		budgetPage--
	}
	return budgetPage, true
}

// isNullJSON 判断一段原始 JSON 是否为 null 形态：字面 null 或字符串 "null"。
// 平台偶发把空列表序列化为 dataList:"null"（字符串），字面 null 在解码层已折叠为
// nil 指针，这里统一识别两种形态，归一为 nil 让调用方走空值契约（[] 或 fallback）。
func isNullJSON(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`"null"`))
}

func rawListBytes(resp types.UnifiedResponse) []byte {
	if resp.DataList == nil {
		return nil
	}
	raw := bytes.TrimSpace(*resp.DataList)
	// null 形态（字面/字符串）视为空列表，不当作非法 JSON 或原样透传
	if isNullJSON(raw) {
		return nil
	}
	if !json.Valid(raw) {
		return nil
	}
	return raw
}

// rawSingleObjectBytes 返回单个对象的原始 JSON。
// 用于 QuerySelfEvaluation 这种"returnData 优先，dataList[0] 兜底"的接口。
//
// 优先 returnData；若为字符串型 token 或为 null，尝试从 dataList 拿第一项；
// 否则尝试 dataMap（object 风格）。
//
// 返回非 nil 时一定是合法 JSON object；若都为 nil/字符串，返回 nil。
func rawSingleObjectBytes(resp types.UnifiedResponse) []byte {
	if resp.ReturnData != nil && len(*resp.ReturnData) > 0 && (*resp.ReturnData)[0] == '{' {
		return *resp.ReturnData
	}
	if resp.DataList != nil && len(*resp.DataList) > 0 {
		trimmed := bytes.TrimSpace(*resp.DataList)
		if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
			var arr []json.RawMessage
			if err := json.Unmarshal(trimmed, &arr); err == nil && len(arr) > 0 {
				return arr[0]
			}
		}
	}
	if resp.DataMap != nil && len(*resp.DataMap) > 0 && (*resp.DataMap)[0] == '{' {
		return *resp.DataMap
	}
	return nil
}

// type→type 映射说明（getStudentCircle）：
//   1 = 公示/全部     → GetPublicCircles
//   2 = 教师写实     → GetTeacherCircles
//   3 = 我发布的     → GetSubmittedCircles
//   4 = 被撤回       → GetWithdrawnCircles

// 以下 8 个入口是按写实列表类型各设一份的旧形式，除方法名与那个数字外
// 逐字相同。现已全部转发到 circle_list_type.go 的统一入口：
// ListCirclesJSON / ListCirclesLimitJSON。保留是为兼容下游调用方，行为
// 逐字不变（由 TestListCirclesJSON_EquivOldEntrypoints 与
// TestListCirclesLimitJSON_EquivOldEntrypoints 锁定），新增代码请直接用
// 统一入口——它少一层要记的名字，且类型系统保证不会传错写实列表类型。

// GetSubmittedCirclesJSON 获取当前用户自己发布的写实记录，返回平台原始 JSON 数组。
// 等价于 ListCirclesJSON(ctx, token, CircleListSubmitted, key)。
func (c *Client) GetSubmittedCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	return c.ListCirclesJSON(ctx, token, CircleListSubmitted, key)
}

// GetSubmittedCirclesLimitJSON 按偏移和条数限制拉取当前用户自己发布的写实记录。
// 等价于 ListCirclesLimitJSON(ctx, token, CircleListSubmitted, offset, limit, key)。
func (c *Client) GetSubmittedCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.ListCirclesLimitJSON(ctx, token, CircleListSubmitted, offset, limit, key)
}

// GetTeacherCirclesJSON 获取教师代写的全部写实记录，返回平台原始 JSON 数组。
// 等价于 ListCirclesJSON(ctx, token, CircleListTeacher, key)。
func (c *Client) GetTeacherCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	return c.ListCirclesJSON(ctx, token, CircleListTeacher, key)
}

// GetTeacherCirclesLimitJSON 按偏移和条数限制拉取教师写实记录。
// 等价于 ListCirclesLimitJSON(ctx, token, CircleListTeacher, offset, limit, key)。
func (c *Client) GetTeacherCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.ListCirclesLimitJSON(ctx, token, CircleListTeacher, offset, limit, key)
}

// GetWithdrawnCirclesJSON 获取被撤回的全部写实记录，返回平台原始 JSON 数组。
// 等价于 ListCirclesJSON(ctx, token, CircleListWithdrawn, key)。
func (c *Client) GetWithdrawnCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	return c.ListCirclesJSON(ctx, token, CircleListWithdrawn, key)
}

// GetWithdrawnCirclesLimitJSON 按偏移和条数限制拉取被撤回写实记录。
// 等价于 ListCirclesLimitJSON(ctx, token, CircleListWithdrawn, offset, limit, key)。
func (c *Client) GetWithdrawnCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.ListCirclesLimitJSON(ctx, token, CircleListWithdrawn, offset, limit, key)
}

// GetPublicCirclesJSON 获取公示的全部写实记录（全班），返回平台原始 JSON 数组。
// 等价于 ListCirclesJSON(ctx, token, CircleListPublic, key)。
func (c *Client) GetPublicCirclesJSON(ctx context.Context, token string, key string) (json.RawMessage, error) {
	return c.ListCirclesJSON(ctx, token, CircleListPublic, key)
}

// GetPublicCirclesLimitJSON 按偏移和条数限制拉取公示写实记录。
// 等价于 ListCirclesLimitJSON(ctx, token, CircleListPublic, offset, limit, key)。
func (c *Client) GetPublicCirclesLimitJSON(ctx context.Context, token string, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	return c.ListCirclesLimitJSON(ctx, token, CircleListPublic, offset, limit, key)
}

// rawResult 存储单页原始 JSON 数据。
type rawResult struct {
	raw []byte
}

// assembleCirclesJSON 将多页数据按页号顺序拼接为单个 JSON 数组。
//
// raw1 是第一页原始 JSON，results 是按页号索引的后续页数据。
// 成功路径返回完整 JSON 数组，失败路径通过 trimArrayToCurrent 截断为已有部分。
//
// 调用方不变式：len(results) 必须大于 totalPage，即 results 长度与页号上界
// 同步。二者的分配都在 getCirclesJSON 内（make(len=declaredPages+1) 与
// budgetTruncatePage 回退后的页号），本函数按该约定直接索引 results[pn]。
//
// 为什么不加边界防护：cumulativeSliceBytes 与 assembleCirclesLimitJSON 写的是
// `pn < len(results)`，形态确与此处不同，但那两处是被当预算扫描器使用的全函数
// （测试刻意传越界页号验证其不 panic），本处是断言式访问。服务端的攻击面只有
// totalNum / totalPage 的上界，已被 derivePageBounds 与本函数的 maxTotalPage
// 钳制双重夹住；收缩方向服务端无法驱动，budgetTruncatePage 的回退只会让页号
// 变小。为不可达的越界加静默截断，会把「页号越界」从显式契约变成隐式容忍。
// 新增合并点时必须同样保证 results 长度与页号上界同步。
//
// 用 first 标志控制逗号，避免 page1 为空数组时产生 leading comma 非法 JSON（[,{...}]）。
// 对齐 assembleCirclesLimitJSON 的拼接策略。
func assembleCirclesJSON(raw1 []byte, results []rawResult, totalPage int, partialErr error) (json.RawMessage, error) {
	// 预分配容量由 assembleCapacity 单点计算并施加两道闸（页数闸 +
	// maxAssembleBuffer 字节闸），理由见该函数注释。
	total := assembleCapacity(raw1, results, totalPage)
	buf := bytes.NewBuffer(make([]byte, 0, total))
	buf.WriteByte('[')
	first := true
	// page1 可能为空数组 "[]"，trim 后长度为 0，不得写逗号
	if trimmed := trimArrayBrackets(raw1); len(trimmed) > 0 {
		buf.Write(trimmed)
		first = false
	}
	for pn := 2; pn <= totalPage; pn++ {
		if len(results[pn].raw) == 0 {
			continue
		}
		trimmed := trimArrayBrackets(results[pn].raw)
		if len(trimmed) == 0 {
			continue
		}
		if first {
			buf.Write(trimmed)
			first = false
		} else {
			buf.WriteByte(',')
			buf.Write(trimmed)
		}
	}
	buf.WriteByte(']')
	if partialErr != nil {
		return json.RawMessage(trimArrayToCurrent(buf.Bytes())), partialErr
	}
	return buf.Bytes(), nil
}

// assembleCapacity 计算 assembleCirclesJSON 的预分配容量，是两道容量闸的
// 唯一实现处。
//
// 两道闸依次施加：
//   - 页数闸：totalPage 超 maxTotalPage 会放大下面的求和并整数溢出，须先钳制。
//     totalPage 来自服务端声明，攻陷服务端可借虚高 totalNum 驱动巨量分配。
//   - 字节闸：求和结果超 maxAssembleBuffer 时钳到上界。各页长度已知，求和
//     即精确容量（此前估算 len(raw1)×totalPage 偏小，会触发 bytes.Buffer
//     倍增扩容的多次整块拷贝）。
//
// 提取为纯函数而非留在调用点：它是纯算术，输入全是已知长度，不发请求也不分配。
// 留在调用点时「容量被钳到上界」只能靠构造超大输入观察，而实测本机连
// make(40GB) 都能分配成功（虚拟内存），这类测试既慢又不可靠；抽出来后可直接
// 断言返回值，且删掉任一道闸断言即变红。
//
// 页号越界不做防护：调用方保证 results 长度与 totalPage 同步。相邻两条装配
// 函数写 pn < len(results) 守卫是因为它们被当预算扫描器使用、测试会刻意传
// 越界页号——本函数无此用法。
func assembleCapacity(raw1 []byte, results []rawResult, totalPage int) int {
	if totalPage > maxTotalPage {
		totalPage = maxTotalPage
	}
	total := len(raw1) + 2
	for pn := 2; pn <= totalPage; pn++ {
		total += len(results[pn].raw) + 1
	}
	if total > maxAssembleBuffer {
		total = maxAssembleBuffer
	}
	return total
}

// getCirclesJSON 是各类型写实记录全量拉取的通用实现。
//
// 多页时使用 errgroup 并发翻页（与 fetchAllCirclePages 对齐），
// 避免串行循环在数据量大时慢 2-5 倍。
// key 透传到 getStudentCircle 的 key 查询参数。
func (c *Client) getCirclesJSON(ctx context.Context, token string, circleType int, key string, methodName string) (json.RawMessage, *types.PageBean, error) {
	pageSize := c.effectivePageSize()

	pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType, key)
	if err != nil {
		return nil, nil, fmt.Errorf("%s 失败: %w", methodName, err)
	}

	if pb == nil || pb.TotalNum <= pageSize {
		if len(raw1) == 0 {
			return []byte("[]"), pb, nil
		}
		return raw1, pb, nil
	}

	// 多页：预分配索引切片 + errgroup 并发翻页，保持页号顺序
	// TotalPage 来自服务端单字段声明，恶意/异常值直接驱动 make 分配——
	// 服务端被攻陷时单请求 OOM 崩进程。超钳制值时直接截断（只返回首页），
	// 不翻页——钳制的意义是防放大而不是真翻 10000 页。
	// 页数下界与上界钳制统一由 derivePageBounds 负责（v1.6.4 / 84982af）：
	// 下界取 max(totalPage, ceil(totalNum/pageSize)) 防 totalPage 虚低漏页，
	// 上界钳到 maxTotalPage 防服务端声明值驱动 make 分配 OOM。
	declaredPages := derivePageBounds(pb.TotalNum, pb.TotalPage, pageSize)
	// 全量路径在 make 前补「页数 × 首页字节」预算守卫，与
	// getCirclesLimitJSON 的预估守卫同纪律。此前翻页后才由
	// capAssembledSlice 复核截断——最坏 10000 页 × 4MB 逐页填充至 40GB，
	// errgroup 已真实发出所有请求才在 g.Wait 后截断（内存放大 + 无谓翻页）。
	// 越界直接截断到首页快照，不翻页（防放大优先于全量完整性）。
	if declaredPages > 1 && estimatePagesBudgeted(declaredPages, len(raw1)) > maxAssembleBuffer {
		slog.Warn("raw_json: 全量翻页预估累积量超过合并预算，截断到首页",
			"estimated_bytes", estimatePagesBudgeted(declaredPages, len(raw1)), "max", maxAssembleBuffer)
		return raw1, pb, nil
	}
	results := make([]rawResult, declaredPages+1)
	results[1] = rawResult{raw: raw1}

	if err := c.fetchRawCirclePages(ctx, token, declaredPages, pageSize, circleType, key, results); err != nil {
		// 部分失败时，已成功的页仍有效；按已有页顺序拼接
		raw, assembleErr := assembleCirclesJSON(raw1, results, declaredPages,
			fmt.Errorf("%s 部分页失败: %w", methodName, err))
		return raw, pb, assembleErr
	}

	// 页面累积原始字节越过 maxAssembleBuffer 预算时截断到已合并
	// 前缀（对齐 submitted 条数闸的语义；这里按字节而非条数判）。
	// 修正：命中预算后必须传已钳制页数（而非声明页数）给
	// assembleCirclesJSON——此前传 declaredPages 让 Bytes.Buffer 仍无上限
	// 增长再做第二次全量拷贝，与「截断防放大」目标矛盾。
	if budgetPage, hit := budgetTruncatePage(raw1, results, declaredPages); hit {
		slog.Warn("raw_json: 多页累积量超过合并预算，截断到已合并前缀",
			"total_bytes", cumulativeSliceBytes(raw1, results, declaredPages),
			"budget_page", budgetPage, "max", maxAssembleBuffer)
		raw, assembleErr := assembleCirclesJSON(raw1, results, budgetPage, nil)
		return raw, pb, assembleErr
	}

	raw, assembleErr := assembleCirclesJSON(raw1, results, declaredPages, nil)
	return raw, pb, assembleErr
}

// getCirclesLimitJSON 是各类型写实记录按偏移/条数限制拉取的通用实现。
//
// 多页时使用 errgroup 并发翻页，但只请求 offset/limit 覆盖到的页：
// endPage = min(TotalPage, ceil((offset+limit)/pageSize))，
// 避免全量翻页再截断造成的多余请求。
// key 透传到 getStudentCircle 的 key 查询参数。
func (c *Client) getCirclesLimitJSON(ctx context.Context, token string, offset, limit int, circleType int, key string, methodName string) (json.RawMessage, *types.PageBean, error) {
	pageSize := c.effectivePageSize()

	if limit <= 0 {
		raw, pb, err := c.getCirclesJSON(ctx, token, circleType, key, methodName)
		return raw, pb, err
	}

	pb, raw1, err := c.fetchCirclePageJSON(ctx, token, 1, pageSize, circleType, key)
	if err != nil {
		return nil, nil, fmt.Errorf("%s 失败: %w", methodName, err)
	}
	if pb == nil || pb.TotalNum == 0 {
		return []byte("[]"), pb, nil
	}
	if offset >= pb.TotalNum {
		return []byte("[]"), pb, nil
	}

	// 只拉到覆盖 offset+limit 的最后一页，不全量翻页。
	//
	// 注意：limitEndPage 的收敛必须用**未钳制**的声明页数。若传
	// derivePageBounds 的结果（已钳到 maxTotalPage），totalNum=1e9 场景会
	// 算出 endPage=10000 并真的翻 10000 页，违反 锁定的
	// 「超限退回首页、不再翻页」。
	declaredPages := derivePageBoundsUnclamped(pb.TotalNum, pb.TotalPage, pageSize)
	endPage := limitEndPage(offset, limit, pageSize, declaredPages)

	// （修正）：getCirclesLimitJSON 的预翻页估算必须以
	// 「页数 × 首页字节」作上界——此前 capAssembledSlice 看 results 里
	// 已拉取字节（pn≥2 尚未拉全为 nil），concat 只有 len(raw1)≤4MB，
	// >64MB 预算永不命中，估算守卫形同虚设。用 endPage×len(raw1) 作为
	// 单页最坏上界（真实页面 ≤ 首页），越界直接截断到首页。
	results := make([]rawResult, endPage+1)
	results[1] = rawResult{raw: raw1}
	if endPage > 1 && estimatePagesBudgeted(endPage, len(raw1)) > maxAssembleBuffer {
		slog.Warn("raw_json: limit 翻页预估累积量超过合并预算，截断到首页",
			"estimated_bytes", estimatePagesBudgeted(endPage, len(raw1)), "max", maxAssembleBuffer)
		endPage = 1
	}

	if endPage > 1 {
		if err := c.fetchRawCirclePages(ctx, token, endPage, pageSize, circleType, key, results); err != nil {
			// 部分失败时，已成功的页仍有效；按已有页顺序拼接
			return c.assembleCirclesLimitJSON(results, pb, offset, limit, endPage, methodName,
				fmt.Errorf("%s 部分页失败: %w", methodName, err))
		}

		// 翻页完成后做合并前累积字节复核——estimatePagesBudgeted
		// 只以「每页 ≤ 首页字节」为假设，服务端分页异常（后续页
		// 实际字节超过首页）可绕过估算预算。与 getCirclesJSON 的同纪律口径
		// 对齐：实际累积越过 maxAssembleBuffer 时线性回退到合法前缀，再交给
		// assembleCirclesLimitJSON 输出（保证 Bytes.Buffer 永不无上限增长）。
		if budgetPage, hit := budgetTruncatePage(raw1, results, endPage); hit {
			slog.Warn("raw_json: limit 多页累积量超过合并预算，截断到已合并前缀",
				"total_bytes", cumulativeSliceBytes(raw1, results, endPage),
				"budget_page", budgetPage, "max", maxAssembleBuffer)
			return c.assembleCirclesLimitJSON(results, pb, offset, limit, budgetPage, methodName, nil)
		}
	}

	return c.assembleCirclesLimitJSON(results, pb, offset, limit, endPage, methodName, nil)
}

// assembleCirclesLimitJSON 将并发翻页结果按 offset/limit 规则拼接为最终 JSON 数组。
// endPage 是实际请求到的最大页号（<= TotalPage），results 长度对应 endPage+1。
func (c *Client) assembleCirclesLimitJSON(results []rawResult, pb *types.PageBean, offset, limit, endPage int, methodName string, partialErr error) (json.RawMessage, *types.PageBean, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	buf.WriteByte('[')
	first := true
	taken := 0
	skipped := 0

	// 按页号顺序处理数据，应用 offset/limit 规则（只遍历已请求页）。
	// endPage 已由调用方按 totalNum/totalPage 推导并与 results 长度对齐，
	// 不能再次按原始 totalPage 截断，否则虚低声明会丢失已请求页。
	for pn := 1; pn <= endPage && pn < len(results); pn++ {
		if len(results[pn].raw) == 0 {
			continue
		}
		skipped, taken = appendPageRange(buf, results[pn].raw, &first, taken, offset, limit, skipped)
		if taken >= limit {
			break
		}
	}

	buf.WriteByte(']')
	if partialErr != nil {
		return json.RawMessage(trimArrayToCurrent(buf.Bytes())), pb, partialErr
	}
	return buf.Bytes(), pb, nil
}

// appendPageRange 从一页原始 JSON 数组中按 offset/limit 规则逐条取出写入 buf。
//
// pageRaw 是 "[{...},{...}]" 格式。用深度扫描分割 JSON 对象，不反序列化为 Go struct。
// 注意：深度扫描感知 JSON 字符串边界，避免字符串内的花括号被误计为对象边界。
func appendPageRange(buf *bytes.Buffer, pageRaw []byte, first *bool, taken, offset, limit, skipped int) (int, int) {
	trimmed := trimArrayBrackets(pageRaw)
	if len(trimmed) == 0 {
		return skipped, taken
	}
	start := 0
	depth := 0
	inString := false
	for i := 0; i <= len(trimmed) && taken < limit; i++ {
		if i < len(trimmed) {
			ch := trimmed[i]
			// JSON 字符串边界感知：在字符串内时，跳过转义字符，只在非转义的 " 处切换状态
			if inString {
				if ch == '\\' && i+1 < len(trimmed) {
					i++ // 跳过转义字符（如 \"、\\、\n 等）
				} else if ch == '"' {
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
			case ',':
				if depth == 0 {
					obj := bytes.TrimSpace(trimmed[start:i])
					if len(obj) > 0 {
						skipped, taken = emitJSONObject(buf, first, taken, offset, limit, skipped, obj)
					}
					start = i + 1
				}
			}
		} else {
			obj := bytes.TrimSpace(trimmed[start:])
			if len(obj) > 0 {
				skipped, taken = emitJSONObject(buf, first, taken, offset, limit, skipped, obj)
			}
		}
	}
	return skipped, taken
}

// emitJSONObject 决定是否将一段 JSON 对象写入 buf。
func emitJSONObject(buf *bytes.Buffer, first *bool, taken, offset, limit, skipped int, obj []byte) (int, int) {
	if skipped < offset {
		return skipped + 1, taken
	}
	if *first {
		buf.Write(obj)
		*first = false
	} else {
		buf.WriteByte(',')
		buf.Write(obj)
	}
	return skipped + 1, taken + 1
}

// trimArrayBrackets 去掉 JSON 数组的首尾方括号（用于多页拼接）。
// 输入必须是合法的 JSON 数组；空数组返回空字节。
func trimArrayBrackets(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
		return trimmed[1 : len(trimmed)-1]
	}
	return nil
}

// trimArrayToCurrent 关闭已写入的 JSON 数组（补齐末尾 ']'），用于 ctx 取消 / 翻页失败
// 时让已合并的部分仍是合法 JSON，便于调用方 partial envelope 渲染。
func trimArrayToCurrent(buf []byte) []byte {
	trimmed := bytes.TrimRight(buf, ",")
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] != ']' {
		return append(trimmed, ']')
	}
	return trimmed
}

// FetchTasksJSON 拉取全维度任务列表，返回平台原始 JSON 数组（跨维度合并）。
//
// 自动跨维度并发拉取并在 SDK 内部合并为单一 JSON 数组，保留所有平台字段。
// 错误聚合与 FetchTasks 对齐：
//   - context 取消且无 partial → ErrRetryable
//   - context 取消且有 partial → ErrBusinessRejected + ErrRetryable + 已合并字节
//   - 业务错误 → 仍返回已合并的字节 + ErrBusinessRejected（cmd 层 partial envelope）
func (c *Client) FetchTasksJSON(ctx context.Context, token string) (json.RawMessage, error) {
	if _, err := c.ActivateSession(ctx, token); err != nil {
		return nil, fmt.Errorf("FetchTasksJSON 预热 session 失败: %w", err)
	}

	dimensions, err := c.fetchDimensions(ctx, token, "FetchTasksJSON getDimensions")
	if err != nil {
		return nil, err
	}

	headers := c.bizHeaders(token)

	// 并发 fan-out、保序落槽与取消传播由 collectDims 内核承担
	// （与结构化路径 FetchTasks 共用同一内核，不再各写一份）。
	// 本路径的差异：维度级 context 错误向 errgroup 传播并记入 dimErrs，
	// 供后续 partial 分支拼出可重试语义；并发上限与维度数上钳沿用既有口径。
	results, dimErrs, gErr := collectDims(ctx, dimensions, fetchTasksConcurrentLimit, collectDimsOpts{
		maxDims:            maxFetchTasksDims,
		propagateDimCancel: true,
		recordCancelled:    true,
	}, func(gctx context.Context, dim types.Dimension) ([]byte, error) {
		return c.fetchTasksDimensionJSON(gctx, dim, headers)
	})
	if len(results) == 0 {
		return []byte("[]"), nil
	}

	// 先按维度顺序拼装已有结果，供 cancel / partial 路径复用
	assemble := func() (json.RawMessage, int) {
		buf := bytes.NewBuffer(make([]byte, 0, 2048))
		buf.WriteByte('[')
		first := true
		totalPages := 0
		// 累积字节预算（maxAssembleBuffer），与 getCirclesJSON 用同一常量与
		// 同一截断形态，但**位置不同**，不是同量级纪律：
		//
		// - getCirclesJSON 在发翻页请求**之前**用 estimatePagesBudgeted 预估
		//   并拦截，本路径的预算判在 assemble() 内，即所有维度请求发完、
		//   分片全部驻留之后，只约束输出侧不约束传输侧。
		// - 驻留由另两道闸钉成常数上界：维度闸（collectDims 的 maxDims，
		//   发请求前截断维度列表）× 单维限读（maxResponseBodySize 4MiB），
		//   最坏 128×4MiB=512MiB。实测同等恶意服务端下写实路径堆峰值
		//   11.1MiB（只发 1 次请求）、本路径 776MiB（发 128 次）——
		//   差约 70 倍，故「对齐」仅指共用常量与截断形态，不指风险同量级。
		//   契约由 fetch_tasks_residency_test.go 锁定。
		assembledLen := 0
		for _, raw := range results {
			if len(raw) == 0 {
				continue
			}
			trimmed := trimArrayBrackets(raw)
			if len(trimmed) == 0 {
				continue
			}
			if assembledLen+len(trimmed)+2 > maxAssembleBuffer {
				slog.Warn("FetchTasksJSON: 累积字节超过预算，截断到已合并前缀",
					"assembled", assembledLen, "max", maxAssembleBuffer)
				break
			}
			if first {
				buf.Write(trimmed)
				first = false
			} else {
				buf.WriteByte(',')
				buf.Write(trimmed)
			}
			assembledLen += len(trimmed) + 1
			totalPages++
		}
		buf.WriteByte(']')
		return buf.Bytes(), totalPages
	}

	if gErr != nil {
		if isContextError(gErr) {
			merged, n := assemble()
			return merged, partialTasksOutcome(n > 0, nil, nil, 0, "FetchTasksJSON", gErr)
		}
		return nil, fmt.Errorf("FetchTasksJSON 并发拉取失败: %w", gErr)
	}

	merged, totalPages := assemble()

	if totalPages == 0 {
		if len(dimErrs) > 0 {
			// 错误分类与结构化路径共用同一口径（classifyDimErrors），
			// 不再在本路径手写第二份判定循环。
			bizErrs, ctxErrs, cancelledCount := classifyDimErrors(dimErrs)
			return nil, partialTasksOutcome(false, bizErrs, ctxErrs, cancelledCount, "FetchTasksJSON", nil)
		}
		return []byte("[]"), nil
	}

	if len(dimErrs) > 0 {
		bizErrs, ctxErrs, cancelledCount := classifyDimErrors(dimErrs)
		return merged, partialTasksOutcome(true, bizErrs, ctxErrs, cancelledCount, "FetchTasksJSON", nil)
	}
	return merged, nil
}

// fetchTasksDimensionJSON 拉取单个维度的任务 dataList 原始字节。
// 请求段（httpDo→解析→业务码）由 doBizGetRaw 单点承载；
// 空 dataList 归一为 [] 与维度上下文错误包装是本调用点知识。
func (c *Client) fetchTasksDimensionJSON(ctx context.Context, dim types.Dimension, headers map[string]string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	statURL := "/api/studentCircleNew/getCircleStatistics?dimensionId=" + strconv.FormatInt(dim.ID, 10)
	resp, err := c.doBizGetRaw(ctx, fmt.Sprintf("维度 %d(%s)", dim.ID, dim.Name), statURL, headers)
	if err != nil {
		return nil, err
	}
	if resp.DataList == nil {
		return []byte("[]"), nil
	}
	return *resp.DataList, nil
}

// marshalUserInfoJSON 将 UserInfo 序列化为 JSON，输出前剔除 StudentUuid 敏感值。
//
// （user-info 域）：StudentUuid 是密码/学生 UUID 载体（写侧），
// 前端 modifyBox.vue:185 读取 getMyInfo 响应后显式清零佐证其只写不读的敏感属性。
// 结构化 GetMyInfo 返回的 info 保留该字段（Go 调用方自行裁决）；
// 但 JSON 透传路径（CLI whoami / GetMyInfoJSON / ActivateSessionJSON 消费）必须剔除，
// 防止敏感值经 envelope 原样透给脚本消费者与日志。
//
// 关键约束：info 与 sm.cachedUserInfo 共享同一指针，禁止原地置空；
// 必须浅拷贝副本后置空。拷贝只涉及 string 字段，成本 O(1) 级。
func marshalUserInfoJSON(info *types.UserInfo, caller string) (json.RawMessage, error) {
	if info == nil {
		return nil, nil
	}
	copyInfo := *info
	copyInfo.StudentUuid = ""
	raw, err := json.Marshal(&copyInfo)
	if err != nil {
		return nil, fmt.Errorf("%s 序列化失败: %w", caller, err)
	}
	return raw, nil
}

// ActivateSessionJSON 激活业务 session，返回 /api/studentInfo/getMyInfo 的 JSON（经 SDK 后处理）。
//
// 经 sessionManager 激活（含 4 步 HAR 请求），Marshal 其返回的后处理 UserInfo 为 JSON。
// 包含学校信息 SSO 降级补全、班级名年级前缀清理等后处理。
// 同时附带 4 步 HAR 激活所需的 HTTP 请求（首页 → getMenu ×2 → getMyInfo），
// 即使下游不消费返回的 UserInfo 也必须执行完这 4 步。
//
// 返回值：
//   - json.RawMessage：处理后的 UserInfo JSON 字节；数据为空时返回 nil。
//   - error：网络/解析/业务错误（含 ErrEmptyUserInfo / ErrSessionBackoff）
func (c *Client) ActivateSessionJSON(ctx context.Context, token string) (json.RawMessage, error) {
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		// 空数据（ErrEmptyUserInfo）透传，与 GetMyInfoJSON 对齐：CLI 侧
		// session activate 与 whoami 两条路径统一按 ErrEmptyUserInfo 处理，
		// 消除「ActivateSessionJSON 吞哨兵 → CLI 死分支」的空语义分裂。
		return nil, err
	}
	if info == nil {
		return nil, ErrEmptyUserInfo
	}
	return marshalUserInfoJSON(info, "ActivateSessionJSON")
}

// GetMyInfoJSON 获取当前用户完整个人资料的 JSON（经 SDK 后处理）。
//
// 内部调用 GetMyInfo() 获取已后处理的 UserInfo struct，再 Marshal 回 JSON。
// 包含 GetMyInfo 的全部后处理：学校信息 SSO 降级补全、班级名年级前缀清理。
//
// 返回值：
//   - json.RawMessage：处理后的 UserInfo JSON 字节；
//     当数据为空时返回 (nil, ErrEmptyUserInfo)。
//   - error：网络/解析/业务错误
func (c *Client) GetMyInfoJSON(ctx context.Context, token string) (json.RawMessage, error) {
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	return marshalUserInfoJSON(info, "GetMyInfoJSON")
}

// QuerySelfEvaluationJSON 查询自我评价状态的原始 JSON。
//
// 等价 QuerySelfEvaluation 但保留平台原始字段。
// 结构化方法内部是 returnData → dataMap → dataList[0] 三解码器 fallback 链；
// 本方法走 rawSingleObjectBytes：returnData 对象优先，其次 dataList[0]，再 dataMap。
// 对账口径：前端唯一读取通道是 dataMap（mainLeft.vue）；若服务端异常地
// 双容器并存且内容不同，本方法透传的是 returnData 内容，与网页所见可能
// 不一致——对账请以结构化方法或 dataMap 字段为准。
func (c *Client) QuerySelfEvaluationJSON(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "QuerySelfEvaluationJSON",
		"/api/studentMoralEduNew/querySelfEvaluation", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	raw := rawSingleObjectBytes(*resp)
	if len(raw) == 0 {
		return nil, nil
	}
	return raw, nil
}

// QuerySelfGradEvaluationJSON 查询毕业评价状态的原始 JSON。
//
// 等价 QuerySelfGradEvaluation 但保留平台原始字段。
// 前端 mainLeft.vue 主读 dataMap.student_comment / isGrad；
// 本方法走 rawSingleObjectBytes：returnData 对象优先，其次 dataList[0]，再 dataMap。
// 对账口径：若服务端异常地双容器并存且内容不同，透传的是 returnData 内容，
// 与网页所见的 dataMap 可能不一致——对账请以结构化方法或 dataMap 字段为准。
// 空数据时返回 (nil, nil)。
func (c *Client) QuerySelfGradEvaluationJSON(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "QuerySelfGradEvaluationJSON",
		"/api/studentMoralEduNew/querySelfGradEvaluation", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	raw := rawSingleObjectBytes(*resp)
	if len(raw) == 0 {
		return nil, nil
	}
	return raw, nil
}

// GetHonorTypesJSON 获取所有荣誉类型的原始 JSON 数组。
//
// 等价 GetHonorTypes 但保留平台原始字段（如备注 / 启用状态 / 上传附件要求等）。
// 自动 fallback：dataList（首选）→ returnData（兼容）。
func (c *Client) GetHonorTypesJSON(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorTypesJSON",
		"/api/studentMoralEduNew/getHonorType", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorTypesJSON 失败: %w", err)
	}
	raw := rawListBytes(*resp)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]")) {
		raw = nil
		if resp.ReturnData != nil {
			raw = *resp.ReturnData
		}
	}
	trimmed = bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`""`)) || bytes.Equal(trimmed, []byte("[]")) {
		return []byte("[]"), nil
	}
	return raw, nil
}

// assembleRecordsPageJSON 将 UnifiedResponse 拼装为 {"records":..., "page":...} 格式。
//
// records 取自 dataList（缺失时回退为 []），page 取自 pageBean（缺失时省略该字段）。
// 被 GetHonorListJSON 和 GetTypicalCaseListJSON 共用，避免重复拼装逻辑。
func assembleRecordsPageJSON(resp *types.UnifiedResponse) json.RawMessage {
	var recordsRaw json.RawMessage
	if resp.DataList != nil {
		recordsRaw = *resp.DataList
		// dataList 的 null 形态（字面 null / 字符串 "null"）先归一，
		// 防脏数组元素 ["null"]。rawListBytes 同款归一（F1 覆盖 getStudentCircle/
		// getHonorType，本函数是 GetHonorListJSON/GetTypicalCaseListJSON 共用拼装点）。
		if isNullJSON(recordsRaw) {
			recordsRaw = nil
		}
	}
	var pageBeanRaw json.RawMessage
	if resp.PageBean != nil {
		pageBeanRaw = *resp.PageBean
	}

	if len(recordsRaw) == 0 {
		recordsRaw = json.RawMessage("[]")
	}
	var buf bytes.Buffer
	buf.WriteString(`{"records":`)
	buf.Write(recordsRaw)
	if len(pageBeanRaw) > 0 {
		buf.WriteString(`,"page":`)
		buf.Write(pageBeanRaw)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// GetHonorListJSON 获取指定页荣誉记录的原始 JSON（不含自动翻页）。
//
// 与 GetHonorList 1:1 等价（按页调用，不自动翻页），
// 返回拼装后的完整 JSON 对象 `{"records":..., "page":...}`，
// records 和 page 字段值都是平台原始字节。
// key 为搜索关键字（可空，会做 URL 转义）。
func (c *Client) GetHonorListJSON(ctx context.Context, token string, pageNo, pageSize int, key string) (json.RawMessage, error) {
	path := "/api/studentMoralEduNew/getHonorByStudentId?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize) + "&key=" + url.QueryEscape(key)
	resp, err := c.doBizAndDecode(ctx, token, "GetHonorListJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHonorListJSON 失败: %w", err)
	}
	return assembleRecordsPageJSON(resp), nil
}
