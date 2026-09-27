package types

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SelfEvalStatus 是自我评价状态。
//
// 前端仅读 student_comment（mainLeft.vue:90/:132、selfgaintloss.vue:107）；teacher_comment
// 为平台响应字段（HAR 可见），SDK 建模备用，无前端读取点（修正注释）。
// 部分 mock / returnData 为 camelCase。Unmarshal 双键兼容；Marshal 输出 camelCase
// （与提交 addSelfEvaluation 的 studentComment 请求键一致）。
type SelfEvalStatus struct {
	ID             int64  `json:"id"`
	StudentComment string `json:"studentComment"`
	TeacherComment string `json:"teacherComment"`
}

// UnmarshalJSON 兼容 student_comment / studentComment 与 teacher_comment / teacherComment。
func (s *SelfEvalStatus) UnmarshalJSON(data []byte) error {
	if s == nil {
		return fmt.Errorf("SelfEvalStatus: UnmarshalJSON on nil pointer")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["id"]; ok {
		if err := json.Unmarshal(v, &s.ID); err != nil {
			return fmt.Errorf("SelfEvalStatus.id: %w", err)
		}
	}
	// snake 主读（平台 dataMap 真实形态），camel 兼容。
	//
	// 注意：本方法与 client.normalizeSelfEvalStatus 是两套独立实现，判据
	// 并不一致——id 在此直接 Unmarshal 到 int64（非法值报错误、中断整条解码），
	// 在 map 路径则经 NormalizeIntegerValue 归零后继续解析评语；空白串在此
	// 原样保留，在 map 路径被 TrimSpace 后视为空。二者曾互相声明「口径一致」，
	// 该断言不成立，已删除。
	//
	// 当前不构成用户可见差异：本方法在产品代码中唯一挂载点是 dataList 容器的
	// DecodeDataList，而该处的解码错误被 client/self_eval.go 的条件吞掉、降级
	// 到 map 宽松路径。returnData 与 dataMap 容器不走本方法。
	studentComment, present, err := firstJSONString(raw, "student_comment", "studentComment")
	if err != nil {
		return fmt.Errorf("SelfEvalStatus.studentComment: %w", err)
	}
	if present {
		s.StudentComment = studentComment
	}
	teacherComment, present, err := firstJSONString(raw, "teacher_comment", "teacherComment")
	if err != nil {
		return fmt.Errorf("SelfEvalStatus.teacherComment: %w", err)
	}
	if present {
		s.TeacherComment = teacherComment
	}
	return nil
}

func firstJSONString(raw map[string]json.RawMessage, keys ...string) (string, bool, error) {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		trimmed := bytes.TrimSpace(v)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			continue
		}
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return "", true, err
		}
		return s, true, nil
	}
	return "", false, nil
}
