package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// 典型案例审核状态是驱动服务端列表过滤的查询参数。传错值不会报错，
// 只会静默返回一份意料之外的记录集合——这正是写实列表类型被做成具名
// 类型并在发请求前 Valid 的同一类风险。四个常量此前是无类型 int
// 常规量，全仓没有任何调用点，CLI 的 --status 因此零校验直发。
//
// 本测试锁定判定发生在「发请求之前」：非法状态必须归 ErrInvalidPayload
// （映射 400 / 退出码 3），且一个业务请求都不许发出。
func TestGetTypicalCaseList_InvalidStatusRejectedBeforeRequest(t *testing.T) {
	// 命中业务端点即说明校验没拦住。计数器由 net/http 为每个请求起
	// goroutine 并发访问，必须用原子类型。
	var hit atomic.Int64
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
			hit.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null,"dataList":[],"dataMap":null}`))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	// 穷举非法区间而非逐个列举：只要将来往枚举里加值，测试仍能覆盖
	// 「枚举之外的一切」，不会因为漏列举某个具体数字而恒绿。
	for _, st := range []int{-99, -2, -1, 4, 5, 99, 1000} {
		_, err := c.GetTypicalCaseListJSON(context.Background(), "test-token-abc", 1, 10, st)
		if err == nil {
			t.Errorf("status=%d 应被拒绝，实际放行并发出请求", st)
			continue
		}
		if !errors.Is(err, client.ErrInvalidPayload) {
			t.Errorf("status=%d 应归 ErrInvalidPayload（映射 400/exit 3），实际 %v", st, err)
		}
	}

	if n := hit.Load(); n != 0 {
		t.Errorf("非法状态不应发出任何业务请求，实际发出 %d 次", n)
	}
}

// 合法值必须继续原样透传——校验不得改变既有行为。
func TestGetTypicalCaseList_ValidStatusPassedThrough(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status client.TypicalCaseStatus
	}{
		{"未审核", client.TypicalCaseStatusPending},
		{"通过", client.TypicalCaseStatusApproved},
		{"驳回", client.TypicalCaseStatusRejected},
		{"全部", client.TypicalCaseStatusAll},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got atomic.Value
			biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/studentCircleNew/getTypicalCase" {
					got.Store(r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null,"dataList":[],"dataMap":null}`))
			})))
			defer biz.Close()

			c := newTestClient(nil, biz, nil)
			if _, err := c.GetTypicalCaseListJSON(context.Background(), "test-token-abc", 1, 10, int(tc.status)); err != nil {
				t.Fatalf("合法 status=%d 不应报错: %v", tc.status, err)
			}
			q, _ := got.Load().(string)
			if q == "" {
				t.Fatalf("合法 status=%d 应发出请求", tc.status)
			}
			if want := "status=" + strconv.Itoa(int(tc.status)); !strings.Contains(q, want) {
				t.Errorf("查询串应含 %s，实际 %s", want, q)
			}
		})
	}
}
