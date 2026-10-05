package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// TestFetchTasks_DimsClampCountsRealDimensions 锁定维度闸按「过滤汇总维度
// 之后的真实维度数」计数，而不是按原始列表长度。
//
// 平台形态：getDimensions 返回的列表首元素是 id==0 的汇总维度，collectDims
// 会跳过它。因此调用点若先在原始列表上截断，汇总维度恰在前 128 内时会提前
// 丢掉一个真实维度——实测 201 维（含首元素汇总）时取到 127 而非 128。
//
// 既有 TestFetchTasks_DimsClamped 的夹具从 ID=1 开始、不含汇总维度，两种
// 计数方式结果相同（都是 128），因此它无法证伪这一形态。本测试用生产同形
// 的夹具补上这个缺口。
func TestFetchTasks_DimsClampCountsRealDimensions(t *testing.T) {
	const realDims = 129 // 超过 maxFetchTasksDims=128 的真实维度数

	dims := make([]types.Dimension, 0, realDims+1)
	// 首元素为汇总维度：平台真实形态，且它占用原始列表的一个位置
	dims = append(dims, types.Dimension{ID: 0, Name: "汇总"})
	for i := 1; i <= realDims; i++ {
		dims = append(dims, types.Dimension{ID: int64(i), Name: "dim"})
	}

	// atomic 而非普通 int：httptest handler 由 net/http 为每个请求起
	// goroutine，FetchTasks 又是多路并发拉取，普通 int 的 ++ 是数据竞争。
	var statCalls atomic.Int64
	var sawSummary atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			resp := types.UnifiedResponse{Code: 1}
			raw, _ := json.Marshal(dims)
			rawMsg := json.RawMessage(raw)
			resp.DataList = &rawMsg
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/studentCircleNew/getCircleStatistics":
			statCalls.Add(1)
			if r.URL.Query().Get("dimensionId") == "0" {
				sawSummary.Store(true)
			}
			resp := types.UnifiedResponse{Code: 1}
			raw, _ := json.Marshal([]any{})
			rawMsg := json.RawMessage(raw)
			resp.DataList = &rawMsg
			_ = json.NewEncoder(w).Encode(resp)
		case "/", "/api/studentInfo/getMenu":
			_ = json.NewEncoder(w).Encode(types.UnifiedResponse{Code: 1, Msg: ptr("ok")})
		case "/api/studentInfo/getMyInfo":
			raw := json.RawMessage(`{"id":1,"name":"t","studentNumber":"S1","schoolId":173,"schoolName":"本地测试学校"}`)
			_ = json.NewEncoder(w).Encode(types.UnifiedResponse{Code: 1, ReturnData: &raw})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.FetchTasks(t.Context(), "test-token"); err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}

	// 汇总维度不参与扇出
	if sawSummary.Load() {
		t.Error("id=0 的汇总维度不应发出 getCircleStatistics 请求")
	}
	// 闸按真实维度计数：129 个真实维度应截断到 128 个请求。
	// 若按原始列表（含汇总的 130 个）截断，则只会发 127 个真实维度的请求。
	if n := statCalls.Load(); n != maxFetchTasksDims {
		t.Errorf("闸应按过滤汇总维度后的真实维度数计数，发出 %d 次请求，期望 %d 次"+
			"（少发说明调用点按原始列表截断，汇总维度占用了一个名额）",
			n, maxFetchTasksDims)
	}
}
