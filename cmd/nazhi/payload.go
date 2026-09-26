package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

const maxPayloadSize = 16 << 20

// parsePayloadFromArg 解析命令行 payload 参数，支持 @file.json 和 -（stdin）语法。
// 这是 task_submit.go 和 honor.go 中公共的 payload 读取逻辑抽取。
func parsePayloadFromArg(ctx context.Context, raw string) ([]byte, error) {
	if strings.HasPrefix(raw, "@") {
		// 与 stdin 路径对齐：@file 也受 16 MiB 上限保护，防止误传大文件撑爆内存
		f, err := os.Open(raw[1:])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		payload, err := io.ReadAll(io.LimitReader(f, maxPayloadSize+1))
		if err != nil {
			return nil, err
		}
		if len(payload) > maxPayloadSize {
			return nil, fmt.Errorf("文件 payload 超过 16 MiB 上限")
		}
		return payload, nil
	}
	if raw == "-" {
		// 与 self-eval submit 的 stdin 保护对齐：交互终端下先给提示符，
		// 再走带超时的读取——避免手滑写 - 时无提示无超时地永久阻塞。
		// 继承调用方 cmd.Context 而非 context.Background，Ctrl+C 可中断。
		printPrompt("请输入 payload JSON（Ctrl+D 结束）: ")
		content, readErr := readStdinWithTimeout(ctx, 60)
		if readErr != nil {
			return nil, readErr
		}
		payload := []byte(content)
		if len(payload) > maxPayloadSize {
			return nil, fmt.Errorf("stdin payload 超过 16 MiB 上限")
		}
		return payload, nil
	}
	return []byte(raw), nil
}

// parseJSONObjectPayload 读取并校验对象型 JSON payload。
// 文件、stdin 和内联 JSON 的读取语义保持由 parsePayloadFromArg 负责。
func parseJSONObjectPayload(ctx context.Context, raw string) ([]byte, error) {
	payload, err := parsePayloadFromArg(ctx, raw)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("顶层 JSON 必须为对象")
	}
	return payload, nil
}

// PayloadPositiveIDValid 校验 update payload 携带正数 id。
// 整数判定口径委派 types.NormalizeIntegerValue（与 FlexInt / parseFlexInt
// / honorMapInt64 / firstInt64 共用同一实现，含 2^63 上界拒绝），
// 本函数只追加「必须为正数」这条业务判定。
//
// 2^63-1 判负的依据：float64(math.MaxInt64) 舍入后恰等于 2^63，
// 该字面量经 int64 转换会溢出为负值，必须拒绝。
func PayloadPositiveIDValid(payload map[string]any) bool {
	id, err := types.NormalizeIntegerValue(payload["id"])
	if err != nil {
		return false
	}
	return id > 0
}

// unknownUpdatePayloadKeys 返回 payload 顶层 JSON 中不在允许键集合内的键名（稳定排序）。
// 与 unknownUserUpdateKeys 同构；该收敛让 honor/typical-case update 与 user update
// 共享同一未知键拒绝语义。
// 允许集统一小写存储，用户键 ToLower 后比较——与 task 族的
// findTaskInputField EqualFold 解码语义对齐，
// 避免用户传 CERTIMGATTACHMENTID/Telephone 等大小写变体被误拒。
func unknownUpdatePayloadKeys(payloadBytes []byte, allowed map[string]struct{}) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(payloadBytes, &top); err != nil {
		return nil // 解析已在调用方完成并报错，此处不重复
	}
	var unknown []string
	for k := range top {
		if _, ok := allowed[strings.ToLower(k)]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return unknown
}
