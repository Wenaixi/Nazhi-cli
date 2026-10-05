package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// TestLogin_ValidateOversizedBody_ClosesWithoutDraining 锁定 Login validate
// 超限分支的处置纪律。
//
// 背景：其余限读出口（httpDo / doBizGet / 上传成功体）在命中上限时都
// 直 Close 放弃 keep-alive，唯独 Login validate 此前只是 return，让
// defer drainAndClose 继续 io.Copy 无上限读完剩余 body。实测无限流服务端
// ��� drain 续读 264MB 直到服务端 EOF——正是 httpDo 注释警告的
// 「恶意无限流下会拖到客户端超时才兜底」形态。
//
// 本测试用一个「限读上限 + 大量可读数据」的服务端，断言客户端在超限后
// **不再消费剩余 body**。注意：单纯断言返回 ErrLoginRejected 不够——
// 既有 TestLogin_ValidateOversizedBody_Rejects 已经断言了哨兵，而无论
// 是否 Close 都会得到同一个错误，恒真。
func TestLogin_ValidateOversizedBody_ClosesWithoutDraining(t *testing.T) {
	const p = "1_8_PROBE"
	_ = p

	// 服务端已写出字节数。客户端超限后若仍 drain，此值会逼近全量；
	// 若直 Close 放弃 keep-alive，则只到上限附近即停止。
	var written atomic.Int64
	release := make(chan struct{})

	bigToken := strings.Repeat("x", 8<<20) // 8MB，远超 4MiB 上限
	head := fmt.Sprintf(`{"code":1,"msg":"成功","returnData":{"token":"%s"`, bigToken)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uiActivityLogin/studentLogin" {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, head)
		written.Add(int64(len(head)))

		// 持续推送直到客户端断开或测试结束，用于观测客户端是否还在读。
		chunk := strings.Repeat("y", 64*1024)
		for {
			select {
			case <-release:
				return
			default:
			}
			n, err := io.WriteString(w, chunk)
			written.Add(int64(n))
			if err != nil {
				return // 客户端已关闭连接
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer func() {
		close(release)
		srv.Close()
	}()

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     nil,
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("超大响应体应报错，实际 nil")
	}
	if !errors.Is(err, ErrLoginRejected) {
		t.Errorf("超大响应体应归 ErrLoginRejected，实际 %v", err)
	}

	// 关键断言：客户端停止读取后，服务端写入量应远小于其想写的总量。
	// 给调度留出余量：判定阈值取「上限 + 8MB」，越过即说明仍在续读。
	// 无上限续读时服务端会一直写到测试超时（数十 MB 起）。
	closeThreshold := int64(maxResponseBodySize) + 8<<20
	if got := written.Load(); got > closeThreshold {
		t.Errorf("Login 超限后仍在无上限续读 body：服务端已写出 %d 字节，"+
			"阈值 %d（直 Close 时应远低于此值）", got, closeThreshold)
	}
}
