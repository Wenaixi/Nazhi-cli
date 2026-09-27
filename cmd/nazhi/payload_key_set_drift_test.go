package main

// 这组测试锁定一条防漂移契约：CLI 写操作命令的 payload 允许键集，
// 是 pkg/types 对应类型的 json tag 的**手工镜像**，没有编译期绑定。
//
// 存在的理由：未知键拒绝（unknownUpdatePayloadKeys）只在「允许集里没有」时
// 才拦得住拼错的键名——而允许集是手写字符串。一旦 SDK 侧新增一个 json tag
// 字段而忘了同步 CLI 允许集，用户就会得到「未知键」参数错误，尽管该字段
// 完全合法；反过来 CLI 多写了键则放行一个服务端会忽略的键，无声失败。
// 历史上 SDK 新增 Birthday 字段时就手工同步过一次，正是这条漂移的现实证据。
//
// 断言只做**一个方向**：SDK 有而 CLI 允许集没有的键必须为空。
// 反方向（CLI 有而 SDK 没有）由显式白名单豁免，因为那里存在多组
// 有意差异（见 taskKeySetIntentionalExtras 与各组的 extras 白名单）。
// 之所以不做双向断言：无 tag 字段、id 这类编辑主键、以及 TaskInput 接口
// 消费但不出站的键，都让「CLI 允许集 ⊆ SDK tag 集」这条等式不成立，
// 硬凑双向只会把有意的设计差异伪装成缺陷。
//
// 反射约定（与 encoding/json 对齐，见 structJSONKeys）：
//   - 字段有 tag 时取 tag 首个逗号前的名字（`,omitempty` 等选项剥离）；
//   - 字段无 tag 时取 Go 字段名本身（encoding/json 也正是这样用字段名做键）；
//   - tag 名为 "-" 的字段不产出键（该字段不参与序列化）。
// omitempty 不影响键是否合法：它只决定零值时是否输出，不改变键名本身。

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// structJSONKeys 返回目标结构体参与 JSON 序列化的全部键名。
// 语义与 encoding/json 一致：tag 优先、无 tag 用字段名、"-" 表示不序列化。
func structJSONKeys(v any) []string {
	typ := reflect.TypeOf(v)
	keys := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		// 名为 "-" 的字段不参与序列化。注意不能写成整串 tag == "-"：
		// 那会漏掉 `json:"-,"` 这类「键名是 - 且带选项」的合法形态——
		// encoding/json 判的是逗号前的名字，不看整串。
		if name == "-" {
			continue
		}
		// tag 名为空（tag 整体为空、或只剩选项）时 encoding/json 用字段名。
		if name == "" {
			name = typ.Field(i).Name
		}
		keys = append(keys, name)
	}
	return keys
}

// missingFromAllowedKeys 返回「SDK 键集有、CLI 允许集无」的键（已按小写折叠比较）。
// CLI 允许集由 newPayloadKeySet 派生时统一小写存储，SDK tag 为驼峰原样，
// 故比较必须折叠大小写，否则 camelCase 键会被误判为缺失。
func missingFromAllowedKeys(sdkKeys []string, allowed map[string]struct{}) []string {
	var missing []string
	for _, k := range sdkKeys {
		if _, ok := allowed[strings.ToLower(k)]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// payloadKeySetDriftCase 描述一组「CLI 允许键集 ↔ SDK 类型 json tag」的对应关系。
type payloadKeySetDriftCase struct {
	// name 是子测试名，需能唯一标识该组键集。
	name string
	// allowed 是 CLI 侧的小写允许集（即各组 *Keys.allowed() 的结果）。
	allowed map[string]struct{}
	// mirror 是被镜像的 SDK 结构体，须以指针传入以便 reflect 取得类型。
	mirror any
	// extras 是白名单：CLI 允许但镜像结构体没有的键，每项须写明为何有意。
	extras []string
	// extrasReason 说明本组 extras 的整体成因，逐项理由写在 extras 旁边的注释里。
	extrasReason string
}

// taskKeySetIntentionalExtras 是 task 族允许集相对 TaskAddCirclePayload
// 多出的键，逐项理由如下——这三条都是**出站请求体里根本不存在**的键，
// 故意不进 wire struct，测试必须豁免否则会误报。
//
//   - taskId：TaskInput 接口的 GetTaskID 消费。它在 CLI 侧被 taskInputFieldAliases
//     改写为出站键 circleTaskId（见 task_payload_json.go 的别名表），
//     并由 buildTaskPayload 从 getCircleTypeByTaskId 拉任务元数据后填入。
//     TaskAddCirclePayload 只有 circleTaskId 一侧，没有 taskId 一侧。
//   - imagePaths：TaskInput 的 GetImagePaths 消费的是**本地图片路径**，
//     由 buildTaskPayload 逐个上传换成 attachmentId 后才汇入 pictureList；
//     本地路径绝不能原样出站（服务端不认本地文件系统路径），故 wire struct 无此键。
//   - imageIds：TaskInput 的 GetImageIDs 消费已上传图片的附件 ID 列表，
//     出站时汇入 pictureList。写法为 imageIds 而非 imageIDs 是等价的大小写变体：
//     TaskSubmitInput/TaskEditInput 的 ImageIDs 字段**没有 json tag**，
//     解码靠 encoding/json 的字段名大小写不敏感匹配，imageIds 与 imageIDs 同解。
var taskKeySetIntentionalExtras = []string{
	"taskId", "imagePaths", "imageIds",
}

// taskDeprecatedKeySetExtras 是 taskInputDeprecatedKeys 相对 TaskAddCirclePayload
// 多出的键。这七个键全部**确实带出站 tag**（name/hostName/circleDate/rank/level/
// termName），能通过反射覆盖到，故不构成白名单项；本表仅在自检用例里用于
// 断言「历史兼容字段确实是 TaskAddCirclePayload 的子集」，防止有人把
// 某个只在 CLI 侧存在的键误塞进历史兼容集合。
var taskDeprecatedKeySetExtras = []string{
	"id", "name", "hostName", "circleDate", "rank", "level", "termName",
}

// payloadKeySetDriftCases 覆盖全部带允许集的写操作命令。
// 新增走 writeOpRunner 的写操作命令时必须在此登记，否则会被下面的
// 基准数量断言拦下。
var payloadKeySetDriftCases = []payloadKeySetDriftCase{
	{
		// task submit / task edit / task preview 两个分支共用同一份允许集
		// （taskInputKeysAll），故按「键集身份」登记一次，不按命令名重复登记。
		name:         "task",
		allowed:      taskInputAllowedKeys,
		mirror:       types.TaskAddCirclePayload{},
		extras:       taskKeySetIntentionalExtras,
		extrasReason: "TaskInput 接口消费但不出站的输入键（任务 ID、本地图片路径、附件 ID 列表）",
	},
	{
		name:    "honorAdd",
		allowed: honorAddAllowedKeys,
		mirror:  types.AddHonorPayload{},
		extras:  nil,
	},
	{
		name:    "honorUpdate",
		allowed: honorUpdateAllowedKeys,
		mirror:  types.AddHonorPayload{},
		// update 走 map[string]any 入口（UpdateHonor 收 map，无 struct tag 可反射），
		// 键集即「AddHonorPayload 出站键 + 编辑记录主键」；id 由 update 路径单独
		// 做正数校验（honorUpdateWriteOp.validateID），不属 AddHonorPayload。
		extras:       []string{"id"},
		extrasReason: "map 入口的编辑记录主键，由 update 路径单独校验，不进 AddHonorPayload",
	},
	{
		name:    "typicalCaseSubmit",
		allowed: typicalCaseAddAllowedKeys,
		mirror:  types.AddTypicalCasePayload{},
		extras:  nil,
	},
	{
		name:    "typicalCaseUpdate",
		allowed: typicalCaseUpdateAllowedKeys,
		mirror:  types.AddTypicalCasePayload{},
		// 与 honorUpdate 同因：map 入口，键集为「出站键 + 编辑记录主键」。
		extras:       []string{"id"},
		extrasReason: "map 入口的编辑记录主键，由 update 路径单独校验，不进 AddTypicalCasePayload",
	},
	{
		name:    "userUpdate",
		allowed: userUpdateAllowedKeys,
		mirror:  types.UserUpdateInput{},
		// userUpdateKeys 对齐的是**用户书写层**（types.UserUpdateInput 的 json tag），
		// 而非出站 wire 层——后者是 client.UpdateMyInfoStructured 内部 remap 出来的
		// 数字码键（gender/youthLeagueFlag/nation/idType，另有 studentName），
		// 两者是两层不同语义，不可混为一谈。判断依据有三，均可在源码核实：
		//  1. CLI 的 user update 走 UpdateMyInfoStructured（写 user_update.go 的
		//     userUpdateWriteOp.call），该方法签名收 types.UserUpdateInput 而非 map；
		//  2. 允许集里的 genderName/nationName/idCardType 等键与 UserUpdateInput 的
		//     json tag 逐字一致，而与 wire 键 gender/nation/idType 不同名，证明对齐前者；
		//  3. 允许集含 nationalStudentNumber，而 UpdateMyInfoStructured 故意不写入
		//     该键（前端只读，防误改学籍）——只有对齐输入 struct 才需要「允许传入
		//     但被忽略」这个语义。
		// 换言之 wire 层键不在本组允许集内是**正确**的：CLI 接受的是用户书写形态，
		// 出站前的 remap 由 SDK 负责，用户写 gender 会因未知键被拒。
		extras:       nil,
		extrasReason: "对齐 UserUpdateInput 输入层 tag，wire 数字码键由 SDK 内部 remap，不属 CLI 允许集",
	},
}

// TestPayloadAllowedKeys_CoverSDKStructTags 断言 CLI 允许键集是 SDK 类型
// json tag 的超集：SDK 侧新增字段而忘了同步 CLI 允许集时，本测试必须变红。
func TestPayloadAllowedKeys_CoverSDKStructTags(t *testing.T) {
	for _, tc := range payloadKeySetDriftCases {
		t.Run(tc.name, func(t *testing.T) {
			missing := missingFromAllowedKeys(structJSONKeys(tc.mirror), tc.allowed)
			if len(missing) > 0 {
				t.Errorf("SDK 类型新增了 json 键但 CLI 允许集未同步，允许集已落后于 SDK：%v\n"+
					"请在对应的 %s 键集声明里补上这些键（用户书写形态，与 json tag 逐字一致）",
					missing, tc.name)
			}
		})
	}
}

// TestPayloadAllowedKeys_ExtrasAreDeclaredAndUsed 白名单必须是真实存在且真实
// 被豁免的：既不能声明一个用不到的项（否则白名单会悄悄失真），
// 也不能漏掉一个实际存在的差异（否则上面的主断言会红）。
func TestPayloadAllowedKeys_ExtrasAreDeclaredAndUsed(t *testing.T) {
	for _, tc := range payloadKeySetDriftCases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.extras) > 0 && tc.extrasReason == "" {
				t.Error("声明了白名单项却没写明为何有意（extrasReason 为空）")
			}
			declared := make(map[string]struct{}, len(tc.extras))
			for _, e := range tc.extras {
				if _, dup := declared[strings.ToLower(e)]; dup {
					t.Errorf("白名单项重复: %s", e)
				}
				declared[strings.ToLower(e)] = struct{}{}
			}

			// 镜像结构体没有、但 CLI 允许集里有的键，必须逐个落在白名单里。
			sdkKeys := make(map[string]struct{})
			for _, k := range structJSONKeys(tc.mirror) {
				sdkKeys[strings.ToLower(k)] = struct{}{}
			}
			var undeclared []string
			for key := range tc.allowed {
				if _, ok := sdkKeys[key]; !ok {
					if _, ok := declared[key]; !ok {
						undeclared = append(undeclared, key)
					}
				}
			}
			sort.Strings(undeclared)
			if len(undeclared) > 0 {
				t.Errorf("CLI 允许集有、SDK 镜像类型无的键未声明为有意差异：%v\n"+
					"确有有意的差异请补进该组白名单并写明理由；否则说明允许集写错了键名",
					undeclared)
			}
		})
	}
}

// TestPayloadAllowedKeys_DriftCasesCoverAllWriteOpModes 兜底：登记表必须覆盖
// 全部带允许集的写操作命令，防止新增命令漏登记而使主断言静默失效。
// 判定标准是「各命令实际用的键集是否都在登记表里」，而不是硬编码数量。
// task submit 与 task edit 共用一份 taskInputKeysAll，故多个命令可映射到
// 同一个登记项；反之登记表里出现无人使用的键集同样报出，防登记项失效。
func TestPayloadAllowedKeys_DriftCasesCoverAllWriteOpModes(t *testing.T) {
	// 每个带允许集的写操作命令，及其实际使用的键集身份。
	// 键集身份取自各 *Keys 变量的名字；task 族三命令共享一份。
	modes := map[string]string{
		"taskSubmit":        "task",
		"taskEdit":          "task",
		"taskPreviewSubmit": "task",
		"taskPreviewEdit":   "task",
		"honorAdd":          "honorAdd",
		"honorUpdate":       "honorUpdate",
		"typicalCaseSubmit": "typicalCaseSubmit",
		"typicalCaseUpdate": "typicalCaseUpdate",
		"userUpdate":        "userUpdate",
	}
	registered := make(map[string]struct{}, len(payloadKeySetDriftCases))
	for _, tc := range payloadKeySetDriftCases {
		registered[tc.name] = struct{}{}
	}
	for cmd, set := range modes {
		if _, ok := registered[set]; !ok {
			t.Errorf("写操作命令 %s 使用的键集 %q 未登记进 payloadKeySetDriftCases，"+
				"SDK 新增字段时它将不受防漂移守卫保护", cmd, set)
		}
	}
	for name := range registered {
		used := false
		for _, set := range modes {
			if set == name {
				used = true
				break
			}
		}
		if !used {
			t.Errorf("登记表里的 %s 不对应任何写操作命令实际使用的键集，登记项已失效", name)
		}
	}
	if len(payloadKeySetDriftCases) == 0 {
		t.Fatal("登记表为空，防漂移守卫整体空转")
	}
}

// TestPayloadAllowedKeys_TaskDeprecatedKeysAreSubsetOfWireTags 历史兼容字段
// 同样是 CLI 侧的输入键，必须是 TaskAddCirclePayload 出站 tag 的子集——
// 它们的存在理由就是「前端 form 有这些键」，而不是「SDK 另有一份定义」。
// 断言其对出站 tag 无超出，防止把 CLI 独有的键误塞进历史兼容集合。
func TestPayloadAllowedKeys_TaskDeprecatedKeysAreSubsetOfWireTags(t *testing.T) {
	sdkKeys := make(map[string]struct{})
	for _, k := range structJSONKeys(types.TaskAddCirclePayload{}) {
		sdkKeys[strings.ToLower(k)] = struct{}{}
	}
	for _, key := range taskDeprecatedKeySetExtras {
		if _, ok := sdkKeys[strings.ToLower(key)]; !ok {
			t.Errorf("历史兼容键 %s 不在 TaskAddCirclePayload 的出站 json tag 中，"+
				"它不是 wire tag 镜像，理由需重新审视", key)
		}
	}
	// 双向确认：这些键确实已进入 taskInputKeysAll 合并后的允许集。
	for _, key := range taskDeprecatedKeySetExtras {
		if _, ok := taskInputAllowedKeys[strings.ToLower(key)]; !ok {
			t.Errorf("历史兼容键 %s 未合并进 task 族允许集", key)
		}
	}
}

// TestStructJSONKeys_ReflectsEncodingJSONSemantics 校验 structJSONKeys 的反射
// 约定本身：tag 取逗号前名字、omitempty 被剥离、无 tag 用字段名、tag 为 "-"
// 的字段不产出键。这条守护的是上面所有断言的共同基础——若它恒绿失效，
// 其余用例的「无差异」结论将不可信。
func TestStructJSONKeys_ReflectsEncodingJSONSemantics(t *testing.T) {
	// 两个 "-" 形态都不产出键：整串为 `json:"-"` 显然如此；
	// `json:"-,"` 则是「键名为 - 且无选项」，encoding/json 判的正是
	// 逗号前的名字，故同样不序列化——这是本辅助函数最容易写错的一处。
	type probe struct {
		Tagged     string `json:"taggedKey"`
		OmitTagged string `json:"omitted,omitempty"`
		MultiOpt   string `json:"multiOpt,string,omitempty"`
		NoTag      string
		NoOptOnly  string `json:",omitempty"`
		Skipped    string `json:"-"`
		SkippedDef string `json:"-,"`
	}
	keys := structJSONKeys(probe{})
	// NoOptOnly 的 tag 只剩选项、键名为空，按字段名兜底。
	want := map[string]struct{}{
		"taggedKey": {},
		"omitted":   {},
		"multiOpt":  {},
		"NoTag":     {},
		"NoOptOnly": {},
	}
	if len(keys) != len(want) {
		t.Fatalf("键数量异常: 得到 %v, 期望 %d 个", keys, len(want))
	}
	for _, k := range keys {
		if _, ok := want[k]; !ok {
			t.Errorf("不该产出的键: %s", k)
		}
	}
	// 键名不得被逗号后的选项污染。
	for _, k := range keys {
		if strings.Contains(k, ",") {
			t.Errorf("键名含逗号选项，未剥离: %s", k)
		}
	}
}
