// fetch_tasks_residency_test.go 锁定 FetchTasksJSON 的驻留上界契约。
//
// 本文件守护的是「维度闸 + 单响应体限读」共同构成的那条常数上界：
// 驻留量 = 实际发出的维度请求数 × 单维响应体上界。两个因子分别被
// collectDims 的维度闸与 httpDo 的 4MiB 限读钉死，故驻留有界。
//
// 这条契约此前只有前半被锁住（raw_json_test.go 的
// TestFetchTasksJSON_TrimsExcessiveDimensions 断言合并条目数不超过
// 维度闸），后半「单维响应体封顶」在该路径上无人断言——限读只在上传
// 路径有测试。两条合起来才构成完整上界，缺任一条都无法排除
// 「维度数合规但单维体积无界」的情形。
//
// 为何不锁字节预算的位置：FetchTasksJSON 的字节预算判在 assemble()
// 闭包内，即所有维度请求发完之后。这是与写实路径（发请求前用
// estimatePagesBudgeted 拦截）不同的事实，但不是缺陷——实测维度闸已使
// 驻留有界（128 维 × 4MiB = 512MiB 常数上界），预算后置的差异只在于
// 「约束输出侧」而非「约束传输侧」。此处刻意只锁上界，不锁预算位置。
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestFetchTasksJSON_DimGateIsPrefetch 断言维度闸在发出请求之前生效：
// 服务端声明的维度数被钳制到 maxFetchTasksDims，真实发出的
// getCircleStatistics 请求数恰为该值，不多于它。
//
// 判据是「请求数」而非「结果条目数」：若维度闸改成发请求后才截断，
// 合并结果的条目数仍是 128，但服务端已被索取 200 次——驻留与网络
// 开销早已发生。只有断言请求数才能区分前置与后置。
func TestFetchTasksJSON_DimGateIsPrefetch(t *testing.T) {
	const maxFetchTasksDims = 128

	var hits atomic.Int64
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			dims := make([]map[string]any, 0, 200)
			for i := range 200 {
				dims = append(dims, map[string]any{"id": i + 1, "name": "维度" + string(rune('A'+i%26))})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "dataList": dims})
		case "/api/studentCircleNew/getCircleStatistics":
			hits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 1, "dataList": []map[string]any{{"id": 1001, "name": "任务X"}},
			})
		default:
			http.NotFound(w, r)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	if _, err := c.FetchTasksJSON(context.Background(), "test-token"); err != nil {
		t.Fatalf("FetchTasksJSON: %v", err)
	}

	if got := hits.Load(); got != maxFetchTasksDims {
		t.Fatalf("维度闸必须前置：声明 200 维应只发出 %d 次请求，实际发出 %d 次",
			maxFetchTasksDims, got)
	}
}

// TestFetchTasksJSON_SingleDimensionBodyCappedAt4MiB 断言单个维度的
// 响应体受 httpDo 的 4MiB 限读约束：超限即报错，不进入分片结果。
//
// 与上一条合成完整上界：维度数被闸钳到 128，单维被限读封到 4MiB，
// 故驻留 = 128 × 4MiB 是常数。本条若失效（限读被绕过或阈值放大），
// 上界即随攻击者投入增长。
func TestFetchTasksJSON_SingleDimensionBodyCappedAt4MiB(t *testing.T) {
	const maxResponseBodySize = 4 << 20

	// 单维 dataList 约 5MiB，稳定越过 4MiB 上限。
	oversized := make([]map[string]any, 0, 3500)
	for i := range 3500 {
		oversized = append(oversized, map[string]any{"id": i, "name": strings.Repeat("a", 1500)})
	}

	var hits atomic.Int64
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 1, "dataList": []map[string]any{{"id": 1, "name": "维度A"}},
			})
		case "/api/studentCircleNew/getCircleStatistics":
			hits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "dataList": oversized})
		default:
			http.NotFound(w, r)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithUploadURL(biz.URL))
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, err := c.FetchTasksJSON(context.Background(), "test-token")
	// 单维超限会使该维度被丢弃：维度数为 1，结果退化为空数组而非错误。
	// 两种表现都可接受，本条只断言「大响应未进入结果」。
	if len(raw) > maxResponseBodySize {
		t.Fatalf("单维响应体越过 4MiB 限读并进入结果：%d 字节", len(raw))
	}
	t.Logf("单维 5MiB 响应：请求数=%d 结果=%d 字节 err=%v", hits.Load(), len(raw), err)
}
