// assembleCirclesJSON 白盒测试：第一页空数组时不得产生 leading comma 非法 JSON。
package client

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestAssembleCirclesJSON_EmptyFirstPage_NoLeadingComma 锁定：
// page1=[]、page2 有数据时，拼接结果必须是合法 JSON（如 [{...}]），不能是 [,{...}]。
func TestAssembleCirclesJSON_EmptyFirstPage_NoLeadingComma(t *testing.T) {
	raw1 := []byte("[]")
	results := make([]rawResult, 3)
	results[2] = rawResult{raw: []byte(`[{"id":200}]`)}

	out, err := assembleCirclesJSON(raw1, results, 2, nil)
	if err != nil {
		t.Fatalf("assembleCirclesJSON 不应返回 error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("结果不应为空")
	}
	// 非法形态：[,{...}]
	if string(out)[0:2] == "[," {
		t.Fatalf("leading comma 非法 JSON: %s", out)
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(out, &arr); jerr != nil {
		t.Fatalf("拼接结果必须是合法 JSON 数组: body=%s err=%v", out, jerr)
	}
	if len(arr) != 1 {
		t.Fatalf("期望 1 条记录, 得到 %d body=%s", len(arr), out)
	}
	if id, _ := arr[0]["id"].(float64); id != 200 {
		t.Errorf("期望 id=200, 得到 %v", arr[0]["id"])
	}
}

// TestAssembleCirclesJSON_EmptyFirstPage_PartialPath 部分失败路径同样不得 leading comma。
func TestAssembleCirclesJSON_EmptyFirstPage_PartialPath(t *testing.T) {
	raw1 := []byte("[]")
	results := make([]rawResult, 4)
	results[2] = rawResult{raw: []byte(`[{"id":1}]`)}
	// page3 缺失（模拟失败页）

	out, err := assembleCirclesJSON(raw1, results, 3, errPartialStub{})
	if err == nil {
		t.Fatal("partialErr 非 nil 时应透传 error")
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(out, &arr); jerr != nil {
		t.Fatalf("partial 路径结果仍须合法 JSON: body=%s err=%v", out, jerr)
	}
	if len(arr) != 1 {
		t.Fatalf("期望 1 条已合并记录, 得到 %d body=%s", len(arr), out)
	}
}

// errPartialStub 仅作 partialErr 占位。
type errPartialStub struct{}

func (errPartialStub) Error() string { return "partial" }

// TestAssembleCirclesJSON_CapHintClamped 锁定两道容量闸真的生效。
//
// 为什么断言 assembleCapacity 而非 assembleCirclesJSON 的返回值：「容量被
// 钳到上界」原本只能靠构造超大输入观察——而实测本机 make(40GB) 都能分配
// 成功（虚拟内存），这类测试既慢又不可靠。前一版测试正因此退化成断言
// 「函数不 panic 且 len(out) != 0」，删掉两道闸后仍然全绿（已变异验证）。
// 两道闸的计算已提取为纯函数 assembleCapacity，可直接断言其返回值。
func TestAssembleCirclesJSON_CapHintClamped(t *testing.T) {
	t.Run("字节闸：各页长度之和超上界时被钳到maxAssembleBuffer", func(t *testing.T) {
		// 每页 4MB、共 100 页 → 求和约 400MB，远超 64MB 上界。页数本身
		// 合法（100 < maxTotalPage），使字节闸成为唯一生效的闸，从而把两道
		// 闸的判据分开。
		//
		// 所有页共用同一个切片：assembleCapacity 只读 len(results[pn].raw)，
		// 求和结果与「每页各自分配」完全一致。逐页分配会让本用例吃掉约 396MiB
		// 堆，而 CI 的 -race 会把影子内存放大 2-4 倍——这是为一条断言付不
		// 起的代价。共享后堆增量降到 4MiB，判据等价。
		const pages = 100
		shared := make([]byte, maxResponseBodySize)
		results := make([]rawResult, pages+1)
		for i := 2; i <= pages; i++ {
			results[i] = rawResult{raw: shared}
		}
		got := assembleCapacity([]byte("[]"), results, pages)
		if got != maxAssembleBuffer {
			t.Fatalf("字节闸未生效：assembleCapacity = %d，期望钳到 maxAssembleBuffer = %d",
				got, maxAssembleBuffer)
		}
	})

	t.Run("字节闸：未超上界时按实际内容精确求和", func(t *testing.T) {
		// 三页、每页 10 字节 → len(raw1)+2 + 2×(10+1) = 34。这条同时锁住
		// 「求和式」本身：若改成页数×首页字节的旧估算，结果会不同。
		results := make([]rawResult, 4)
		results[2] = rawResult{raw: make([]byte, 10)}
		results[3] = rawResult{raw: make([]byte, 10)}
		const want = len("[]") + 2 + 2*(10+1)
		if got := assembleCapacity([]byte("[]"), results, 3); got != want {
			t.Fatalf("精确求和 = %d，期望 %d", got, want)
		}
	})

	t.Run("页数闸：超大页号不击穿纯函数", func(t *testing.T) {
		// 真实判据是「不 panic」：删掉页数闸后求和会遍历到 maxTotalPage+1，
		// 在断言执行之前就 index out of range。
		//
		// 因此本例不写成「返回值不超过字节闸上界」——results 全为 nil 时
		// total = 2+2+(maxTotalPage-1) 远小于 64MiB，那条断言恒为假、只会
		// 给人「断言在起作用」的错觉。页数闸在生产本就不可达（三个调用点的
		// 页号都经 derivePageBounds 或 budgetTruncatePage 钳制，探针实测
		// 病态输入下恒返回 ≤ maxTotalPage），本例锁的是纯函数自身的边界
		// 安全性，生产路径由 TestPerfBudget 与装配层测试覆盖。
		results := make([]rawResult, maxTotalPage+1)
		got := assembleCapacity([]byte("[]"), results, maxTotalPage+1)
		if got < 0 {
			t.Fatalf("assembleCapacity 返回负数 = %d，钳制逻辑异常", got)
		}
	})
}

// TestAssembleCirclesJSON_UsesCapacityHelper 锁定 assembleCirclesJSON 确实经
// assembleCapacity 取预分配容量。
//
// 为什么需要它：容量计算被提取为纯函数后，「两道闸生效」由对该函数的断言
// 守护，但那只锁住了 helper 本身。若生产调用点改回内联的无闸逻辑、helper
// 仍在（变异实测：两道闸全删而 pkg/client 全包 26s 全绿），所有断言依然
// 通过——提取式重构的典型副作用：断言锁住 helper，没锁住调用关系。
//
// 判据是函数体内的调用存在性，与本仓 sentinel_coverage_guard_test.go
// 「两侧 AST 提取、无人工登记表可失配」同一形态。正向断言同时要求扫到本
// 文件与命中该调用，避免解析失败或判据形态写错时静默全绿。
func TestAssembleCirclesJSON_UsesCapacityHelper(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "raw_json.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 raw_json.go 失败: %v", err)
	}

	var callerFound, helperCalled bool
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Name.Name == "assembleCirclesJSON" {
			callerFound = true
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "assembleCapacity" {
					helperCalled = true
				}
				return true
			})
		}
	}

	if !callerFound {
		t.Fatal("未在 raw_json.go 找到 assembleCirclesJSON，判据已失效（改名或移文件？）")
	}
	if !helperCalled {
		t.Error("assembleCirclesJSON 未调用 assembleCapacity —— 预分配容量绕过了两道闸的直接实现，" +
			"调用点的断言将与之脱钩。容量闸实现已内联回调用点或改走其它路径。")
	}
}

// capAssembledSlice 对多页累积原始字节做总量预算判定——首页 4MB +
// 每页 4MB 连续多页累积越过 64MB 预算即返回 true（防攻陷服务端报
// 10000 页×4MB≈40GB 渐进填充进程内累积）。
func TestCapAssembledSlice_BudgetExceeded(t *testing.T) {
	// 首页 1 页满 4MB
	raw1 := bytes.Repeat([]byte("x"), maxResponseBodySize)
	// 20 页每页 4MB → 累积 80MB > 64MB 预算
	results := make([]rawResult, 21)
	for i := 2; i <= 20; i++ {
		results[i] = rawResult{raw: bytes.Repeat([]byte("y"), maxResponseBodySize)}
	}
	if !capAssembledSlice(raw1, results, 20) {
		t.Fatalf("累积 80MB 应越过 64MB 预算（预算闸未生效）")
	}
	if got := cumulativeSliceBytes(raw1, results, 20); got != 20*maxResponseBodySize {
		t.Fatalf("cumulativeSliceBytes = %d, want %d", got, 20*maxResponseBodySize)
	}
	// 小量翻页不误伤：仅首页 + 2 页小数据
	small := make([]rawResult, 3)
	small[2] = rawResult{raw: []byte(`[{"id":1}]`)}
	if capAssembledSlice(raw1, small, 2) {
		t.Fatal("小量累积不应被误判超预算")
	}
	// 页号越界不 panic
	if capAssembledSlice(raw1, small, 100) {
		t.Fatal("越界页号应安全返回（不越界访问 results）")
	}
}

// TestEstimatePagesBudgeted_Clamped 只验证 estimatePagesBudgeted 这个纯算术
// 函数的三个边界形态，**不覆盖**编排路径「getCirclesJSON 命中累积预算后是否
// 真的把钳制页数传给 assembleCirclesJSON」。
//
// 原注释声称「白盒直接验证 getCirclesJSON 走预算分支时 assembleCirclesJSON
// 拿到的页数 ≤ 预算页数」，而函数体只调了纯算术函数，从未触及那两个函数——
// 属悬空承诺（注释说测了、实际没测）。编排层的容量闸现由
// TestAssembleCirclesJSON_CapHintClamped 经 assembleCapacity 断言，两条各司
// 其职。
func TestEstimatePagesBudgeted_Clamped(t *testing.T) {
	// 首页 4MB、500 页 → 估算 2GB，越 64MB 预算
	got := estimatePagesBudgeted(500, maxResponseBodySize)
	if got <= maxAssembleBuffer {
		t.Fatalf("estimatePagesBudgeted(500, 4MB) = %d, want > 64MB", got)
	}
	// 首页 4KB、20 页 → 80KB，不越预算
	if got := estimatePagesBudgeted(20, 4096); got != 81920 {
		t.Fatalf("estimatePagesBudgeted(20, 4096) = %d, want 81920", got)
	}
	// 边界页 0 安全
	if got := estimatePagesBudgeted(0, 100); got != 0 {
		t.Fatalf("estimatePagesBudgeted(0,100) = %d, want 0", got)
	}
}
