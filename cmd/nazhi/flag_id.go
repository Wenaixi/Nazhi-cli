package main

import (
	"errors"
	"strconv"

	"github.com/spf13/cobra"
)

// circleIDFromFlag 读取 --id 并校验为正整数，失败时输出参数错误信封。
//
// 收口的理由：写实 delete / like / comment 与典型案例 delete 四处各写一遍
// 「读 --id → 判空 → ParseInt → 判正」，连错误文案都逐字相同。改判据或改文案
// 时要四处手工同步，漏一处就出现命令间行为不一致。
//
// 返回 false 表示已输出参数错误信封，调用方必须立即返回、不得继续建客户端——
// 先校后建是本仓的用户可见契约：缺 --token 与 --id 非法同时发生时，
// 用户必须先看到真正该修的那个错，而不是先抱怨 token 缺失。
func circleIDFromFlag(cmd *cobra.Command) (int64, bool) {
	idStr, _ := cmd.Flags().GetString("id")
	if idStr == "" {
		printParamError(errors.New("--id 为必填"))
		return 0, false
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		printParamError(errors.New("--id 必须为正整数"))
		return 0, false
	}
	return id, true
}
