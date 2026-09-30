package main

import (
	"sort"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
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

纯本地查表，不需要 --token，不访问网络。取值与网页端展示文案一致；
平台为准，本表供脚本与人工速查。`,
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

// taskLevelUsage 由 SDK 对照表派生 --level 的 usage 文案。
//
// 此前两处 flag 各自硬编码「4=区县」，与 TaskLevelName 的「区/县/街道/社区」
// 长期分叉：用户照提示理解的值与脚本从 SDK 取到的名称对不上。
// 由表派生后，改表即改文案，不会再漂。
//
// sort.Strings 排的是字符串序，不是数值序：当前键只有 "1".."6"，两者恰好
// 同序，所以用户看到的是自然顺序。若平台日后出现两位数编号（如 "10"），
// 字符串序会把它排到 "2" 之前，usage 文案的码序就不再自然——那时需要改成
// 数值排序。注意 CLI 的 level-codes 输出走 encoding/json，本身按字符串序
// 序列化同一批键，届时两处会一起出现同样的顺序问题。
func taskLevelUsage() string {
	keys := make([]string, 0, len(types.TaskLevelNames))
	for code := range types.TaskLevelNames {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, code := range keys {
		parts = append(parts, code+"="+types.TaskLevelNames[code])
	}
	return "等级代码（可选，写实：" + strings.Join(parts, " ") + "；空则原样不默认 5）"
}
