// Package recoverx 提供 panic recover 统一工具。
//
// 两条 panic-recover 路径（cmd/nazhi/main.go 与 pkg/client/task.go）此前各自
// 手写 os.Stderr.Write(debug.Stack()) + fmt.Errorf(...)。RecoverPanic 收敛为
// 单点，确保 debug.Stack() 策略在所有 recover 路径一致。
//
// 输出目标固定为 os.Stderr，不提供注入缝：唯一可能的调用方是同包测试，而
// 测试经 os.Pipe 猴补 os.Stderr 即可断言，无需生产代码为此背负一个全局单值。
// 此处曾有过 SetPanicWriter 注入缝，全仓零调用点，且其「传 nil 恢复默认」
// 分支会存入动态值 nil 的 interface（类型断言成功但接收者为 nil），使此后
// panic 摘要与 stack 静默丢失——已连同该分支一并删除。
package recoverx

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync/atomic"
)

// quiet 包级静默标志：--quiet 模式下由 main 在 cobra PersistentPreRun
// 阶段（quiet flag 已解析）调用 SetQuiet(true)。pkg/client 的 recover 路径
// （fetchTasksForDimensionSafe）无法感知命令行 flag，统一经此包级入口获得
// quiet 语义。这是真实存在的跨层依赖，保留。
var quiet atomic.Bool

// SetQuiet 设置包级静默标志。--quiet 模式下 RecoverPanic 不再把
// debug.Stack() 写到 stderr（--quiet 承诺「关闭所有 stderr 输出」）。
func SetQuiet(q bool) { quiet.Store(q) }

// RecoverPanic 处理 recover() 拿到的值 r：
//   - r == nil：返回 nil（不做任何事，让正常流程继续）
//   - r != nil：非静默时输出 goroutine stack trace 到 stderr；
//     静默时只输出一行稳定摘要（含 name 与 panic 值）。然后用 sentinel +
//     name 包装为 error
//
// sentinel 为 nil 时跳过 %w 直接构造 message。
func RecoverPanic(r any, sentinel error, name string) error {
	if r == nil {
		return nil
	}
	if quiet.Load() {
		// --quiet 契约：不写 debug.Stack()，只留一行可归因的摘要
		fmt.Fprintf(os.Stderr, "panic: %s: %v\n", name, r)
	} else {
		_, _ = os.Stderr.Write(debug.Stack())
	}
	if sentinel != nil {
		return fmt.Errorf("%s: %w: %v", name, sentinel, r)
	}
	return fmt.Errorf("%s: %v", name, r)
}
