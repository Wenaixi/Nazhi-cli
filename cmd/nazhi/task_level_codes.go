package main

import (
	"sort"
	"strings"

	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/Nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// taskLevelCodesCmd 输出写实域「数字→中文名」的完整对照表。
//
// 纯本地查表：不联网、不需要 token、不发任何请求，因此不注册 --token 等
// 业务 flag。表内容取自 pkg/types 的三张对照表（唯一真相源），命令不另抄一份。
//
// 定位是离线速查：平台上写实列表的展示文案由前端硬编码 switch 给出，
// 管理端表单另有字典接口（cateCode=23）供选值；本表与前端展示路径同源。
var taskLevelCodesCmd = &cobra.Command{
	Use:   "level-codes",
	Short: "查看写实各编号字段的取值对照表",
	Long: `输出写实域三组编号字段的取值对照表（等级 level、审核情况 checkResult、承担角色 playRole）。

纯本地查表，不需要 --token，不访问网络。取值与网页端写实列表的展示文案一致。

与 circle dict --cate-code 23 的分工：本命令是离线速查，取值来自网页端的展示映射；
那条命令读服务端字典接口，用于核对平台当前实际的字典内容。两者都可用，若不一致
以服务端为准。`,
	Example: `  nazhi task level-codes`,
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		printEnvelope(envelope.Success(map[string]map[string]string{
			"level":       types.TaskLevelNames,
			"checkResult": types.CheckResultNames,
			"playRole":    types.PlayRoleNames,
		}))
	},
}

func init() {
	taskCmd.AddCommand(taskLevelCodesCmd)
}

// taskLevelUsage 由 SDK 对照表派生 --level 的 usage 文案（提交/编辑语义）。
//
// 此前两处 flag 各自硬编码「4=区县」，与 TaskLevelName 的「区/县/街道/社区」
// 长期分叉：用户照提示理解的值与脚本从 SDK 取到的名称对不上。
// 由表派生后，改表即改文案，不会再漂。
func taskLevelUsage() string {
	return "等级代码（写实：" + taskLevelCodeList() + "）。" + taskLevelRequiredNote
}

// taskLevelOverrideUsage 派生 preview 命令的 --level usage 文案。
//
// preview 与 submit/edit 的 flag 措辞本就分两套：后两者描述「选一个等级提交」，
// preview 只覆盖 payload 里的字段、不提交，措辞讲的是覆盖行为。刻意保留各自的
// 语义前缀，但码表与必填规则必须同样由表派生——否则改表时 preview 这处会漂，
// 而 preview 同样能覆盖 level，用户看不到可选值就无从下手。
func taskLevelOverrideUsage() string {
	return "覆盖等级代码，可选值：" + taskLevelCodeList() + "。" + taskLevelRequiredNote
}

// taskLevelRequiredNote 说明 level 的填写约束，由 flag usage 告知用户。
//
// CLI 侧不校验这些规则（必填由调用方按活动类型保证，是 SDK 的有意取舍），
// 但前端会校验：活动类型十的表单里 level 无条件必填，类型二/三/四/七的表单里
// 名次与等级成对——填了其一必须填另一个。不告知的话，用户会撞上平台的拒绝却
// 找不到原因。取值来自 managementRightTop.vue 的表单校验分支。
//
// 「留空原样发送」说的是 CLI 的实际行为：flag 与 payload 都不填 level 时，
// 请求里就是空字符串，CLI 不会替你猜一个等级填进去。
const taskLevelRequiredNote = "留空原样发送，CLI 不代填。活动类型 10 必填；" +
	"类型 2/3/4/7 需与名次（rank）成对出现。"

// taskLevelCodeList 把对照表拼成「1=国家 2=省 …」形式的码表串。
//
// sort.Strings 排的是字符串序，不是数值序：当前键只有 "1".."6"，两者恰好
// 同序，所以用户看到的是自然顺序。若平台日后出现两位数编号（如 "10"），
// 字符串序会把它排到 "2" 之前，文案里的码序就不再自然——那时需要改成
// 数值排序。注意 CLI 的 level-codes 输出走 encoding/json，本身按字符串序
// 序列化同一批键，届时两处会一起出现同样的顺序问题。
func taskLevelCodeList() string {
	keys := make([]string, 0, len(types.TaskLevelNames))
	for code := range types.TaskLevelNames {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, code := range keys {
		parts = append(parts, code+"="+types.TaskLevelNames[code])
	}
	return strings.Join(parts, " ")
}
