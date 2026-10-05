package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// TestTaskHierarchyCommandsRegistered 验证完整的任务三级层级命令及别名均挂载在 taskCmd 下。
func TestTaskHierarchyCommandsRegistered(t *testing.T) {
	expectedSubcommands := []*cobra.Command{
		taskDimensionsCmd,
		taskCategoriesCmd,
		taskItemsCmd,
		taskCircleTypeCmd,
		taskRecentCmd,
	}
	for _, command := range expectedSubcommands {
		if command.Parent() == nil {
			t.Fatalf("任务命令 %q 未挂载到父命令", command.Name())
		}
		if command.Parent() != taskCmd {
			t.Fatalf("任务命令 %q 的父命令不是 taskCmd", command.Name())
		}
	}

	// 验证别名（保证开发者使用 categories 或 types，items 或 tasks 均可）
	if len(taskCategoriesCmd.Aliases) == 0 || taskCategoriesCmd.Aliases[0] != "types" {
		t.Fatalf("task categories 应包含别名 types，实际为 %v", taskCategoriesCmd.Aliases)
	}
	if len(taskItemsCmd.Aliases) == 0 || taskItemsCmd.Aliases[0] != "tasks" {
		t.Fatalf("task items 应包含别名 tasks，实际为 %v", taskItemsCmd.Aliases)
	}

	// 验证必要参数存在
	if taskCategoriesCmd.Flags().Lookup("dimension-id") == nil || taskCategoriesCmd.Flags().Lookup("pid") == nil {
		t.Fatal("task categories 应暴露 --dimension-id 与 --pid")
	}
	if taskItemsCmd.Flags().Lookup("type-id") == nil {
		t.Fatal("task items 应暴露 --type-id")
	}
}

// TestTaskHierarchyCommands_Execution 验证各层级命令的正常执行与输出信封格式。
func TestTaskHierarchyCommands_Execution(t *testing.T) {
	queries := make(map[string]string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"测试学生","studentNumber":"TEST2026001"}}`))
		case "/api/studentCircleNew/getCircleType":
			queries["categories.dim"] = r.URL.Query().Get("dimensionId")
			queries["categories.pid"] = r.URL.Query().Get("pid")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":101,"name":"思想素养类别"}]}`))
		case "/api/studentCircleNew/getCircleTask":
			queries["items.typeId"] = r.URL.Query().Get("typeId")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":202,"name":"敬老院志愿活动"}]}`))
		case "/api/studentCircleNew/getRecentlyCircleTask":
			queries["recent.hit"] = "1"
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"id":303,"name":"近期劳动写实","circleTaskStatus":"1","scopeTypeName":"年段任务"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// 1. 验证 task categories
	{
		cmd := makeMetadataTestCmd(t, srv.URL)
		_ = cmd.Flags().Set("dimension-id", "14")
		_ = cmd.Flags().Set("pid", "node-root")
		swapGlobals(t)
		stdout, stderr, restore := captureStdio(t)
		taskCategoriesCmd.Run(cmd, nil)
		restore()

		if pendingExitCode.Load() != 0 {
			t.Fatalf("task categories 退出码异常: %d, stderr: %s", pendingExitCode.Load(), stderr.String())
		}
		var env envelope.Envelope
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("解析 task categories 输出信封失败: %v, raw stdout: %s", err, stdout.String())
		}
		if env.Code != 200 {
			t.Fatalf("期望状态码 200，实际 %d", env.Code)
		}
		if queries["categories.dim"] != "14" || queries["categories.pid"] != "node-root" {
			t.Fatalf("服务端收到的参数不符合预期: %v", queries)
		}
	}

	// 2. 验证 task items
	{
		cmd := makeMetadataTestCmd(t, srv.URL)
		_ = cmd.Flags().Set("type-id", "101")
		swapGlobals(t)
		stdout, stderr, restore := captureStdio(t)
		taskItemsCmd.Run(cmd, nil)
		restore()

		if pendingExitCode.Load() != 0 {
			t.Fatalf("task items 退出码异常: %d, stderr: %s", pendingExitCode.Load(), stderr.String())
		}
		var env envelope.Envelope
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("解析 task items 输出信封失败: %v, raw stdout: %s", err, stdout.String())
		}
		if env.Code != 200 {
			t.Fatalf("期望状态码 200，实际 %d", env.Code)
		}
		if queries["items.typeId"] != "101" {
			t.Fatalf("服务端收到的 typeId 不符合预期: %s", queries["items.typeId"])
		}
	}

	// 3. 验证 task recent
	{
		cmd := makeMetadataTestCmd(t, srv.URL)
		swapGlobals(t)
		stdout, stderr, restore := captureStdio(t)
		taskRecentCmd.Run(cmd, nil)
		restore()

		if pendingExitCode.Load() != 0 {
			t.Fatalf("task recent 退出码异常: %d, stderr: %s", pendingExitCode.Load(), stderr.String())
		}
		var env envelope.Envelope
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("解析 task recent 输出信封失败: %v, raw stdout: %s", err, stdout.String())
		}
		if env.Code != 200 {
			t.Fatalf("期望状态码 200，实际 %d", env.Code)
		}
		if queries["recent.hit"] != "1" {
			t.Fatal("未发起 getRecentlyCircleTask 请求")
		}
	}
}

// TestTaskHierarchyCommands_Validation 验证非法参数拒绝与错误通道。
func TestTaskHierarchyCommands_Validation(t *testing.T) {
	// 1. task categories 非法 dimension-id
	{
		cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
		_ = cmd.Flags().Set("dimension-id", "0")
		swapGlobals(t)
		_, stderr, restore := captureStdio(t)
		taskCategoriesCmd.Run(cmd, nil)
		restore()
		assertParamError(t, stderr.String(), "--dimension-id")
	}

	// 2. task items 非法 type-id
	{
		cmd := makeMetadataTestCmd(t, "http://127.0.0.1:1")
		_ = cmd.Flags().Set("type-id", "-1")
		swapGlobals(t)
		_, stderr, restore := captureStdio(t)
		taskItemsCmd.Run(cmd, nil)
		restore()
		assertParamError(t, stderr.String(), "--type-id")
	}
}
