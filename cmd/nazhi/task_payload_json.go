package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

var taskInputNumericStringFields = [...]string{
	"hours",
	"level",
	"checkResult",
	"playRole",
}

// taskInputDeprecatedFields 是前端表单有键但用户从不手填的历史兼容字段。
// 放行它们是为了不把「旧调用方还在传」误判为「未知键」，否则会误伤历史
// payload——这一点与字段是否被提交链路读取无关。
//
// 注意：这七个键并非「SDK 不消费」。buildTaskPayload 会读取其中六个并
// 原样带上 Name/HostName/CircleDate/Rank/Level/TermName 出站（空值不覆盖、
// 不发明默认值）；只有 id 由编辑路径单独处理，不经 ActivityFields。
// 前端 form 对照实证：practice 表单 JSON.stringify 恒含 id/""、name/""、hostName/""、
// circleDate/""、rank/""、level/""、termName/""；art 表单含 name；edit 恒注入 id。
var taskInputDeprecatedKeys = newPayloadKeySet(
	"id", "name", "hostName", "circleDate", "rank", "level", "termName",
)

// taskInputAllowedKeys 是 task submit/edit payload 顶层 JSON 的全部允许键：
// TaskInput 消费的 json 键 + 别名对（circleTaskId/pictureList）+ 历史兼容字段。
// 清晰列出允许集，未知键以参数错误拒绝（对齐 user update）。
// 键名按用户书写形态声明（与 taskPayloadInput 的 json 键逐字一致），
// 小写索引由 newPayloadKeySet 派生——不手工维护第二份，避免两处脱节。
var taskInputKeys = newPayloadKeySet(
	// TaskInput 消费的普通字段（TaskAddCirclePayload 出站 json 键全集）
	"id", "name", "hostName", "circleDate", "rank", "level", "content",
	"pictureList", "circleTaskId", "circleTypeId", "dimensionId", "hours",
	"circleBeginDate", "circleEndDate", "checkResult", "patentType", "patentNum",
	"address", "termName", "activityName", "sportsName", "teamName", "orgName",
	"resultsName", "obtainTime", "specialtyTechnology", "playRole",
	"likeSpecialty1", "likeSpecialty2", "likeSpecialty3",
	// TaskInput 接口消费但非出站 json 键的输入字段
	"taskId", "imagePaths", "imageIds",
)

// taskInputKeysAll 叠加历史兼容字段：这两类键用户都可能传入，合并在一处
// 派生，避免维护两份需要手工同步的列表。
var taskInputKeysAll = merged(taskInputKeys, taskInputDeprecatedKeys)

// taskInputAllowedKeys 是 task 族的比较用小写允许集。
var taskInputAllowedKeys = taskInputKeysAll.allowed()

var taskInputFieldAliases = [...]struct {
	canonical string
	alias     string
}{
	{canonical: "taskId", alias: "circleTaskId"},
	{canonical: "imageIDs", alias: "pictureList"},
}

// decodeTaskInputJSON 解码 CLI 写实 payload，并兼容前端编辑回填的数字字段与提交字段别名。
// 归一化只属于 CLI 输入边界；SDK 公开的 Task*Input 仍保持普通 Go 字段语义。
func decodeTaskInputJSON(data []byte, target any) error {
	normalized, err := normalizeTaskInputJSON(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, target)
}

func normalizeTaskInputJSON(data []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}

	for _, field := range taskInputFieldAliases {
		if raw, ok := findTaskInputField(fields, field.canonical); ok {
			setTaskInputField(fields, field.canonical, raw)
			continue
		}
		if raw, ok := findTaskInputField(fields, field.alias); ok {
			setTaskInputField(fields, field.canonical, raw)
		}
	}

	for _, name := range taskInputNumericStringFields {
		raw, ok := findTaskInputField(fields, name)
		if !ok {
			continue
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
			continue
		}
		var number json.Number
		if err := json.Unmarshal(raw, &number); err != nil {
			return nil, fmt.Errorf("%s: 期望字符串或数字: %w", name, err)
		}
		if name != "hours" {
			value, err := normalizeTaskInputNumericCode(number)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			number = json.Number(value)
		}
		encoded, err := json.Marshal(number.String())
		if err != nil {
			return nil, fmt.Errorf("%s: 数字转字符串失败: %w", name, err)
		}
		setTaskInputField(fields, name, encoded)
	}

	return json.Marshal(fields)
}

// normalizeTaskInputNumericCode 把「数字代码必须是有限整数」判定收为具名函数，
// 并显式声明与 pkg/types/flexnum 的合法集合差异。
//
// CLI --payload 是用户输入边界：level/checkResult/playRole 等数字代码字段
// 接受任意大整数（big.Rat 判定），转成字符串码后由服务端按字符串语义消费，
// 大数转字符串无害。flexnum（SDK 结构化解码路径）则拒绝 2^63 以上——它防的
// 是 int64 溢出回绕，两类出口的约束不同，**差异是有意设计**，两处各自演进。
//
// 若未来发现平台对超大数字代码的真实行为，应统一两处口径并删除本差异声明。
func normalizeTaskInputNumericCode(number json.Number) (string, error) {
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", fmt.Errorf("数字代码必须是有限整数")
	}
	integer, ok := new(big.Rat).SetString(number.String())
	if !ok || !integer.IsInt() {
		return "", fmt.Errorf("数字代码必须是有限整数")
	}
	return integer.Num().String(), nil
}

func findTaskInputField(fields map[string]json.RawMessage, name string) (json.RawMessage, bool) {
	if raw, ok := fields[name]; ok {
		return raw, true
	}
	keys := make([]string, 0, 1)
	for key := range fields {
		if strings.EqualFold(key, name) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil, false
	}
	// 对大小写变体保持确定性；精确 canonical key 已在上方优先处理。
	sort.Strings(keys)
	return fields[keys[0]], true
}

func setTaskInputField(fields map[string]json.RawMessage, name string, value json.RawMessage) {
	for key := range fields {
		if strings.EqualFold(key, name) {
			delete(fields, key)
		}
	}
	fields[name] = value
}

func decodeTaskSubmitInput(data []byte) (types.TaskSubmitInput, error) {
	var input types.TaskSubmitInput
	if err := decodeTaskInputJSON(data, &input); err != nil {
		return input, err
	}
	return input, nil
}

func decodeTaskEditInput(data []byte) (types.TaskEditInput, error) {
	var input types.TaskEditInput
	if err := decodeTaskInputJSON(data, &input); err != nil {
		return input, err
	}
	return input, nil
}
