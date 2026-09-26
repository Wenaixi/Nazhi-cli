package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
)

// 命令行输出通道。
//
// 「stdout 只承载成功数据、错误一律写 stderr」是一条用户可见契约，全 CLI
// 二十余个生产文件、一百六十余处调用点依赖它。此前该模块的唯一注入点是
// 进程全局 os.Stdout / os.Stderr，测试要观察输出必须猴补全局变量绕过后门
// 注入——最该被测的行为被放到了测试面之外，且猴补全局本身就是进程级竞争
// 类别（当前只因无人写 t.Parallel 而未被 -race 报出）。
//
// outputSink 把通道持有为值：生产用绑定 os.Stdout / os.Stderr 的默认实例，
// 测试用绑定 bytes.Buffer 的实例。两个 adapter 此前就已在事实上存在
// （captureStdio 与 os.Pipe 夹具），这里只是让它们走同一个 interface。
//
// 出口归属：printEnvelope 走 out，其余（错误信封、verbose、交互提示、配置告警）
// 走 err。
//
// quiet 不在此持有：它是 --quiet 命令配置而非通道属性，由包级 quiet 变量
// 单点持有，避免出现「改了 sink.quiet 却对 printEnvelope 无效」的两套真相。
type outputSink struct {
	out io.Writer
	err io.Writer
}

// processOutputSink 是生产路径使用的实例：每次写入都现取当前的
// os.Stdout / os.Stderr，而不是在包级初始化时把指针捕获下来。
//
// 现取而非缓存是有原因的：既有测试夹具 captureStdio 通过重新赋值
// os.Stderr / os.Stdout 捕获输出，若这里缓存初始化时的指针，猴补将不生效、
// 全部通道断言读到空串。测试若要注入缓冲，走 newOutputSink 显式构造。
func processOutputSink() *outputSink {
	return &outputSink{out: os.Stdout, err: os.Stderr}
}

// newOutputSink 构造绑定指定通道的输出目标，供测试注入缓冲。
func newOutputSink(out, err io.Writer) *outputSink {
	return &outputSink{out: out, err: err}
}

// writeOut 把信封序列化到成功通道（缩进两格，与既有格式一致）。
func (s *outputSink) writeOut(e *envelope.Envelope) error {
	enc := json.NewEncoder(s.out)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}

// writeErrJSON 把错误信封序列化到错误通道。
func (s *outputSink) writeErrJSON(e *envelope.Envelope) error {
	enc := json.NewEncoder(s.err)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}

// writeErrLine 把纯文本行写入错误通道，供 verbose、交互提示与配置告警复用。
// quiet 守卫由调用方持有（它是命令配置，与通道无关）。
func (s *outputSink) writeErrLine(text string) {
	_, _ = fmt.Fprint(s.err, text)
}
