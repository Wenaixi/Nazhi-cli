package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDoBizAndDecode_BadJSONHasInvalidResponseSentinel 回归测试：
// 200 + 非 JSON body（nginx 维护页 / WAF 挑战页）的解码失败必须携带
// ErrInvalidResponse 哨兵。
//
// 背景：非 2xx 分支经 classifyHTTPStatus 恒有哨兵，
// 唯独「服务端异常但状态码说谎」的 200+HTML 路径走裸 fmt.Errorf 包装，
// SDK 用户按文档推荐的 errors.Is(err, ErrInvalidResponse) 判定落空——
// 同一台服务器 502+HTML 有哨兵而 200+HTML 没有，分类体系在此失效。
func TestDoBizAndDecode_BadJSONHasInvalidResponseSentinel(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>维护中</html>"))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithSSOBase(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("期望解析错误，但得到 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("200+非 JSON 解码失败应包装 ErrInvalidResponse，实际: %v", err)
	}
}

// TestDecodeResponseSingleScaffold 锁定「业务响应解码」只有一处实现。
//
// 背景：doBizAndDecode 曾内联一句与 decodeOrInvalidResponse 逐字相同的
// fmt.Errorf，两处包裹的 error 同来自 types.DecodeResponse，哨兵与 opName
// 也同参。当时保留内联的理由是「日志上下文就地可读」，但 helper 本身就收
// opName，该理由不成立——重复是唯一遗留理由。
//
// 本守卫按骨架计数而非按文案匹配：文案改写无需重新裁决，但「新增一处
// types.DecodeResponse 裸调用」必须被挡住，因为裸调用会绕过
// ErrInvalidResponse 哨兵，CLI 漏斗随之把「200 但响应体不可信」判成
// default 500/exit2，而 httpDo/doBizGet 的同形态走 502/exit2。
func TestDecodeResponseSingleScaffold(t *testing.T) {
	data, err := os.ReadFile("request.go")
	if err != nil {
		t.Fatalf("读取 request.go 失败: %v", err)
	}
	var sites []string
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "types.DecodeResponse(") {
			sites = append(sites, "第"+strconv.Itoa(i+1)+"行")
		}
	}
	if len(sites) != 1 {
		t.Fatalf("types.DecodeResponse( 应只在 decodeOrInvalidResponse 内出现一次，实际出现 %d 次（%s）。\n"+
			"新增裸调用会绕过 ErrInvalidResponse 哨兵，使 CLI 把「200 但响应体不可信」\n"+
			"判成 default 500/exit2，与 httpDo/doBizGet 的 502/exit2 不一致。",
			len(sites), strings.Join(sites, "、"))
	}
}
