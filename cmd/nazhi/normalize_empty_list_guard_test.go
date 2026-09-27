package main

import (
	"encoding/json"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
)

// TestNormalizeEmptyList_RawMessageNotTreatedAsRecordList 锁定反射判据的载荷边界。
//
// normalizeEmptyList 的职责是「nil 记录切片归一为空数组」，但 json.RawMessage
// 的底层类型也是 []byte，反射的 Kind 判据无法区分「记录列表」与「不透传的
// 原始 JSON 字节」。后果是：nil RawMessage 被换成非 nil 的零长 RawMessage，
// 它既不再是 nil（调用点的 == nil 判据失效），又不是合法 JSON（MarshalJSON
// 返回空字节，编码器报 unexpected end of JSON input）。
//
// 正确判据是问「这是不是记录列表」，而不是「这是不是 nil 切片」。
func TestNormalizeEmptyList_RawMessageNotTreatedAsRecordList(t *testing.T) {
	var nilRaw json.RawMessage

	got := normalizeEmptyList(nilRaw)

	raw, ok := got.(json.RawMessage)
	if !ok {
		t.Fatalf("载荷类型应保持 json.RawMessage，实际 %T", got)
	}
	if raw == nil {
		return // 判据正确：原样返回 nil，由 success 闭包按载荷语义自行决定空形态
	}
	t.Errorf("nil json.RawMessage 被归一成非 nil 零长值 %v（len=%d），"+
		"既非 nil 又非合法 JSON，封进信封后序列化必然失败", got, len(raw))
}

// TestNormalizeEmptyList_NilRawMessageEnvelopeSerializes 端到端确认终局后果：
// 载荷若被归一成零长 RawMessage，信封序列化失败且 stdout 无任何输出。
func TestNormalizeEmptyList_NilRawMessageEnvelopeSerializes(t *testing.T) {
	var nilRaw json.RawMessage

	env := envelope.Success(normalizeEmptyList(nilRaw))
	if _, err := json.Marshal(env); err != nil {
		t.Errorf("nil RawMessage 归一后信封不可序列化: %v", err)
	}
}

// TestNormalizeEmptyList_RecordListsStillNormalized 确认修判据不回归原有职责：
// 具名切片的 nil 仍须归一为空数组，否则输出 "data":null。
func TestNormalizeEmptyList_RecordListsStillNormalized(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"无类型 map 列表", []map[string]any(nil)},
		{"any 列表", []any(nil)},
		{"具名切片", []json.RawMessage(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeEmptyList(tt.input)
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("归一结果不可序列化: %v", err)
			}
			if string(b) == "null" {
				t.Errorf("归一结果序列化为 null，应为空数组：%s", b)
			}
		})
	}
}
