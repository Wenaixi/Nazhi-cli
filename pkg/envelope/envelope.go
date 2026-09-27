// Package envelope 提供 CLI 统一响应 envelope。
//
// 设计动机：CLI 所有命令输出统一 JSON envelope，方便脚本 parse。
//   - Status 字段：success / partial / error 三态
//   - Code 字段：HTTP 风格状态码（200/4xx/5xx）
//   - Message 字段：人类可读提示
//   - Data 字段：业务负载（任意类型）
//
// 双层 code 对照（易混淆提醒）：
//
// 本包的 envelope.code 是 CLI 外层 HTTP 风格码，业务成功固定为 200；
// 平台原始业务响应为 pkg/types.UnifiedResponse.code，业务成功固定为 1。
// 两层 code 同名不同层，切勿混用 jq .code 判成功。
//
//	| 层次 | 类型 | 成功值 | 失败值 | 位置 |
//	|------|------|--------|--------|------|
//	| CLI 信封 | envelope.code | 200 | 4xx/5xx | 本包 Envelope.Code |
//	| 业务响应 | UnifiedResponse.code | 1 | 非 1 | pkg/types.UnifiedResponse.Code |
//
// 对照：UnifiedResponse.code==1 等价 envelope.code==200。
// 脚本判成功请以 envelope.status=="success" 或 envelope.code==200 为准，
// 不要直接用业务层的 jq .code==1 逻辑去判断外层信封。
//
// 退出码三分契约（见 ExitCode 方法）：
//   - 0: 成功
//   - 1: partial / 业务错误 (4xx 非 400)
//   - 2: 网络/服务端错误 (5xx)
//   - 3: 参数错误 (400)
package envelope

// Status 是 envelope 的状态字段。
type Status string

const (
	// StatusSuccess 表示完全成功。
	StatusSuccess Status = "success"
	// StatusPartial 表示部分成功（有数据但也有错误）。
	StatusPartial Status = "partial"
	// StatusError 表示完全失败。
	StatusError Status = "error"
)

// Envelope 是 CLI 统一的响应结构。
type Envelope struct {
	Status  Status `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// ExitCode 返回 CLI 退出码。
//
// 三分契约：
//   - 0  成功（status=success）
//   - 1  部分成功 或 业务错误 (4xx 非 400)
//   - 2  网络/服务端错误 (5xx)
//   - 3  参数错误 (400)
func (e *Envelope) ExitCode() int {
	switch e.Status {
	case StatusSuccess:
		return 0
	case StatusPartial:
		return 1
	case StatusError:
		switch {
		case e.Code == 400:
			return 3
		case e.Code >= 500:
			return 2
		case e.Code >= 400 && e.Code < 500:
			return 1
		}
	}
	// 兜底：未知状态视为失败
	return 1
}

// Success 构造成功 envelope。
func Success(data any) *Envelope {
	return &Envelope{Status: StatusSuccess, Code: 200, Data: data}
}

// Empty 构造空数据 envelope（HTTP 204 风格）。
func Empty(msg string) *Envelope {
	return &Envelope{Status: StatusSuccess, Code: 204, Message: msg, Data: nil}
}

// Partial 构造部分成功 envelope。
//
// Deprecated: 语义不明确（code 由调用方任意传）。列表部分页失败请用
// PartialData（code 恒 207）；session 冷却请用 Pulse（code 恒 429）。
// 保留仅为兼容旧调用方。
func Partial(code int, msg string, data any) *Envelope {
	return &Envelope{Status: StatusPartial, Code: code, Message: msg, Data: data}
}

// PartialData 构造「列表部分页失败」envelope（HTTP 207）。
//
// 部分完成在 CLI 只有一种业务形态：列表/任务取数时已拿到部分数据但后续
// 页失败。code 恒 207 由模块单点持有，调用方不再手抄字面量；
// ExitCode 对 StatusPartial 恒返回 1。
func PartialData(msg string, data any) *Envelope {
	return &Envelope{Status: StatusPartial, Code: 207, Message: msg, Data: data}
}

// Pulse 构造「会话冷却」envelope（HTTP 429）。
//
// 与列表部分失败语义不同：session 激活被 backoff 抑制不是「部分完成」，
// 而是「有状态但暂不可用」。单独构造器让两种语义不再共用外观相同的
// Partial 调用；ExitCode 仍为 1（StatusPartial 恒 1）。
func Pulse(msg string) *Envelope {
	return &Envelope{Status: StatusPartial, Code: 429, Message: msg, Data: nil}
}

// Error 构造错误 envelope。
func Error(code int, msg string) *Envelope {
	return &Envelope{Status: StatusError, Code: code, Message: msg, Data: nil}
}
