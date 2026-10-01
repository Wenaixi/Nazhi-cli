package client

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 端到端验证 httpDo 的响应体日志出口不泄漏「响应体自身在敏感值中间被截断」
// 这一形态的明文。
//
// 背景：该出口曾写 logx.RedactBodyThenTruncate(respBytes, 100)，自带字面量
// 长度且跳过 clipPrefixWindow。当响应体在敏感值中间被切断时（服务端/反向代
// 理超时是常见来源），脱敏正则因缺闭合引号整体失配，明文进入日志。修复是改
// 走 RedactSnippet，由它内部收口该形态。
//
// 为什么用真实 HTTP 往返而非直接调 logx：判据是「日志里有没有明文」，而日志
// 内容取决于整条链路的真实形态（状态码 → levelForStatus → logEnabled 判定
// → 摘要生成）。直接调 logx 只能证伪函数的一半。
func TestHTTPDo_TruncatedSensitiveValueNotLeaked(t *testing.T) {
	const secret = "SUPERSECRETTOKENVALUE1234567890"

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{
			// 500 走 levelForStatus→Error，默认 warn 级别下必定输出，
			// 这是默认配置下最易命中的形态。
			name:   "500 且 token 值被截断",
			status: http.StatusInternalServerError,
			body:   `{"code":500,"token":"` + secret,
		},
		{
			name:   "401 且 authorization 值被截断",
			status: http.StatusUnauthorized,
			body:   `{"code":401,"authorization":"Bearer ` + secret,
		},
		{
			// 2xx 走 Info，默认 warn 级别下被过滤；显式开 debug 才输出。
			// 保留它是为了覆盖「启用日志后 2xx 路径同样受保护」。
			name:   "200 且 password 值被截断",
			status: http.StatusOK,
			body:   `{"code":1,"password":"` + secret,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			var logBuf bytes.Buffer
			// 用仓库既有的构造器（benchClient）补齐 http/sm 等字段，
			// 只覆盖 logger 以捕获日志输出。
			c := benchClient(t, srv.URL)
			c.logger = slog.New(slog.NewTextHandler(&logBuf,
				&slog.HandlerOptions{Level: slog.LevelDebug}))

			_, _ = c.httpDo(context.Background(), http.MethodGet,
				srv.URL+"/api/probe", nil, nil, "")

			got := logBuf.String()
			if strings.Contains(got, secret) {
				t.Fatalf("日志泄漏了被截断的敏感值明文:\n  secret=%s\n  日志=%s",
					secret, got)
			}
		})
	}
}
