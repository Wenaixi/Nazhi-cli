package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/Nazhi-cli/pkg/client"
)

// TestListCircleRecords_MergedRecordsClampedByRecordUpperBound 锁定合并阶段的条数闸。
//
// 背景：fetchAllCirclePages 的容量早退分支（capacity > maxSubmittedRecords）
// 只按服务端声明的 totalNum 判定，而翻页页数取
// max(totalPage, ceil(totalNum/pageSize))——totalPage 虚高时页数可远超
// totalNum 推导值。虚高页数乘以每页满页条数累加后，已驻留的记录条数能数倍
// 越过条数上界 maxSubmittedRecords，而合并循环此前无任何上界。
//
// 夹具形态与生产同形：totalNum=40000（服务端低报总数，低于条数闸一半）
// 配 totalPage=1001（虚高声明，真实只需 400 页）。totalPage 远大于
// totalNum 推导值是 derivePageBounds 已锁定的合法规则输入
// （见 TestDerivePageBounds_UsesTotalNumAsLowerBound 的同名用例），
// 服务端 count 与 list 不自洽亦为代码注释已承认的现实（见容量早退处的
// totalNum < len(page1) 防御）。每页满页返回 100 条，声明页数
// 1001 × 100 = 100100 条，越过条数闸（10 万）恰好 100 条。
//
// 三条互不依赖的判据，避免「截断到任意较短前缀」蒙混过关：
//   - len(records) == 100000：锁定回退到的精确页号，不接受任何更短前缀；
//   - 末条 ID == 100000：ID 全局连续是独立于 len 的第二条判据，若合并时
//     跳过或重复某页而恰好凑够条数，len 判据无法区分；
//   - 请求次数 == 1001：条数闸落在合并循环处（与字节闸 budgetTruncatePage
//     同层），不减少已发出的翻页请求。若把它前移到发请求之前，此断言变红——
//     这正是本测试要锁定的 seam 位置。
func TestListCircleRecords_MergedRecordsClampedByRecordUpperBound(t *testing.T) {
	const (
		declaredTotalNum = 40_000 // 服务端低报总数：低于条数闸 10 万的一半
		declaredPages    = 1_001  // 虚高声明：真实 ceil(40000/100)=400 页
		pageSize         = 100    // 等于 submittedPageSize，每页满页
	)
	const wantRecords = 100_000

	var callCount int64
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		atomic.AddInt64(&callCount, 1)
		pageNo, err := strconv.Atoi(r.URL.Query().Get("pageNo"))
		if err != nil || pageNo < 1 {
			t.Errorf("pageNo 参数异常: %q", r.URL.Query().Get("pageNo"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		list := make([]map[string]any, pageSize)
		for i := range list {
			// ID 全局连续：第 pn 页承载 ID (pn-1)*100+1 .. pn*100。
			list[i] = submittedRecord(int64((pageNo-1)*pageSize+i+1), "满页记录", 0)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":     1,
			"pageBean": json.RawMessage(submittedPageBean(pageNo, pageSize, declaredTotalNum, declaredPages)),
			"dataList": list,
		})
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL),
		client.WithTimeout(30*time.Second),
		client.WithSubmittedPageSize(pageSize),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	records, err := c.ListCircleRecords(context.Background(), "tok", client.CircleListSubmitted, "")
	if err != nil {
		t.Fatalf("ListCircleRecords: %v", err)
	}

	if len(records) != wantRecords {
		t.Errorf("合并条数应回退到合法前缀 %d 条，实际 %d 条（声明 %d 页 × 满页 %d = %d 条）",
			wantRecords, len(records), declaredPages, pageSize, declaredPages*pageSize)
	}
	if last := len(records) - 1; last >= 0 && records[last].ID != int64(wantRecords) {
		t.Errorf("末条 ID 应为 %d（第 1000 页末条），实际 %d：回退边界错位或合并跳过/重复了页",
			wantRecords, records[last].ID)
	}
	if first := records[0].ID; first != 1 {
		t.Errorf("首条 ID 应为 1（首页起始），实际 %d", first)
	}
	if got := atomic.LoadInt64(&callCount); got != int64(declaredPages) {
		t.Errorf("条数闸在合并处不减少翻页请求，实际发出 %d 次，期望 %d 次", got, declaredPages)
	}
}

// TestListCircleRecords_SelfConsistentPagesUnaffected 反向安全验证：
// 服务端自洽场景（totalNum 与 totalPage 匹配、末页不满）行为必须逐字未变——
// 合并条数不超过条数闸时不触发任何回退，全部记录原样返回。
//
// 与 TestListCircleRecords_MergedRecordsClampedByRecordUpperBound 配对：
// 那条验证「越界时截断到精确前缀」，本条验证「未越界时一个都不丢」。
func TestListCircleRecords_SelfConsistentPagesUnaffected(t *testing.T) {
	const (
		declaredTotalNum = 250 // 3 页：100 + 100 + 50
		declaredPages    = 3
		pageSize         = 100
	)

	var callCount int64
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		atomic.AddInt64(&callCount, 1)
		pageNo, err := strconv.Atoi(r.URL.Query().Get("pageNo"))
		if err != nil || pageNo < 1 {
			t.Errorf("pageNo 参数异常: %q", r.URL.Query().Get("pageNo"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// 末页只返回 50 条：自洽分页的正常形态。
		pageLen := pageSize
		if pageNo == declaredPages {
			pageLen = declaredTotalNum - (declaredPages-1)*pageSize
		}
		list := make([]map[string]any, pageLen)
		for i := range list {
			list[i] = submittedRecord(int64((pageNo-1)*pageSize+i+1), "自洽记录", 0)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":     1,
			"pageBean": json.RawMessage(submittedPageBean(pageNo, pageSize, declaredTotalNum, declaredPages)),
			"dataList": list,
		})
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithSubmittedPageSize(pageSize),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	records, err := c.ListCircleRecords(context.Background(), "tok", client.CircleListSubmitted, "")
	if err != nil {
		t.Fatalf("ListCircleRecords: %v", err)
	}

	if len(records) != declaredTotalNum {
		t.Errorf("自洽场景应原样返回全部 %d 条，实际 %d 条", declaredTotalNum, len(records))
	}
	for i, rec := range records {
		if want := int64(i + 1); rec.ID != want {
			t.Fatalf("第 %d 条 ID 应为 %d，实际 %d（合并顺序被打乱或有条被丢弃）", i, want, rec.ID)
		}
	}
	if got := atomic.LoadInt64(&callCount); got != declaredPages {
		t.Errorf("自洽场景应发出 %d 次请求，实际 %d 次", declaredPages, got)
	}
}
