package main

import (
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
