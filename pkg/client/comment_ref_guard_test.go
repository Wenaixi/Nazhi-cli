package client

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestNoOutOfRangeFileLineRefs 禁止注释里的「file.go:NNN」引用指向不存在的行。
//
// 为什么需要它：注释用行号跨文件指向别处的实现，是本仓反复出现的失实来源——
// 目标文件一改动，引用就悄悄指错地方，而没有任何东西会报红。本测试把
// 「引用必须落在目标文件实际行数内」这条底线机械化。
//
// 刻意不校验「引用行是否恰好是所述符号」：那需要解析双方 AST 并做语义匹配，
// 判据脆弱且易误报（注释常指向函数体的某一具体语句而非声明行）。行数越界是
// 无歧义的硬错误，先守住这条；语义漂移靠「引用符号名而非行号」的纪律避免
// （CLAUDE.md「注释与测试纪律」已要求注释用符号名而非行号交叉引用）。
//
// 豁免：internal/version/version.go 的版本演进注释块按设计记录历史行号
// （"v1.x 修了 file.go:435 的 XX"），语义上不随代码漂移，不适用本守卫。

func TestNoOutOfRangeFileLineRefs(t *testing.T) {
	// 匹配注释中的 file.go:123 形式，排除测试文件自身。
	ref := regexp.MustCompile(`([a-z_]+\.go):(\d+)`)

	// 收集各文件的行数：同名文件可能存在于多个目录，取最大行数并记录全部。
	lineCounts := map[string][]int{}
	// 扫描根为仓库根：go test 的工作目录是包目录 pkg/client，故需上溯两级。
	// 取仓库根而非包目录，使 pkg / cmd / internal 三处的跨文件引用一并纳入
	// ——漂移实际多发于 cmd/ 层。
	repoRoot := mustAbs("../..")
	for _, root := range []string{repoRoot} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name != "." && (strings.HasPrefix(name, ".") || name == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			n := len(strings.Split(string(data), "\n"))
			lineCounts[info.Name()] = append(lineCounts[info.Name()], n)
			return nil
		})
	}

	var violations []string
	for _, root := range []string{repoRoot} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if err == nil && info != nil && info.IsDir() {
					name := info.Name()
					if name != "." && (strings.HasPrefix(name, ".") || name == "testdata") {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			// 版本演进注释按设计记录历史行号，豁免。
			if info.Name() == "version.go" && strings.Contains(filepath.ToSlash(path), "internal/version") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			for i, line := range strings.Split(string(data), "\n") {
				// 只看注释行，避开字符串字面量里的偶然匹配。
				trimmed := strings.TrimSpace(line)
				if !strings.HasPrefix(trimmed, "//") {
					continue
				}
				for _, m := range ref.FindAllStringSubmatch(line, -1) {
					target, lineNo := m[1], 0
					if _, err := strconv.Atoi(m[2]); err != nil {
						continue
					}
					lineNo, _ = strconv.Atoi(m[2])
					counts, ok := lineCounts[target]
					if !ok {
						continue // 目标文件不在本包范围（如引用 docs/），不判
					}
					max := 0
					for _, c := range counts {
						if c > max {
							max = c
						}
					}
					if lineNo > max {
						violations = append(violations, fmt.Sprintf(
							"%s:%d 引用 %s，但该文件最长仅 %d 行", path, i+1, m[0], max))
					}
				}
			}
			return nil
		})
	}

	if len(violations) > 0 {
		t.Errorf("注释中的行号引用越界 %d 处（目标文件已变短，引用失实）:\n  %s\n"+
			"请改用符号名引用（CLAUDE.md「注释与测试纪律」要求），行号会随编辑漂移。",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// mustAbs 把路径解析为绝对路径。
//
// 存在的理由：filepath.Walk 以 root 自身的 basename 作为第一次回调的
// info.Name()。传相对路径 "../.." 时该 name 就是 ".."，会被「跳过以点开头
// 的隐藏目录」判据 SkipDir ——整个 walk 尚未展开就被跳过，守卫静默失效
// 且全绿。先解析成绝对路径即可避开该陷阱。
func mustAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
