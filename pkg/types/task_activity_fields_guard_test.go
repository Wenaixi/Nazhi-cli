// Package types 公共类型契约测试 — 活动字段声明一致性守卫。
//
// 24 个活动字段在仓库里有三处结构体声明与两份搬运函数：
//
//   - ActivityFields（聚合 struct，唯一的消费入口）
//   - TaskSubmitInput（平铺声明 + GetActivityFields 搬运）
//   - TaskEditInput（平铺声明 + GetActivityFields 搬运）
//
// buildTaskPayload 经 GetActivityFields 一次取值，因此「某字段没被搬进
// ActivityFields」等于「该字段永远不出现在提交 payload 里」。而这类漏改
// 编译器不报错：给 TaskSubmitInput 单独加一个字段后，go build / go vet /
// 全量单测三者全绿。
//
// 为什么用守卫而不是把 ActivityFields 内嵌进两个输入类型：Go 的字段提升
// 只对选择器表达式（v.Name）生效，对键名复合字面量（TaskSubmitInput{Name:…}）
// 永久失效——实测 go vet 报 unknown field Name in struct literal。仓内 18 个
// 文件 51 处键名字面量会全部编译失败，且爆炸点在下游 SDK 消费者的编译期，
// 本仓 CI 全绿也无法拦截。因此保留平铺声明，用本守卫把「三处必须一致」与
// 「搬运必须覆盖全集」变成可执行断言。
//
// 验证策略（两条缺一不可）：
//  1. 三处结构体的活动字段集互相一致（否则搬运函数无法两侧都覆盖）
//  2. 两份 GetActivityFields 各自覆盖 ActivityFields 全集
//
// 只锁第 1 条会恒绿：三处结构体一致、但两份搬运同时漏搬某字段时，字段集
// 仍然一致而该字段已丢失。
package types

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"
)

// activityFieldSets 是活动字段的三处声明点。
//
// 非活动字段（TaskID / Content / ImagePaths / ImageIDs / ID）不在搬运范围
// 内，由 buildTaskPayload 单独处理，故比对时剔除。
var nonActivityFields = map[string]bool{
	"TaskID": true, "Content": true,
	"ImagePaths": true, "ImageIDs": true,
	"ID": true,
}

// parseTaskGo 解析 task.go，返回其 AST。
func parseTaskGo(t *testing.T) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "task.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 task.go 失败: %v", err)
	}
	return fset, f
}

// structFieldSet 返回指定结构体的字段名集合。
func structFieldSet(t *testing.T, f *ast.File, name string) map[string]bool {
	t.Helper()
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s 不是 struct 类型", name)
			}
			out := make(map[string]bool)
			for _, field := range st.Fields.List {
				for _, n := range field.Names {
					out[n.Name] = true
				}
			}
			return out
		}
	}
	t.Fatalf("task.go 中未找到类型 %s", name)
	return nil
}

// movedFieldSet 返回 recv.GetActivityFields 搬运的字段名集合
// （即函数体里形如 `Field: in.Field` 的键）。
func movedFieldSet(t *testing.T, f *ast.File, recv string) map[string]bool {
	t.Helper()
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "GetActivityFields" || fd.Recv == nil {
			continue
		}
		if len(fd.Recv.List) == 0 {
			continue
		}
		ident, ok := fd.Recv.List[0].Type.(*ast.Ident)
		if !ok || ident.Name != recv {
			continue
		}
		out := make(map[string]bool)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				return true
			}
			sel, ok := kv.Value.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "in" {
				out[key.Name] = true
			}
			return true
		})
		return out
	}
	t.Fatalf("task.go 中未找到 %s 的 GetActivityFields 方法", recv)
	return nil
}

// activityFields 剔除非活动字段后的字段名集合。
func activityFields(all map[string]bool) map[string]bool {
	out := make(map[string]bool, len(all))
	for name := range all {
		if !nonActivityFields[name] {
			out[name] = true
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// diffKeys 返回只在 a 中出现的键。
func diffKeys(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// TestActivityFields_DeclarationsAgree 守护：三处结构体的活动字段集一致。
//
// 新增活动字段时若只改其中一处，本测试精确点名。
func TestActivityFields_DeclarationsAgree(t *testing.T) {
	_, f := parseTaskGo(t)

	af := activityFields(structFieldSet(t, f, "ActivityFields"))
	sub := activityFields(structFieldSet(t, f, "TaskSubmitInput"))
	edit := activityFields(structFieldSet(t, f, "TaskEditInput"))

	pairs := []struct {
		left, right     string
		leftSet, rightS map[string]bool
	}{
		{"ActivityFields", "TaskSubmitInput", af, sub},
		{"ActivityFields", "TaskEditInput", af, edit},
		{"TaskSubmitInput", "TaskEditInput", sub, edit},
	}
	for _, p := range pairs {
		if onlyLeft := diffKeys(p.leftSet, p.rightS); len(onlyLeft) > 0 {
			t.Errorf("%s 有而 %s 无的活动字段 %v：搬运函数无法两侧都覆盖，新增字段必须同时改三处",
				p.left, p.right, onlyLeft)
		}
		if onlyRight := diffKeys(p.rightS, p.leftSet); len(onlyRight) > 0 {
			t.Errorf("%s 有而 %s 无的活动字段 %v：同上", p.right, p.left, onlyRight)
		}
	}
}

// TestActivityFields_MoveFunctionsCoverAllFields 守护：两份搬运函数各自
// 覆盖 ActivityFields 全集。
//
// 这是第 1 条守不住的失败面：三处结构体一致、但两份 GetActivityFields 同时
// 漏搬某字段时，字段集断言恒绿，而该字段已永久丢失出提交 payload。
func TestActivityFields_MoveFunctionsCoverAllFields(t *testing.T) {
	_, f := parseTaskGo(t)

	af := activityFields(structFieldSet(t, f, "ActivityFields"))
	for _, recv := range []string{"TaskSubmitInput", "TaskEditInput"} {
		moved := movedFieldSet(t, f, recv)
		if missing := diffKeys(af, moved); len(missing) > 0 {
			t.Errorf("%s.GetActivityFields 漏搬字段 %v：该字段永远不会出现在提交 payload 里"+
				"（buildTaskPayload 经 ActivityFields 一次取值）", recv, missing)
		}
		if extra := diffKeys(moved, af); len(extra) > 0 {
			t.Errorf("%s.GetActivityFields 搬运了 ActivityFields 中不存在的字段 %v", recv, extra)
		}
	}
}
