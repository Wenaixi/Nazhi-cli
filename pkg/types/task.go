package types

import "strings"

// 任务作用域常量（对应服务端 scopeType）。
const (
	ScopeClass = 1 // 班级任务
	ScopeGrade = 2 // 年段任务
	ScopeStage = 3 // 学段任务
)

// 承担角色常量（对应服务端 playRole 数字编码）。
const (
	PlayRoleHost            = "1" // 主持策划者
	PlayRoleMainParticipant = "2" // 主要参与者
	PlayRoleParticipant     = "3" // 参与者
)

// PlayRoleNames 是承担角色码到展示名的对照表，是 playRole 组编号语义的唯一真相源。
//
// 取值来自前端三组件逐字一致的 switch；平台无对应字典接口，是纯前端硬编码。
var PlayRoleNames = map[string]string{
	PlayRoleHost:            "主持策划者",
	PlayRoleMainParticipant: "主要参与者",
	PlayRoleParticipant:     "参与者",
}

// PlayRoleName 返回承担角色代码的展示名，未知返回空串。
func PlayRoleName(code string) string {
	return PlayRoleNames[code]
}

// 写实等级常量（对应服务端 level，对齐前端管理端字典与展示映射）。
//
// 前端来源（reference/nazhi/src 对照）：
//   - 获取字典：GET /api/common/sys/dict/list?cateCode=23 → 填充下拉（level 列表）
//   - 展示映射：managementRightBottom.vue / yhmanagement 同步的 switch(map.level)
//     1=国家  2=省  3=地区/市  4=区/县/街道/社区  5=校  6=年段
//   - 提交校验：checkData 中 rank/level 成对校验（部分 targetName）
//
// SDK 约定：空串原样发送，不发明 "5"；展示名与字典名以服务端为准，此处常量仅为调用方显式赋值时的可读别名。
const (
	TaskLevelNational = "1" // 国家
	TaskLevelProvince = "2" // 省
	TaskLevelCity     = "3" // 地区/市
	TaskLevelCounty   = "4" // 区/县/街道/社区
	TaskLevelSchool   = "5" // 校
	TaskLevelGrade    = "6" // 年段
)

// TaskLevelNames 是写实等级码到展示名的对照表，是 level 组编号语义的唯一真相源。
//
// 取值来自前端三组件逐字一致的 switch（公示页与管理页的写实列表展示路径），
// 与前端 management 端表单的字典接口（cateCode=23）取值同源但用途不同：
// 字典供表单选值，本表供展示与离线速查。
//
// 注意：典型案例域另有一套 level 码表（1=国际/2=省/3=市/4=区县/5=学校），
// 与本表语义不同，严禁合并。
var TaskLevelNames = map[string]string{
	TaskLevelNational: "国家",
	TaskLevelProvince: "省",
	TaskLevelCity:     "地区/市",
	TaskLevelCounty:   "区/县/街道/社区",
	TaskLevelSchool:   "校",
	TaskLevelGrade:    "年段",
}

// TaskLevelName 返回等级代码对应的展示名（1..6），未知返回空串。
func TaskLevelName(code string) string {
	return TaskLevelNames[code]
}

// 审核情况常量（对应服务端 checkResult，前端写实表单）。
//
// 前端来源（原生 src）：
//   - managementRightTop/Bottom 的 switch(map.check_result): 1=优秀 2=良 3=合格 4=差
//   - 表单中部分 targetName 用 radio（1/3），部分用 select；展示映射一致。
const (
	CheckResultExcellent = "1" // 优秀
	CheckResultGood      = "2" // 良
	CheckResultPass      = "3" // 合格
	CheckResultPoor      = "4" // 差
)

// CheckResultNames 是审核情况码到展示名的对照表，是 checkResult 组编号语义的唯一真相源。
//
// 取值来自前端三组件逐字一致的 switch。同一字段在前端另有两种表述：
// 弹窗「考核情况」单选只给两项（1 优秀 / 3 合格），弹窗「审核情况」下拉把第三档写作「中」。
// 本表取写实列表展示路径的「合格」，与既有 SDK 行为一致。
var CheckResultNames = map[string]string{
	CheckResultExcellent: "优秀",
	CheckResultGood:      "良",
	CheckResultPass:      "合格",
	CheckResultPoor:      "差",
}

// CheckResultName 返回审核情况代码的展示名。
func CheckResultName(code string) string {
	return CheckResultNames[code]
}

// Task 是面向调用方的精简任务条目。
//
// 日期展示字段为 string（startDateStr / endDateStr / auditStartDateStr /
// auditEndDateStr，源自服务端的 *Str 键），保留服务端原始日期格式（如
// "2026-01-12"）；creationTime / modifyTime 则是平台原始的 number 数组
// （HAR 实证，如 [2026,1,30,10,19,52]），供脚本按需取值。
type Task struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	TypeName         string  `json:"typeName"`
	DimensionName    string  `json:"dimensionName"`
	Hours            float64 `json:"hours"`
	Score            float64 `json:"score"`
	Remark           string  `json:"remark"`
	CircleTaskStatus string  `json:"circleTaskStatus"`
	Submitted        bool    `json:"submitted"`
	NeedPic          bool    `json:"needPic"`
	StartDate        string  `json:"startDateStr"` // 字符串，如 "2026-01-12"
	EndDate          string  `json:"endDateStr"`   // 字符串，如 "2026-02-10"
	AuditStartDate   string  `json:"auditStartDateStr"`
	AuditEndDate     string  `json:"auditEndDateStr"`
	CreatorName      string  `json:"creatorName"`
	RoleName         string  `json:"roleName"`
	CreationTime     []int   `json:"creationTime"`
	CreationTimeStr  string  `json:"creationTimeStr"` // 字符串
	TermID           int64   `json:"termId"`
	PushNum          int     `json:"pushNum"`
	ScopeType        int     `json:"scopeType"`
	ScopeTypeName    string  `json:"scopeTypeName"`

	// 扩展字段
	SchoolID          int64   `json:"schoolId,omitempty"`
	CircleTypeID      int64   `json:"circleTypeId,omitempty"`
	Creator           int64   `json:"creator,omitempty"`
	Modifier          *int64  `json:"modifier,omitempty"`
	ModifyTime        []int   `json:"modifyTime,omitempty"`
	RoleID            int64   `json:"roleId,omitempty"`
	AuditorSubjectID  *int64  `json:"auditorSubjectId,omitempty"`
	StateType         int     `json:"stateType,omitempty"`
	AreaID            int64   `json:"areaId,omitempty"`
	AreaTaskID        int64   `json:"areaTaskId,omitempty"`
	UpPic             int     `json:"upPic,omitempty"`
	EvaluatedNumber   *int    `json:"evaluatedNumber,omitempty"`
	UnEvaluatedNumber *int    `json:"unEvaluatedNumber,omitempty"`
	UnsubmittedNumber *int    `json:"unsubmittedNumber,omitempty"`
	SubmitNumber      int     `json:"submitNumber,omitempty"`
	PictureList       []int64 `json:"pictureList,omitempty"`
	ClassID           *int64  `json:"classId,omitempty"`
	GradeID           *int64  `json:"gradeId,omitempty"`
}

// ActivityFields 是任务提交/编辑输入中按活动类型区分的全部活动字段聚合。
//
// 对应前端 managementRightBottom.vue 的 form：14 类活动类型共用同一组字段，
// 用户按类型填写其中一部分，空串原样提交（不发明学校名/默认等级）。
// 它不是 wire 类型（不进任何 JSON tag），只作为 TaskInput 的聚合访问单元，
// 让 buildTaskPayload 一次取值而非逐字段 Get* 回声。
type ActivityFields struct {
	PlayRole            string
	Address             string
	Level               string
	Name                string
	HostName            string
	CircleDate          string
	TermName            string
	Rank                string
	ActivityName        string
	SportsName          string
	TeamName            string
	OrgName             string
	ResultsName         string
	ObtainTime          string
	SpecialtyTechnology string
	LikeSpecialty1      string
	LikeSpecialty2      string
	LikeSpecialty3      string
	Hours               string
	CircleBeginDate     string
	CircleEndDate       string
	CheckResult         string
	PatentType          string
	PatentNum           string
}

// TaskInput 定义任务提交/编辑输入的公共接口，用于提取公共 payload 构建逻辑。
//
// 接口只保留真正的差异点：ID（新增 nil / 编辑有值）、任务与正文、图片访问器、
// 活动字段聚合。24 个活动字段不再逐字段 Get* 回声——消费方经 GetActivityFields
// 一次取值，新增字段只动 ActivityFields 一处，不再「改接口 + 两个实现」三处同步。
//
// 兼容：具体类型仍保留 deprecated 薄壳 Getter（转发聚合字段），外部调用方
// 直接调 input.GetName() 仍编译通过。
type TaskInput interface {
	Validate() error
	GetID() *int64
	GetTaskID() int64
	GetContent() string
	GetImagePaths() []string
	GetImageIDs() []int64
	GetActivityFields() ActivityFields
}

// TaskSubmitInput 是公开给 SDK 调用方的最小任务提交输入。
//
// 用户应填：TaskID（选任务）、Content、按任务类型的活动字段、图片；
// Address/OrgName/Level/PlayRole 等按活动类型由调用方填写（前端手填，空串原样提交）；
// Hours：任务 getCircleTypeByTaskId 的 hours>0 时可省略（SDK 用元数据，对齐前端只读自动填）；
// 任务 hours≤0 时必须显式填写（对齐前端可编辑 + checkData）。
// SDK 自动：circleTaskId/circleTypeId/dimensionId（元数据）、pictureList（上传）。
// 不再发明：空 Address/OrgName 不填学校名、空 Level 不默认 "5"（与前端一致）。
// CircleDate/TermName 前端无 v-model，非用户输入，仅兼容保留。
//
// Go 直调提醒：本结构体全部活动字段均为 string（含 Hours/Level/PlayRole/CheckResult 等）；
// Go 调用方如源数据为 number，须自行转为 string 后再赋值（例如 fmt.Sprintf("%v", v)），
// number→string 的兼容仅在 CLI --payload 边界生效（cmd/nazhi 私有 JSON helper），SDK 侧不做自动类型转换。
//
// 校验策略（有意设计）：Validate 仅校验 TaskID>0 && Content 非空，不复制前端 14 分支条件必填；
// 调用方需按活动类型（targetName 1-14）自行保证必填字段，与前端 checkData 对齐；
// 服务端仍会做最终业务校验，缺字段将以 ErrBusinessRejected 返回。
type TaskSubmitInput struct {
	TaskID     int64
	Content    string
	ImagePaths []string
	ImageIDs   []int64
	PlayRole   string
	Address    string
	Level      string

	Name     string
	HostName string
	// CircleDate / TermName：前端 form 有键但**无 v-model**，用户从不手填。
	// 仅兼容旧调用方；推荐保持空串，勿当作用户必填字段。
	CircleDate          string
	TermName            string
	Rank                string
	ActivityName        string
	SportsName          string
	TeamName            string
	OrgName             string
	ResultsName         string
	ObtainTime          string
	SpecialtyTechnology string
	LikeSpecialty1      string
	LikeSpecialty2      string
	LikeSpecialty3      string
	Hours               string
	CircleBeginDate     string
	CircleEndDate       string
	CheckResult         string
	PatentType          string
	PatentNum           string
}

// SetAddressLevel 供 CLI --address/--level flag 覆盖逻辑写入（经
// taskApplyAddressLevelFlags 泛型约束访问）。仅 CLI 边界使用，SDK 调用方
// 直接赋值字段即可。
func (in *TaskSubmitInput) SetAddressLevel(address, level string) {
	if address != "" {
		in.Address = address
	}
	if level != "" {
		in.Level = level
	}
}

func (in TaskSubmitInput) Validate() error {
	if in.TaskID <= 0 {
		return ErrTaskInputTaskIDRequired
	}
	if strings.TrimSpace(in.Content) == "" {
		return ErrTaskInputContentRequired
	}
	return nil
}

// TaskInput 接口实现：TaskSubmitInput 没有 ID 字段，新增记录时 ID 为 nil。
// GetActivityFields 返回活动字段聚合（值拷贝）；以下 Get* 为 deprecated 薄壳，
// 转发聚合字段，仅供外部旧调用方直接取值，新代码请走 GetActivityFields。
func (in TaskSubmitInput) GetID() *int64           { return nil }
func (in TaskSubmitInput) GetTaskID() int64        { return in.TaskID }
func (in TaskSubmitInput) GetContent() string      { return in.Content }
func (in TaskSubmitInput) GetImagePaths() []string { return in.ImagePaths }
func (in TaskSubmitInput) GetImageIDs() []int64    { return in.ImageIDs }
func (in TaskSubmitInput) GetActivityFields() ActivityFields {
	return ActivityFields{
		PlayRole: in.PlayRole, Address: in.Address, Level: in.Level,
		Name: in.Name, HostName: in.HostName, CircleDate: in.CircleDate,
		TermName: in.TermName, Rank: in.Rank, ActivityName: in.ActivityName,
		SportsName: in.SportsName, TeamName: in.TeamName, OrgName: in.OrgName,
		ResultsName: in.ResultsName, ObtainTime: in.ObtainTime,
		SpecialtyTechnology: in.SpecialtyTechnology,
		LikeSpecialty1:      in.LikeSpecialty1, LikeSpecialty2: in.LikeSpecialty2,
		LikeSpecialty3: in.LikeSpecialty3, Hours: in.Hours,
		CircleBeginDate: in.CircleBeginDate, CircleEndDate: in.CircleEndDate,
		CheckResult: in.CheckResult, PatentType: in.PatentType, PatentNum: in.PatentNum,
	}
}

// Deprecated: 薄壳 Getter 转发聚合字段，仅供旧调用方；新代码走 GetActivityFields。
func (in TaskSubmitInput) GetPlayRole() string            { return in.PlayRole }
func (in TaskSubmitInput) GetAddress() string             { return in.Address }
func (in TaskSubmitInput) GetLevel() string               { return in.Level }
func (in TaskSubmitInput) GetName() string                { return in.Name }
func (in TaskSubmitInput) GetHostName() string            { return in.HostName }
func (in TaskSubmitInput) GetCircleDate() string          { return in.CircleDate }
func (in TaskSubmitInput) GetTermName() string            { return in.TermName }
func (in TaskSubmitInput) GetRank() string                { return in.Rank }
func (in TaskSubmitInput) GetActivityName() string        { return in.ActivityName }
func (in TaskSubmitInput) GetSportsName() string          { return in.SportsName }
func (in TaskSubmitInput) GetTeamName() string            { return in.TeamName }
func (in TaskSubmitInput) GetOrgName() string             { return in.OrgName }
func (in TaskSubmitInput) GetResultsName() string         { return in.ResultsName }
func (in TaskSubmitInput) GetObtainTime() string          { return in.ObtainTime }
func (in TaskSubmitInput) GetSpecialtyTechnology() string { return in.SpecialtyTechnology }
func (in TaskSubmitInput) GetLikeSpecialty1() string      { return in.LikeSpecialty1 }
func (in TaskSubmitInput) GetLikeSpecialty2() string      { return in.LikeSpecialty2 }
func (in TaskSubmitInput) GetLikeSpecialty3() string      { return in.LikeSpecialty3 }
func (in TaskSubmitInput) GetHours() string               { return in.Hours }
func (in TaskSubmitInput) GetCircleBeginDate() string     { return in.CircleBeginDate }
func (in TaskSubmitInput) GetCircleEndDate() string       { return in.CircleEndDate }
func (in TaskSubmitInput) GetCheckResult() string         { return in.CheckResult }
func (in TaskSubmitInput) GetPatentType() string          { return in.PatentType }
func (in TaskSubmitInput) GetPatentNum() string           { return in.PatentNum }

// TaskAddCirclePayload 是 SDK 内部使用的 addCircle 完整请求体。
type TaskAddCirclePayload struct {
	ID                  *int64  `json:"id,omitempty"`
	Name                string  `json:"name"`
	HostName            string  `json:"hostName"`
	CircleDate          string  `json:"circleDate"`
	Rank                string  `json:"rank"`
	Level               string  `json:"level"`
	Content             string  `json:"content"`
	PictureList         []int64 `json:"pictureList"`
	CircleTaskID        int64   `json:"circleTaskId"`
	CircleTypeID        int64   `json:"circleTypeId"`
	DimensionID         int64   `json:"dimensionId"`
	Hours               float64 `json:"hours"`
	CircleBeginDate     string  `json:"circleBeginDate"`
	CircleEndDate       string  `json:"circleEndDate"`
	CheckResult         string  `json:"checkResult"`
	PatentType          string  `json:"patentType"`
	PatentNum           string  `json:"patentNum"`
	Address             string  `json:"address"`
	TermName            string  `json:"termName"`
	ActivityName        string  `json:"activityName"`
	SportsName          string  `json:"sportsName"`
	TeamName            string  `json:"teamName"`
	OrgName             string  `json:"orgName"`
	ResultsName         string  `json:"resultsName"`
	ObtainTime          string  `json:"obtainTime"`
	SpecialtyTechnology string  `json:"specialtyTechnology"`
	PlayRole            string  `json:"playRole"`
	LikeSpecialty1      string  `json:"likeSpecialty1"`
	LikeSpecialty2      string  `json:"likeSpecialty2"`
	LikeSpecialty3      string  `json:"likeSpecialty3"`
}

// 兼容旧调用方
type TaskSubmitPayload = TaskAddCirclePayload

// TaskResult 是 addCircle 的业务返回摘要。
type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// TaskCircleTypeInfo 是 getCircleTypeByTaskId 返回的任务提交元数据。
type TaskCircleTypeInfo struct {
	TaskName      string  `json:"task_name"`
	CircleTypeID  int64   `json:"circle_type_id"`
	Hours         float64 `json:"hours"`
	TypeName      string  `json:"type_name"`
	DimensionID   int64   `json:"dimension_id"`
	DimensionName string  `json:"dimension_name"`
	TaskID        int64   `json:"task_id"`
	Remark        string  `json:"remark"`
	Type          int     `json:"type"`
}

// TaskEditInput 是修改写实记录的最小输入。
//
// hours 半自动语义：Hours 留空时回填任务元数据预设（与提交侧一致），
// 但前端编辑场景是用列表记录值覆盖——要保留原值请显式从 CircleRecord.Hours 回填。
//
// 图片语义：ImageIDs 不传时 wire 上发送 pictureList:[]，而前端编辑恒回填原记录图
// （openEdit→imgList）。要保留原图请从 CircleRecord.ImgList 的 attachment_id 回填。
type TaskEditInput struct {
	ID                  int64
	TaskID              int64
	Content             string
	ImagePaths          []string
	ImageIDs            []int64
	PlayRole            string
	Address             string
	Level               string
	Name                string
	HostName            string
	CircleDate          string
	TermName            string
	Rank                string
	ActivityName        string
	SportsName          string
	TeamName            string
	OrgName             string
	ResultsName         string
	ObtainTime          string
	SpecialtyTechnology string
	LikeSpecialty1      string
	LikeSpecialty2      string
	LikeSpecialty3      string
	Hours               string
	CircleBeginDate     string
	CircleEndDate       string
	CheckResult         string
	PatentType          string
	PatentNum           string
}

func (in *TaskEditInput) SetAddressLevel(address, level string) {
	if address != "" {
		in.Address = address
	}
	if level != "" {
		in.Level = level
	}
}
func (in TaskEditInput) Validate() error {
	if in.ID <= 0 {
		return ErrTaskInputIDRequired
	}
	if in.TaskID <= 0 {
		return ErrTaskInputTaskIDRequired
	}
	if strings.TrimSpace(in.Content) == "" {
		return ErrTaskInputContentRequired
	}
	return nil
}

// TaskInput 接口实现：TaskEditInput 的 ID 字段用于修改已有记录。
// GetActivityFields 返回活动字段聚合（值拷贝）；以下 Get* 为 deprecated 薄壳。
func (in TaskEditInput) GetID() *int64           { return &in.ID }
func (in TaskEditInput) GetTaskID() int64        { return in.TaskID }
func (in TaskEditInput) GetContent() string      { return in.Content }
func (in TaskEditInput) GetImagePaths() []string { return in.ImagePaths }
func (in TaskEditInput) GetImageIDs() []int64    { return in.ImageIDs }
func (in TaskEditInput) GetActivityFields() ActivityFields {
	return ActivityFields{
		PlayRole: in.PlayRole, Address: in.Address, Level: in.Level,
		Name: in.Name, HostName: in.HostName, CircleDate: in.CircleDate,
		TermName: in.TermName, Rank: in.Rank, ActivityName: in.ActivityName,
		SportsName: in.SportsName, TeamName: in.TeamName, OrgName: in.OrgName,
		ResultsName: in.ResultsName, ObtainTime: in.ObtainTime,
		SpecialtyTechnology: in.SpecialtyTechnology,
		LikeSpecialty1:      in.LikeSpecialty1, LikeSpecialty2: in.LikeSpecialty2,
		LikeSpecialty3: in.LikeSpecialty3, Hours: in.Hours,
		CircleBeginDate: in.CircleBeginDate, CircleEndDate: in.CircleEndDate,
		CheckResult: in.CheckResult, PatentType: in.PatentType, PatentNum: in.PatentNum,
	}
}

// Deprecated: 薄壳 Getter 转发聚合字段，仅供旧调用方；新代码走 GetActivityFields。
func (in TaskEditInput) GetPlayRole() string            { return in.PlayRole }
func (in TaskEditInput) GetAddress() string             { return in.Address }
func (in TaskEditInput) GetLevel() string               { return in.Level }
func (in TaskEditInput) GetName() string                { return in.Name }
func (in TaskEditInput) GetHostName() string            { return in.HostName }
func (in TaskEditInput) GetCircleDate() string          { return in.CircleDate }
func (in TaskEditInput) GetTermName() string            { return in.TermName }
func (in TaskEditInput) GetRank() string                { return in.Rank }
func (in TaskEditInput) GetActivityName() string        { return in.ActivityName }
func (in TaskEditInput) GetSportsName() string          { return in.SportsName }
func (in TaskEditInput) GetTeamName() string            { return in.TeamName }
func (in TaskEditInput) GetOrgName() string             { return in.OrgName }
func (in TaskEditInput) GetResultsName() string         { return in.ResultsName }
func (in TaskEditInput) GetObtainTime() string          { return in.ObtainTime }
func (in TaskEditInput) GetSpecialtyTechnology() string { return in.SpecialtyTechnology }
func (in TaskEditInput) GetLikeSpecialty1() string      { return in.LikeSpecialty1 }
func (in TaskEditInput) GetLikeSpecialty2() string      { return in.LikeSpecialty2 }
func (in TaskEditInput) GetLikeSpecialty3() string      { return in.LikeSpecialty3 }
func (in TaskEditInput) GetHours() string               { return in.Hours }
func (in TaskEditInput) GetCircleBeginDate() string     { return in.CircleBeginDate }
func (in TaskEditInput) GetCircleEndDate() string       { return in.CircleEndDate }
func (in TaskEditInput) GetCheckResult() string         { return in.CheckResult }
func (in TaskEditInput) GetPatentType() string          { return in.PatentType }
func (in TaskEditInput) GetPatentNum() string           { return in.PatentNum }

var (
	ErrTaskInputIDRequired      = taskInputError("id 为必填且必须 > 0")
	ErrTaskInputTaskIDRequired  = taskInputError("taskId 为必填且必须 > 0")
	ErrTaskInputContentRequired = taskInputError("content 为必填")
)

// submittedBlacklist 是「视为未提交」的状态关键词表。
//
// circleTaskStatus 是自由文本族（docs/README.md 表 C：「上传期/已结束 × 未提交/已提交」），
// 前端 managementLeftBottom.vue 同样以子串方式消费该字段；因此用 Contains 匹配，
// 兼容未来新增文案变体。注意：本判定只回答「是否已提交」；
// 「能否提交」（含「已结束」时禁止上传）属另一语义，调用方应单独判断。
var submittedBlacklist = []string{"未提交"}

func (t *Task) SetSubmittedByStatus() {
	// 空串保守视为未提交——平台不返回 status 字段时
	// 误判已提交会隐藏学生提交入口。
	if strings.TrimSpace(t.CircleTaskStatus) == "" {
		t.Submitted = false
		return
	}
	for _, kw := range submittedBlacklist {
		if strings.Contains(t.CircleTaskStatus, kw) {
			t.Submitted = false
			return
		}
	}
	t.Submitted = true
}

// SetNeedPicFromUpPic 用平台 upPic（int 0/1）推导 NeedPic。
//
// getCircleStatistics 只返回 upPic，不返回 needPic；encoding/json 不会把
// upPic 填进 NeedPic。调用方依赖 NeedPic 做"是否要求图片"时，必须在解码后调用本方法。
// FetchTasks 已内部调用；手工 DecodeDataList 的调用方才需显式调用。
func (t *Task) SetNeedPicFromUpPic() {
	if t == nil {
		return
	}
	t.NeedPic = t.UpPic > 0
}

type taskInputError string

func (e taskInputError) Error() string { return string(e) }
