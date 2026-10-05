package main

import (
	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/Nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// typicalCaseCodesCmd 输出典型案例域「数字→中文名」的完整对照表。
//
// 纯本地查表：不联网、不需要 token、不发任何请求，因此不注册 --token 等
// 业务 flag。表内容取自 pkg/types 的三张对照表（唯一真相源），命令不另抄一份。
//
// 与 nazhi task level-codes 的关系：两域各有一套 level 码表，本命令查的是
// 典型案例域（1 国际 / 2 省 / 3 市 / 4 区县 / 5 学校），那条查的是写实域
// （1 国家 / 2 省 / 3 地区市 / 4 区县街道社区 / 5 校 / 6 年段）。编号相同
// 处语义不同，两表严禁互相代入——写实提交填 level=4 表示「区/县/街道/社区」，
// 典型案例填 level=4 表示「区县」。故本命令与那条分设两路、输出分属不同信封。
var typicalCaseCodesCmd = &cobra.Command{
	Use:   "level-codes",
	Short: "查看典型案例各编号字段的取值对照表",
	Long: `输出典型案例域三组编号字段的取值对照表（材料类别 type、个人角色 role、获奖级别 level）。

纯本地查表，不需要 --token，不访问网络。取值与网页端典型案例表单的下拉选项一致。

注意：本表与 nazhi task level-codes 查的不是同一套码表。典型案例的 level
是「获奖级别」（1 国际 / 2 省 / 3 市 / 4 区县 / 5 学校），写实的 level 是
「行政层级」（1 国家 … 6 年段）。两表编号重叠处语义不同，不可互相代入。`,
	Example: `  nazhi typical-case level-codes`,
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		printEnvelope(envelope.Success(map[string]map[string]string{
			"type":  types.TypicalCaseTypeNames,
			"role":  types.TypicalCaseRoleNames,
			"level": types.TypicalCaseLevelNames,
		}))
	},
}
