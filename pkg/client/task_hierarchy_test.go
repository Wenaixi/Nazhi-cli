package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestTaskHierarchy_CategoriesAlias 验证 GetTaskCategories 与 GetCircleTypes 行为一致且正确传递 pid 与 dimensionId。
func TestTaskHierarchy_CategoriesAlias(t *testing.T) {
	const rawPID = "node&123=x"
	var gotPath string
	var gotRawQuery string

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getCircleType" {
			gotPath = r.URL.Path
			gotRawQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"msg":  "success",
				"dataList": []map[string]any{
					{"id": float64(101), "name": "德育类别"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	// 1. 调用 GetTaskCategories
	cats, err := c.GetTaskCategories(context.Background(), "test-token", 14, rawPID)
	if err != nil {
		t.Fatalf("GetTaskCategories 失败: %v", err)
	}
	if gotPath != "/api/studentCircleNew/getCircleType" {
		t.Fatalf("请求路径不符合预期: %s", gotPath)
	}
	if len(cats) != 1 || cats[0]["name"] != "德育类别" {
		t.Fatalf("返回结果不符合预期: %v", cats)
	}
	wantEscaped := url.QueryEscape(rawPID)
	if !strings.Contains(gotRawQuery, "pid="+wantEscaped) {
		t.Fatalf("期望 RawQuery 包含 pid=%s，实际为 %s", wantEscaped, gotRawQuery)
	}

	// 2. 调用 GetCircleTypes 确保别名一致性
	types, err := c.GetCircleTypes(context.Background(), "test-token", 14, rawPID)
	if err != nil {
		t.Fatalf("GetCircleTypes 失败: %v", err)
	}
	if len(types) != 1 || types[0]["name"] != "德育类别" {
		t.Fatalf("GetCircleTypes 返回与 GetTaskCategories 不一致: %v", types)
	}
}

// TestTaskHierarchy_ItemsAlias 验证 GetTaskItems 与 GetCircleTasks 行为一致。
func TestTaskHierarchy_ItemsAlias(t *testing.T) {
	var gotPath string
	var gotQuery string

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getCircleTask" {
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"msg":  "success",
				"dataList": []map[string]any{
					{"id": float64(9274), "name": "志愿服务活动"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	// 1. 调用 GetTaskItems
	tasks, err := c.GetTaskItems(context.Background(), "test-token", 101)
	if err != nil {
		t.Fatalf("GetTaskItems 失败: %v", err)
	}
	if gotPath != "/api/studentCircleNew/getCircleTask" || gotQuery != "typeId=101" {
		t.Fatalf("请求路径或参数错误: %s?%s", gotPath, gotQuery)
	}
	if len(tasks) != 1 || tasks[0]["name"] != "志愿服务活动" {
		t.Fatalf("返回任务结果不符合预期: %v", tasks)
	}

	// 2. 调用 GetCircleTasks
	circleTasks, err := c.GetCircleTasks(context.Background(), "test-token", 101)
	if err != nil {
		t.Fatalf("GetCircleTasks 失败: %v", err)
	}
	if len(circleTasks) != 1 || circleTasks[0]["name"] != "志愿服务活动" {
		t.Fatalf("GetCircleTasks 返回与 GetTaskItems 不一致: %v", circleTasks)
	}
}

// TestTaskHierarchy_RecentlyCircleTask 验证 GetRecentlyCircleTask 获取最新提交任务。
func TestTaskHierarchy_RecentlyCircleTask(t *testing.T) {
	var callCount int

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getRecentlyCircleTask" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				"msg":  "success",
				"dataList": []map[string]any{
					{
						"id":               float64(5521),
						"name":             "学雷锋志愿服务",
						"scopeTypeName":    "年段任务",
						"circleTaskStatus": "1",
						"startDateStr":     "2026-03-01",
						"endDateStr":       "2026-06-30",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	recent, err := c.GetRecentlyCircleTask(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetRecentlyCircleTask 失败: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("期望调用 1 次，实际 %d", callCount)
	}
	if len(recent) != 1 {
		t.Fatalf("期望 1 条记录，实际 %d", len(recent))
	}
	if recent[0]["name"] != "学雷锋志愿服务" || recent[0]["circleTaskStatus"] != "1" {
		t.Fatalf("返回字段内容异常: %v", recent[0])
	}
}

// TestTaskHierarchy_RecentlyCircleTask_BusinessError 验证业务失败时返回 ErrBusinessRejected。
func TestTaskHierarchy_RecentlyCircleTask_BusinessError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getRecentlyCircleTask" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": -1,
				"msg":  "获取最近任务失败",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetRecentlyCircleTask(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望返回错误，实际为 nil")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Fatalf("期望 ErrBusinessRejected，实际: %v", err)
	}
}
