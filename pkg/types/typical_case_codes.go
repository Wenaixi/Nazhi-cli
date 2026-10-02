package types

// typical_case_codes.go 是典型案例域编号码表的唯一真相源。
//
// 取值来自前端 classiccanter.vue 的 el-option 硬编码（下拉无字典接口，
// 平台侧亦无对应字典端点），Go 表与之逐字一致。
//
// 与写实域三张表的关系：典型案例 level（1 国际 / 2 省 / 3 市 / 4 区县 /
// 5 学校）与写实 level（1 国家 / 2 省 / 3 地区市 / 4 区县街道社区 / 5 校 /
// 6 年段）是**两套独立码表**，编号相同处语义不同。严禁合并、严禁互相代入：
// 写实提交时 level=4 是「区/县/街道/社区」，典型案例 level=4 是「区县」。
// 查表命令亦分设两路（nazhi task level-codes 与 nazhi typical-case level-codes），
// 不并入同一信封输出，以免制造混用入口。
//
// 可变包级状态：这三张表是导出变量，下游写入会直接改变补全函数的返回值。
// Go 没有只读 map 的惯用写法，而可枚举正是本组表存在的理由（CLI 查表命令
// 需遍历它），故此处不加防御。下游只应读取，不要写入。

// TypicalCaseTypeNames 是典型案例材料类别码到展示名的对照表。
var TypicalCaseTypeNames = map[string]string{
	"1": "研究性学习报告",
	"2": "社会调查报告",
	"3": "艺术创作作品",
	"4": "其他",
}

// TypicalCaseRoleNames 是典型案例个人角色码到展示名的对照表。
var TypicalCaseRoleNames = map[string]string{
	"1": "负责人",
	"2": "参与者",
}

// TypicalCaseLevelNames 是典型案例获奖级别码到展示名的对照表。
//
// 注意 level=1 为「国际」而非写实域的「国家」，level=5 为「学校」而非「校」——
// 这两处正是两套码表最容易互相代入的地方。
var TypicalCaseLevelNames = map[string]string{
	"1": "国际",
	"2": "省",
	"3": "市",
	"4": "区县",
	"5": "学校",
}
