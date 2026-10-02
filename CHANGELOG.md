# CHANGELOG

## [Unreleased]

## 第十二轮架构评审的证伪与落地

本轮由 `improve-codebase-architecture` 驱动（全部模块、深度对照真实前端源码 69 个 .vue），
7 条候选经 4 路独立反方 critic 逐条尝试推翻。**3 条推翻、3 条部分成立收窄、1 条标 Speculative**，
最终落地 4 项零行为变更的收口。三条被证伪的候选已记入 CLAUDE.md「已证伪误报与已知挂账」，
复发需新证据。

### 落地（四项，均不改线协议行为、不改公开 SDK 签名、不改 CLI 输出契约）

- **业务响应解析失败包装收为单点**：
  - `doBizAndDecode` 原内联一句与 `decodeOrInvalidResponse` 逐字相同的 `fmt.Errorf`，
    两处包裹的 error 同来自 `types.DecodeResponse`、哨兵与 `opName` 同参。
  - 现改走 helper，错误文案与 `errors.Is` 链逐字不变。原「保持各自内联以便日志
    上下文就地可读」的理由不成立——helper 本身就收 `opName`。
  - 新增守卫 `TestDecodeResponseSingleScaffold`：按骨架计数锁定 `types.DecodeResponse`
    裸调用面为 1。新增裸调用会绕过 `ErrInvalidResponse` 哨兵，使 CLI 把「200 但响应体
    不可信」判成 default 500/exit2，与 `httpDo`/`doBizGet` 的 502/exit2 不一致。
- **错误分类去掉文件内同款重复**：
  - `ParallelDims` 末尾的五分类三分支 switch 改为 `isContextError` 二分，与同文件
    `classifyDimErrors` 同口径。原 switch 中 `NetworkTimeout` 与 `BusinessError` 两分支
    循环体逐字相同、`default` 又与它们相同，三分支实为两桶。
  - 新增 `TestParallelDims_ErrorClassification`：三类错误落桶的期望值为独立字面量，
    不写成对 `ClassifyError` 的二次映射（否则断言与被测实现同源、恒绿）。
  - `ClassifyError` 与五个 `CategoryXxx` 常量保持导出不变（属已交付的公开 SDK 能力）。
- **写实互动三命令控制流收进读命令 runner**：
  - `circle delete` / `circle like` / `circle comment` 三者的 Run 体前 15 行逐字相同
    （读 `--id` → 判空 → `ParseInt` → 判正 → 建客户端 → 进度文案 → 调 SDK → 输出），
    现三处改走 `runReadOp`，`readOpMode` 零新钩子即表达。
  - 新建 `cmd/nazhi/flag_id.go` 收口四处 `--id` 判据（三个写实命令 + 典型案例 delete），
    两句错误文案逐字相同。「先校后建」次序不变：缺 `--token` 与 `--id` 非法同时发生
    时用户先看到真正该修的那个错。
  - 新增占位类型 `emptyPayload` 承载「成功但无业务负载」：runner 的
    `normalizeEmptyList` 会把 nil 归一成空数组，`success` 闭包无法再用 `result == nil`
    区分「删除成功」与「评论成功但服务端未返回 `returnData`」。
  - 新增守卫 `TestCircleCommands_DelegatedToRunReadOp`：三文件不得直接出现
    `buildBizClient(`，控制流归属由骨架计数锁定。
- **典型案例编号码表移入类型包并新增域内查表命令**：
  - 三张码表（type / role / level）从 `pkg/client/typical_case.go` 迁入
    `pkg/types/typical_case_codes.go`，与写实三表同址。此前它们未导出，
    CLI 无法枚举，用户无从核对 payload 里 `type`/`role`/`level` 的合法取值。
  - 新增 `nazhi typical-case level-codes` 纯本地查表命令（不注册业务 flag、
    不需要 token）。**刻意不并入 `nazhi task level-codes`**：典型案例 level 是
    「获奖级别」（1 国际 … 5 学校），写实 level 是「行政层级」（1 国家 … 6 年段），
    编号重叠处语义不同，并入同一信封正是 CLAUDE.md 契约 C 明令禁止的混用入口。
  - 新增三个守卫：`TestTypicalCaseCodes_CommandRegistered`（命令已挂树）、
    `TestTypicalCaseCodes_GroupValues`（码值以独立字面量逐项核对）、
    `TestTypicalCaseCodes_SurvivesTableTampering`（变异验证：篡改码表后比较逻辑
    必须敏感，杜绝恒绿假通过）。

### 证伪（不落地，结论已入 CLAUDE.md）

- **写实列表翻页合并为一个泛型内核**：limit 路径走 `derivePageBoundsUnclamped` +
  `limitEndPage` 的「先收敛后判超限→退回首页」，全量路径走 `derivePageBounds` 的
  「先钳制后直接用」，收敛次序语义相反且各有测试锁定；`collectDims` 的三个 opt 字段
  在翻页侧一个都用不上，可复用的只有 errgroup 加落槽十几行。
- **五个取数方法改建模类型**：非测试代码里只有 5 个调用点且全是一行透传，
  建模换不到类型安全收益，却把「字段歪了照样输出」换成 `DecodeDataList` 整页全灭；
  而这些端点由服务端按十四种活动类型异构驱动。改签名同时是 BREAKING。
- **典型案例码表并入 `nazhi task level-codes`**：理由见上方「刻意不并入」。
- **CLI runner 全量收口**：`whoami` 的 `ErrEmptyUserInfo` 分支与 `task list` 的
  partial 分支要求「SDK 失败但改走信封」，`runReadOp` 的 `fetch` 返回 `(any, error)`
  形状在 err 分支永不到达 `success`；`file upload`/`file download` 走
  `buildClient(urlType)` 且不注册 `--token`，要表达需再注入客户端构造钩子。
- **`ErrorCategory` 降私有 / `String()` 删除**：`ErrorCategory` 与常量是 CHANGELOG 已
  交付的公开 SDK 能力，降私有是 BREAKING；`String()` 虽零调用点，但删除会让下游
  `%v` 输出由分类名退化为数字，属有意降级而非白拿。仅采纳 switch 简化。
## [1.9.0] - 2026-10-02

发布链接：[v1.9.0](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.9.0)

### 新增

- **任务三级层级深模块收敛（Task Hierarchy Deepening）**：
  - SDK 层面：`pkg/client/task.go` 深度统一任务三级选择模型（维度→类别→任务），新增 `GetTaskCategories` 与 `GetTaskItems`，旧方法 `GetCircleTypes` 与 `GetCircleTasks` 保留为兼容别名；
  - 补齐前端唯一真实业务缺口：新增 `GetRecentlyCircleTask`（获取学生近期填报任务，对应 `/api/studentCircleNew/getRecentlyCircleTask`，来自 `mainLeft.vue:73`）；
  - CLI 层面：`cmd/nazhi/task_metadata.go` 新增 `nazhi task categories`（别名 `types`）、`task items`（别名 `tasks`）、`task recent` 子命令。旧 `nazhi circle types/tasks` 保持透明兼容。
- **CLI 写操作伪泛型消除与代码收敛**：
  - `cmd/nazhi/write_op_runner.go` 中的 `taskApplyAddressLevelFlags` 从带动态断言的伪泛型重构为类型 switch 直传，消除 4 处逐字重复的匿名闭包。

### 优化与治理

- **CLAUDE.md 核心记忆库深度精简**：
  - 文件体积由 77,045 字节精简至 15,339 字节（缩减 80%），彻底根除系统提示词预算截断告警；
  - 100% 完整保留凭据、69 个前端 Vue 组件业务映射、A–H 核心契约与已证伪假阳性清单。
- **文档与术语对齐**：
  - 同步更新 `CONTEXT.md` 任务三级层级与最近任务术语；
  - 同步更新 `docs/README.md` 功能↔Go源码↔前端源码三方对照表。

## 第十一轮架构评审的落地修复

本轮对全部模块做架构深化评审（8 路只读侦察 + 4 路反方 critic + 主代理逐条亲读 + 探针与变异验证）。8 条候选进入复核，**1 条定级为已复现缺陷、3 条坐实为清理项、4 条被反方推翻**——这是本项目历史上第一次反方推翻率高于成立率。

### 修复

- **诊断摘要存在第八处出口，绕过脱敏安全接缝致明文进入日志**。`httpDo` 的响应体日志出口写的是 `logx.RedactBodyThenTruncate(respBytes, 100)`：自带字面量长度，且跳过了 `RedactSnippet` 内部的 `clipPrefixWindow`。当响应体自身在敏感值中间被截断时（服务端或反向代理超时是常见来源），脱敏正则因缺闭合引号整体失配，明文直接进入日志。
  - 可达性比初判更广：`levelForStatus` 对 4xx 映射到警告级、5xx 映射到错误级，而命令行默认日志级别即为警告，故**默认配置下每次错误响应都会走到这一行**——恰恰 4xx/5xx 是响应体被 WAF 或网关截断的形态。输出通道为标准错误与 `--log-file` 指定的日志文件，后者常被持续化留存。
  - 修复为一处改动：改走 `logx.RedactSnippet`，长度与脱敏次序由该函数单点持有。此前仓库记忆记载「七处调用点已全部收口」，该记载因这一处而失实，修复后才真正成立。

### 变更

- **`Client.LogDebugForTest` 改为委托内部实现**：原实现与私有的携带 context 版调试入口函数体逐字相同，而后者有 11 处生产消费。同一份知识的两份副本必然漂移，现收口为单一实现——与第十轮把 `Enabled` 改为委托内部判定同属一个模式。
- **删除 `Client.Close` 里的死分支**：函数内的错误切片恒为 nil，聚合分支不可达。`git show a97d07d` 证实成因——v1.7.0 移除验证码识别器时带走了唯一的错误追加点，聚合分支作为消费者被留下。签名保留 `error` 不变（调用方按「可能失败」聚合而设计，公开 SDK 不宜因实现细节变更签名），函数体现在恒返回 nil，随之未使用的 `errors` 导入一并删除。

### 测试

- **新增「诊断摘要一律经 `RedactSnippet`」的语法树守卫**，扫描 `pkg/client` 与 `cmd/nazhi` 的非测试文件，禁止直调 `logx.RedactBodyThenTruncate`。本项目已有三次同类纪律复发（限读两处、哨兵两处、摘要一处），全部靠人工审查发现；本守卫把它变成结构性约束。正向断言逐个扫描根校验，确保任一根失效时报错而非静默全绿。
- **新增端到端泄漏回归测试**：以真实 HTTP 往返覆盖三种状态码（200/401/500）× 敏感值被截断的形态，直接断言日志中不含明文。已做双向变异验证——恢复旧写法后三个子用例全部变红。
- **两条恒绿守卫改为真实断言**。分页合并的容量钳制测试此前名为验证「容量上界不能被击穿」，实际只断言「函数不 panic 且结果非空」；另一条的注释承诺验证编排路径，函数体却只测纯算术函数。变异验证确认：删掉两道容量闸后二者与整个包全部通过。
  - 容量计算提取为纯函数后可直接断言其返回值。提取引入的新缺口（断言锁住辅助函数、没锁住调用关系）由新增的语法树守卫补上——变异验证确认两道闸被彻底绕过时全包变红。
  - 字节闸子测试改为全部页共用同一切片：容量计算只读各页长度，求和结果完全等价，而堆增量从约 396MiB 降到 4MiB（持续集成环境带竞态检测运行，影子内存会再放大 2-4 倍）。

### 已证伪（不再重提）

- **三个更新入口不是浅接口，改为强类型会造成数据丢失**。`AddHonorPayload` 八个字段中仅两个带 `omitempty`，若改为结构体，用户只写两个键只想改一个字段时，未提供的六个字段会以零值发出，服务端据全量更新即清空它们。map 在此承载的是**部分更新语义**，结构体表达不了它。
- **`pkg/envelope` 的退出码语义并未泄漏**：该包零外部导入，公开接口面只有 `pkg/client` 与 `pkg/types`；档位决策的单点在命令层的哨兵漏斗，纯映射函数结构上不可能知道哪个编码是永久性错误。


## 第十轮架构评审的落地修复

本轮对图片处理链、CLI 判定与测试可信度做全量架构评审，8 条候选经反方与 verifier 独立核实后 **7 条推翻、7 项落地**。8 条候选中仅 1 条构成用户可见的缺陷修复，其余为守卫建设与文案订正。

### 修复

- **图片编码失败被误报为服务端故障（退出码错档）**。`prepareImageForUpload` 的三处编码失败出口（q92 起步、q80 降档、缩放级联末档）返回的错误不含任何 `client.Err*` 哨兵，穿过上传路径的透传包装后，CLI 哨兵漏斗的全部 `errors.Is` 判定均不命中，落入兜底分支被映射为 HTTP 500 与**退出码 2（服务端或网络故障，可退避重放）**。
  - 触发门槛极低：标准库 `image/jpeg` 对任一边长 ≥ 65536 像素直接拒绝编码，而 PNG 解码器没有尺寸上限，故此类图片解码成功却在压缩环节失败。实测一张 **1186 字节**的 70000×4 像素 PNG 即可触发，远低于上传路径 200MB 的体积预检线（预检拦不住）。
  - 这是调用方可控的本地问题（换一张图即可，重试永远不会成功），此前却被告知「服务端故障、建议重试」，脚本会对永不成功的请求无限退避重放。
  - 三处出口统一携带 `ErrInvalidPayload`，与既有的解码失败、附件读取、目标文件创建处置口径一致。修复前退出码 2 / 编码 500，修复后退出码 3 / 编码 400。
- **图片预处理的两个哨兵此前未登记进 CLI 漏斗**。`ErrImageTooLarge`（压缩后仍超限）与 `ErrUnsupportedFormat`（不支持的格式，部分 BMP 变体）同样落兜底分支报 500 / 退出码 2。现与参数类错误同归 400 / 退出码 3。
- **枚举映射的拒绝文案补回合法值清单**。`nazhi user update --payload` 传入未登记的中文枚举值时，错误只说「不支持的民族值 "汉"」，而 `--help` 与允许键清单都只列键名、不列取值域——用户与脚本（含 AI 代理）无从得知应填什么。现由映射表派生清单随文案给出，四处口径一致。
  - 该提示在 `b58f5db`（switch 改 map 的重构）中被删除且未在别处补偿，属回归而非新增。
- **`--timeout` 的告警不再对用户谎报**。非法值告警原文案硬编码「使用默认 15 秒超时」，而 `nazhi file upload` / `file download` 的 flag 注册默认值是 30——`--help` 承诺 `(default 30)`，告警却说 15。改为从该命令自身的 flag 注册默认值派生，文案与实际生效值同源，不可能脱节。

### 变更

- **删除 `rawDoWithResp`**：它是 `do` 的零逻辑别名（函数体仅一行委托 + 错误早退，签名逐字相同）。删除的实质收益是把「调用方负责关闭响应体」这条安全契约上移到真正被调用的 `do`——原契约写在一个三个消费方都不再经过的层上。该符号私有且无测试引用，不构成公开 API 破坏。
- **`Client.Enabled` 改为委托内部实现**：原实现与内部判定逻辑是同一份知识的两份副本，且漏掉了后者的 nil context 归一，使 context 为 nil 时「级别守卫通过」与「实际会输出」不等价。保留导出形态（跨包测试需要），收口实现。

### 测试

- **透明图片类型表的完备性守卫**。该枚举表**已漏过一次**（补 NYCbCrA 分支的修复随 v1.5.1 之前的版本上线），而全仓对它的断言至今仅一条单点测试。守卫以**运行时解码真实字节**判定（而非扫描 import 块推导格式清单——后者会漏掉经传递依赖自注册的 TIFF），含三条互相独立的断言：枚举集非空且覆盖期望集、样本解码出的带 alpha 类型必须被命中、期望集与运行时观察集双向吻合。
  - 新增 `testdata/lossy-alpha.webp`（有损 VP8+ALPH，190 字节）：Go 无 WebP 编码器，该样本是覆盖有损 WebP 路径的唯一来源。
- **哨兵覆盖守卫的扫描范围扩展到整个包**。原守卫只从 `errors.go` 提取哨兵，而 `image_prep.go` 另声明了两个导出哨兵，故它们从未进入检查。
- **两条缩放级联守卫改为断言性质**。其一断言「不存在对某历史标识符的 range 循环」，而该标识符在生产代码中出现 0 次——注入语义等价但异名的循环会静默通过；其二用固定字符窗口做子串匹配，窗口内 `return` 出现三次、三条断言中两条恒真。现改为按语法树定位并断言真实性质。
- **大图压缩测试的夹具此前从未进入压缩路径**。原夹具用的是高压缩率的渐变图，实测产出约为体积上限的 1.2%，连闸门都碰不到（测试自身打印的「压缩率 1768.7%」即是自证）。改用固定种子随机噪点，覆盖「质量降档达标」与「进入缩放级联」两条路径，断言用**输出像素尺寸**而非字节数——后者在多条路径下同样成立，无法区分。
- **枚举映射表的码值锁定**。期望值使用独立字面量而非由被测表派生，否则两者一起漂移时断言恒成立。危险形态不是「新增一项」（未命中即拒绝，是安全失败），而是「改错已有码值」——例如某民族代码误改，会把该民族用户静默存成另一民族，属无声的数据损坏。附正向锚点断言，防止表被整体清空后逐项比对静默通过。
- **修复一条恒为真的断言**：`nazhi user update` 的非法枚举值测试把三条断言包在「退出码为 0 且 stderr 不含 error」的条件里，而正常路径退出码非 0 使外层恒假，内层文案断言对任何输出都通过。

### 文档

- **透明类型表与图片解码的注释订正**：原注释把一次已完成的重构（「将某类型合并进 type switch」）写成契约，读者会误以为类型表已完备；解码函数的注释称使用标准库接口（实为第三方库）且把格式枚举为四种（漏了已在线可达的 TIFF）。
- **压缩降档序列的注释订正**：原注释把**条件链写成无条件链**，漏掉三处守卫（超大图整档跳过质量降档、两档缩放各有最小边长要求）以及一个更早的编码失败出口。
- **`ErrInvalidResponse` 的语义边界订正**：其注释声明为「HTTP 非 200」，而实际用法远不止于此（响应体超限、2xx 但响应体非预期结构、重定向违规、0 字节响应、附件超限）。真实语义是「响应不可用或不可信、重试无意义」，哨兵文本同步调整。


## 第九轮架构评审的落地修复

本轮对写实取数、上传、CLI 判定与文件归属注释做全量架构评审，5 条候选经反方核实后 4 条推翻、1 条成立。此处只记落地部分。

### 修复

- **写实结构化路径的条数闸只守预分配容量、不守最终条数**。`fetchAllCirclePages` 的容量早退分支按服务端声明的 `totalNum` 判定，而翻页页数取 `max(totalPage, ceil(totalNum/pageSize))`——服务端 count 与 list 不自洽（`totalPage` 虚高）时，满页累加的记录条数能数倍越过 `maxSubmittedRecords`，而该形态对早退分支不可见（声明的 `totalNum` 并未超闸）。合并循环此前无任何上界。seam 放在合并循环处，与透传路径的字节预算复核同层：整页粒度回退到已合并的合法前缀并告警，不静默截断、不丢弃已拉取数据。容量早退分支保持不变。
  - 触发形态是分页声明失配，而 `totalPage` 虚高是既有测试明确接受的合法输入（页数下界取 `max` 即为此设立）。实测夹具下可合并出闸值两倍的记录数，堆占用显著上升；服务端声明自洽时行为逐字未变。
  - 该闸的现有测试此前恰好绕开缺口：用超闸的 `totalNum` 触发早退分支，从不触及累积环节。

### 变更

- **`task list` 的部分失败判定移除两个恒不可满足的哨兵**。`ErrEmptyUserInfo` 与 `ErrSessionBackoff` 在错误链上确实可达（激活链步骤 4 与会话冷却窗口），但该命令的部分失败分支要求 `len(tasks) > 0`，而预热或维度拉取阶段失败时 `tasks` 恒为 nil，故这两个条件永不相遇。改前改后对五种场景的真实命令执行逐字节比对一致。`ErrBusinessRejected` 与 `ErrRetryable` 在非空结果下确实可达，判定保留。

### 文档

- **上传文案与附件白名单真相源脱钩**。附件白名单此前有四份副本：SDK map（真相源）、SDK godoc、`nazhi file upload` 的 `Long`、`--file` 的 usage。SDK godoc 漏了 PDF；`--file` 的 usage 则把图片格式与附件白名单并列为一张「支持的扩展名」清单，而实际分派是「扩展名命中附件白名单则直传，否则一律按图片解码」——该措辞在两个方向上都不准。两处均改为陈述判据而非枚举：图片格式不是白名单，而是解码器注册表，任何枚举都必然不完整。
  - `Long` 顺带按两条路径分句，各带自己的体积上限——图片限 5MB、附件限 20MB，原措辞易被读成图片也限 20MB。
  - 未从 SDK 导出白名单：它是无序集合，导出会把顺序问题推给调用方，属破坏性面扩张；而守卫从源码提取即可拿到同一份集合。
- **三处失实的文件归属注释订正**：其中一处是分页常量注释称「已上提 honor.go」，实际定义在组装层公共文件，且是该次迁移留下的过期残留。另有一处描述上传附件 ID 的解码路径，而该手写回落链早已收为统一的数值归一入口，照原文读会以为生产仍走有缺陷的旧路径。
- **`sessionManager` 锁契约注释订正**：`StoreToken` 的注释称其持锁写 token，而 token 存于原子值、本不需锁；`clearBackoff` 的注释称仅在持锁路径调用，却被无锁的 `StoreToken` 调用。改为陈述真实契约并列出三类调用方各自的锁语义。零行为变化。

### 测试

- **上传文案守卫**（从 SDK 源码提取附件白名单，断言 CLI 两处文案覆盖全项且不枚举图片格式）与**解码器注册守卫**（断言 SDK godoc 的格式枚举等于 `image_prep.go` 真实导入的解码器集合）。后者修的是前者覆盖不到的缺口：SDK 侧那份枚举同样无人看守。
  - 解码器提取的判据来自实测：标准库格式由包初始化自注册（普通导入即可），扩展库格式须空导入；混用单一判据会漏项或把 `image` 包的辅助子包误计为格式。


## 编号对照表与承担角色映射

写实域的「数字→中文名」此前只有两个零生产消费的 SDK 函数，CLI 侧完全没有暴露：脚本从 `nazhi task list` 拿到 `"level":"5"` 时，无任何命令能把它翻成「校」。本段把三组编号语义收为单点并对外暴露。

### 新增

- **`nazhi task level-codes`**：输出写实域三组编号字段（等级 `level`、审核情况 `checkResult`、承担角色 `playRole`）的取值对照表。纯本地查表——不需要 `--token`、不访问网络、不注册任何业务 flag，供脚本与人工离线速查。
- **SDK `types.PlayRoleName`**：承担角色码到展示名的映射，此前是三组里唯一只有常量、没有名称函数的缺口。
- **SDK 三张可枚举对照表** `types.TaskLevelNames` / `types.CheckResultNames` / `types.PlayRoleNames`：三组编号语义的唯一真相源，CLI 查表命令直接引用，不另抄一份。

### 重构

- `types.TaskLevelName` 与 `types.CheckResultName` 的实现由 switch 改为查表。**签名与返回值逐字不变**（既有穷举测试逐码锁定，构成兼容性保护网），未知码仍返回空串——该契约改由 Go map 的零值语义保证。

### 文档

- `task submit` / `task edit` / `task preview` 的 `--level` 说明均改为由对照表派生。`submit` 与 `edit` 此前各自硬编码「4=区县」，而 SDK 表内是「区/县/街道/社区」——用户照提示理解的值与脚本从 SDK 取到的名称对不上；`preview` 同样能覆盖该字段却完全没有码表，用户无从得知可选值。三处现在共用同一个由表派生的码表串，改表即改文案。`preview` 保留其特有的覆盖语义措辞（它不提交，只覆盖 payload 字段），码表追加在语义前缀之后。
- `docs/README.md` 表 C 补入三组编号的取值与前端判定处，与既有的 `记录 type` / `记录 status` 等同类信息并列；并加脚注说明表内字段名是平台响应侧的 snake_case 键，而查表命令输出的分组键是 camelCase，两者指同一组字段。

### 修复

- **查表命令的码序断言曾是恒等断言**：原断言把 `taskLevelCodeList()` 的返回值同时当作期望与比对对象，期望与被测函数同源，排序一旦出错两边一起错，断言恒成立。反序注入实测过该盲区，现改为独立字面量期望。
- **`--level` 用例测试只遍历硬编码的两个命令**，新增同族命令时不会进入检查范围——`task preview` 正是因此漏掉了派生而无人发现。现改为从 `task` 命令树反查所有声明了该 flag 的子命令。

### 与既有命令的关系

- `nazhi task level-codes` 是**离线速查**（零凭据零网络），`nazhi circle dict --cate-code 23` 读**服务端字典接口**用于核对平台当前实际字典内容。两者是同一份 level 知识的两个来源，查表命令的 `--help` 已写明分工，并声明不一致时以服务端为准。

### 已知边界

- 三张表的键由 `encoding/json` 按**字符串序**序列化，`--level` 的 usage 文案用 `sort.Strings` 排序。当前键只有 `"1"`..`"6"`，字符串序与数值序恰好相同故用户看到自然顺序；若平台日后出现两位数编号（如 `"10"`），两处的码序会同时变得不自然，届时需改为数值排序。已在源码注释写明该边界与失效条件。
- 典型案例域另有一套 `level` 码表（1 国际 / 2 省 / 3 市 / 4 区县 / 5 学校，定义在 `pkg/client` 内），与写实的 `level` 语义不同。两表**严禁合并**，已在 `TaskLevelNames` 的注释中显式警示。


## 第八轮架构评审（5 路反方证伪 + 变异验证）

本轮**落地 4 项、证伪 2 项、1 项降级为只订正注释**。方法上首次改为「派子代理做**反方**（任务是推翻候选而非找证据）+ 变异验证双保险」——反方推翻了两条候选、否决了我最初推荐的修复方向，并自行发现了守卫方案的盲区。全部结论与依据见 CLAUDE.md 第 I 节。

### 修复

- **维度闸归位：闸按真实维度数计数**（`pkg/client/parallel.go` / `pkg/client/task.go`）。`collectDimsOpts.maxDims` 零值「不钳制」是并发内核的显式契约，但导出的 `ParallelDims` 恰传零值，使维度上界的防护被移到调用点之外；同时 `FetchTasks` 的调用点又在**原始维度列表**上先行截断。两处叠加产生一个可观测的行为差异：闸在 `collectDims` 内于 **id=0 汇总维度过滤之后**生效，而调用点截断作用于过滤之前的列表——当汇总维度恰在前 128 名内时，会提前丢掉一个真实维度（实测 201 维含首元素汇总时取到 127 而非 128 个）。`用户可见行为变更`：服务端声明的维度数超上限且汇总维度靠前时，`nazhi task` 现按真实维度数取满上界，此前会少取一个维度的数据。正常规模（真实维度数远低于上界）行为不变。

### 架构深化

- **活动字段声明一致性守卫**（`pkg/types/task_activity_fields_guard_test.go`）。24 个活动字段在 3 处结构体声明与 2 份逐字符相同的搬运函数中重复，漏改症状是「`go build` / `go vet` / 全量单测三者全绿，而该字段静默丢失出提交 payload」。**未采用内嵌重构**：Go 的字段提升只对选择器表达式生效，对键名复合字面量（如 `TaskSubmitInput{Name: …}`）永久失效，会使下游 SDK 消费者的代码无法编译，且本仓 CI 无法拦截。改以双断言守卫把「三处字段集一致」与「搬运覆盖全集」变成可执行断言——两条缺一不可，只锁前者会在「三处一致但两份搬运同时漏搬」时恒绿。已做双向变异验证。
- **删除两个零函数空壳文件**（`cmd/nazhi/client_builder.go` / `opt_builder.go`）：各 4 行，仅含包声明与一句「已收敛至 assembly.go」注释，无任何代码、测试或文档引用。注释中「保留以避免 git 历史断裂」不成立，文件内容在 git 历史中完整可查。

### 文档

- **维度截断告警文案改由常量派生**（`pkg/client/task.go`）：此前硬编码「截断到前 128 维」而紧邻参数使用上限常量，调整常量后日志将向运维报错数值。与既往「分页上限硬编码进错误文案」是同一条纪律的复发。
- **订正 `Task` godoc 的时间字段描述**：原文「时间字段为 string」对 `creationTime` / `modifyTime` 两个平台原始 number 数组字段为假，已按 HAR 实证区分「日期展示字段（`*Str` 键）」与「原始时间数组」。
- **兑现一条悬空的变更日志承诺**：`CHANGELOG` 第四轮条目曾声明「维度钳制统一经 `maxDims` 声明（FetchTasks 删除手工截断段）」，但对应提交从未改动该文件。本轮已实际删除。

### 方法论

- **探针输入必须与生产同形，否则「发现」的缺陷是幻觉**：本轮两条被完全证伪的候选都源于探针喂了平台从不产生的输入。动手写探针前必须先回答「这个响应形态服务端真会返回吗，依据在哪」。
- **反方子代理比侦察子代理有效得多，且会推翻主代理的方案方向**：本轮 5 路反方推翻 2 条候选、否决 1 个修复方向、自行发现 1 处守卫盲区。

## 第七轮架构评审（4 路分区侦察 + 逐条行级复核 + 探针实证）

本轮**1 条已复现缺陷已修、3 条候选已落地、4 条候选经深挖后被推翻或降级**。与前六轮不同的是，方法上改为「派子代理做深挖核实，主代理逐条亲读复核」——而本轮最有价值的产出恰恰来自那些**被证伪的候选**：多数子代理指控经复核不成立，而价值最高的缺陷是顺着子代理线索亲读代码挖出的，非子代理总结所得。全部结论与依据见 CLAUDE.md 第 I 节。

### 修复

- **上传附件 ID 的静默错误解码**（`pkg/client/file.go`）。`UploadFile` 解析 `returnData.id` 时手写了 `json.Number → Int64 失败 → Float64 → int64(f)` 回落链，缺整值判定与 int64 范围上界。该分支在 `UseNumber` 下是生产必走路径：`2^63` 被回绕为 `-9223372036854775808`（**正 ID 变负数**）、`3.9` 被静默截断为 `3`，且均不报错；负数与零值亦直接放行。负数附件 ID 会流入 `pictureList` 变成无效载荷，把错误推迟到服务端才暴露，排查方向被误导到远端。现改走 `types.NormalizeInteger` 单点归一，并在调用点补正数性判定（归一模块刻意把缺省形态归零且不承载业务语义）。`用户可见行为变更`：上传响应中 `id` 为非整值、超出 int64 范围、或非正数时，`nazhi file upload` 改为显式报错（此前静默产出错误值）；正常形态（裸整数、整值浮点、数字字符串）行为不变。

### 架构深化

- **写实列表族 flag 声明收为单点**（`cmd/nazhi/circle_list_mode.go`）：`public` / `teacher` / `withdrawn` 的 `init` 各写一份逐字符相同的 flag 注册块，`submitted` 因 `done` 别名需再写一份，合计 5 份。`circleListMode` 早已收口「怎么跑」，「声明有哪些 flag」仍散在四个命令文件——是 `CircleListType` 深化的半程残留。helper 与消费侧 `run` 同处一份知识，净减 15 行，新增列表级 flag 只改一处。先例为 `registerBizFlags`（`assembly.go`）。已用构建产物黑盒验证 `--help` 输出逐字未变。

### 测试

- **哨兵漏斗完整性守卫**（`cmd/nazhi/sentinel_coverage_guard_test.go`）：SDK 新增哨兵而 CLI 退出码漏斗 `mapSentinelToHTTPCode` 未同步映射时，该哨兵静默落 `default 500`。**本缺陷已复发四次**，每次靠人工审查在 1~8 周内补齐（`ErrCookieSyncFailed` 的引入 commit 未动 `output.go`，带着 500/exit2 跑了 7 天）；四次修复都只加了逐哨兵正确性测试，无一次加完整性守卫。500 并不保守：`pkg/envelope` 对 `code>=500` 映射退出码 2 即「可退避重放」，是三档里最主动重试的一档，把永久性错误报此档会让脚本对一个永不成功的请求无限退避重放。守卫两侧数据均从源码 AST 提取，不存在人工登记表——新增哨兵必然改变 `errors.go` 的提取集合，故没有任何「忘了登记」能让守卫失效。已做变异验证：注入第 16 个哨兵后精确点名，还原后复绿。
- **锁定 `DecodeDataList` 的整页全灭语义**（`pkg/types/response_data_list_semantics_test.go`）：该语义此前只散落在 6 处字段旁注释，函数 godoc 零声明，且无任何测试断言过——把 `decodeFieldSlice` 的成功路径改为丢弃结果后全包测试零变红。新增三条测试锁边界，核心断言是「已解出的元素被丢弃」而非仅「返回了错」。同时补全 godoc 并订正归因：整页丢弃是本包的**主动选择**而非 `encoding/json` 的固有性质（原生 Unmarshal 会保留已解出元素）。已做双向变异验证。

### 撤回与降级（经深挖证伪，结论已记入 CLAUDE.md）

- **`DecodeReturnDataSlice` 的 `*[]T` 签名**——证伪。它不是形态不一致，而是**承重设计**：`pkg/client/honor.go` 的 returnData 兜底判据依赖「缺失为 nil」与「空数组为非 nil 空切片」的可区分性，改成 `([]T, error)` 会使兜底失效，且有针对性测试锁定。
- **flexnum 三个零跨包消费符号应下沉**——降级为不做。核实发现被引以为据的「跨包消费才导出」纪律**在全仓无任何表述**，且 `NormalizeInteger` 自身同样零跨包消费却未被列入，指控属选择性取证。改小写经实测零行为影响，但属 breaking change，是否值得由维护者裁决。
- **候选 2 的风险论**——降级。经逐 commit 核查，这 5 份 flag 声明历史上从未漂移过（引入 `key` 时四份同改、Run 守卫前移时四命令同步修改成功），风险属理论推演；但删除测试成立，落地为一致性收口。

### 方法论

- Scout 子代理的指控**默认不可信，须逐条亲读复核**：本轮三组指控（`ParallelDims` 自建错误分类、`writeOpMode` 过浅、`GetSchoolID` 绕过漏斗）全部证伪，且证伪理由同源——把「没复用某个 helper」读成「有行为差异的重复」。
- **探针要挑与生产同形的输入**：本轮实证 `UseNumber` 使 `float64` 分支零可达，只测该分支会得出「生产无影响」的错误结论。

## 第六轮架构评审（全量扫描 + 反方证伪）

本轮改用**反方证伪**推进：6 条候选交给 6 路独立子代理尝试推翻，主代理逐条复核。结果是 **1 条修复、4 条订正、4 条撤回**——被拒的 4 条里，每一条的拒绝理由都指向主代理自己的误读。全部结论与依据见 CLAUDE.md 第 I 节。

### 安全修复

- **诊断摘要的未闭合敏感值不再泄漏**（`pkg/logx/redact.go`）。此前脱敏正则要求值有闭合引号，跨窗或截断在值中间时整体失配，摘要开头即明文。修复分两步：新增 `clipPrefixWindow` 收口粗截切腰的残段；再经代码评审发现同类路径未堵——长度未达粗截窗口时原样返回，而 body 自身停在值内部（服务端或反向代理超时截断 JSON 是常见来源），21 至 85 字节的输入即可让 token / password / captcha 明文进入用户可见的错误摘要。现收口对所有输入无条件执行。`用户可见行为变更`：错误信息中的响应体摘要，对未闭合的敏感键值改为输出 `***`。

### 文档订正（零行为变化）

- 订正四处失实：`pkg/types/honor.go` 悬空引用从未存在的 `FlexString` 类型；`pkg/client/pagination_bounds.go` 声称预算纯函数同文件承载、实际都在 `raw_json.go`；`cmd/nazhi/output.go` 的 `maxCLILimit` 归因错误（`--limit` 走透传路径按字节预算设闸，从不读 `maxSubmittedRecords`；取值本身安全）；`cmd/nazhi/task_payload_json.go` 称历史兼容键「SDK 不消费」而实测消费其中六个。
- **清零全仓审计编号 15 处**（十三/十五/十六域审计、C2/G1/T10/P0-A7/CC1 等）——这些编号全部指向已清空的台账，读代码者无从查证。CLAUDE.md 早已列为待办纪律但从未执行。
- 订正 CLAUDE.md 中 `verify_gitignore` 的记述：该守卫全文只有一个测试、只断言 `CLAUDE.md` 一个路径，`.e2e_token` 无测试守护。

### 测试

- 跨窗脱敏守卫 7 条：6 个敏感键各自跨窗、大小写不敏感、冒号两侧空白、同行先完整键后跨窗键、`RedactSnippetLen` 变体，以及 body 自身残缺（token/password/captcha + 闭合值对照组）。
- CLI 允许键集防漂移守卫：断言「SDK 有而允许集没有」为空，反方向由显式白名单逐项写明理由。配套两条反向断言——白名单不得养僵尸条目、历史兼容键自检表必须与生产声明双向对齐。
- 变异验证：完全撤销脱敏收口使 9 条测试变红；退回到「只对超窗输入收口」时精确只有新增的 2 条变红；删除允许集中的键、白名单僵尸项、deprecated 键三处变异均被精确捕获，无虚假连带。
- 删除一条恒绿测试（跨窗 URL 查询参数守卫）：其三种夹具形态对「给查询正则加终止符锚定」与「撤销收口」两次变异都不红，注释却宣称锁住了该性质。

### 撤回的四条候选（经反方证伪，结论已记入 CLAUDE.md）

分页四道闸的「装配」知识、HTTP 成功谓词统一、`outputSink` 改注入缝、CLI 允许键集改反射生成——四条均证伪成立，不再重提。

本轮由第四轮架构深化扫描驱动（`improve-codebase-architecture`，全模块全细节）。**16 个候选全部经深度核实后落地**：消除跨文件重复实现、把散落知识收为单点、语义化接口，并统一两条路径的行为。**用户可见行为变更**：`ActivateSessionJSON` 空数据从返回 `(nil, nil)` 改为透传 `ErrEmptyUserInfo`（CLI `session activate` 与 `whoami` 的空响应文案统一为 `get_my_info_empty`）；`envelope` 新增 `PartialData`（207）/`Pulse`（429）构造器；`TaskInput` 接口收缩但具体类型保留薄壳 Getter 零破坏。

### 架构深化（第四轮，16 项）

- **限读纪律收为两档 helper**（`pkg/client/request.go`）：新增 `readBodyCapped`（完整档，哨兵由调用点传入）与 `readErrorSnippet`（错误档，64KB + 超限 Close）。此前错误体三行在 httpDo/doBizGet/doGetMenu/上传/下载五处重复，且 64KB 档内部两副面孔（doGetMenu 有 +1 探限+Close、file.go 两处无 Close 也无说明）——后者正是 Login 修复前「drain 无上限续读」的同形态。现错误档成单一形态，完整档四处收口。
- **业务 GET 管线单点化**（`pkg/client/request.go` 的 `doBizGetRaw`）：getMyInfoRaw（自定义 Referer /modify + 双解码器 + postProcessUserInfo 钩子）与两条任务维度管线此前各自手写「请求→解析→业务码」段，现统一经 `doBizGetRaw(ctx, opName, path, headers)`（不预热，锁内安全）；错误包装差异（维度上下文、空归 []）保留在调用点。
- **任务维度 partial 决策单点化**（`partialTasksOutcome` 纯函数）：FetchTasks 与 FetchTasksJSON 各约 40-50 行的 partial 决策表镜像（取消占位/仅取消分支/全失败/部分失败双包装）收为一处定义与测试；维度钳制统一经 `collectDimsOpts.maxDims` 声明（FetchTasks 删除手工截断段）。
- **读命令族收编至 11 命令**（`cmd/nazhi/read_op_runner.go`）：task dimensions/circle-type、honor types/levels/type-options/level-options、self-eval grad-status 共 7 个此前手写七步骨架的命令收编 `runReadOp`，消除各自手抄的 nil 归一兜底（`if x==nil { x=[]T{} }`）。
- **flag 校验派豁免升级为一类命令**（CLAUDE.md D 节）：circle comment/like/delete 与 typical-case delete/delete-batch、honor delete 统一归入 flag 校验派（`--id` 必填→ParseInt→正数→建客户端→调 SDK→envelope），发明 runner 是接口≈实现的新浅层，豁免按类别判定不再枚举命令。
- **TaskInput 接口收缩为 7 方法**（`pkg/types/task.go`）：新增 `ActivityFields` 聚合（24 活动字段，非 wire 类型）与 `GetActivityFields()`，29 个 Getter 回声收敛；具体类型保留 deprecated 薄壳 Getter（外部 `input.GetName()` 仍编译通过，公开 SDK 零破坏）+ 新增 `SetAddressLevel` 供 CLI flag 覆盖。
- **envelope 部分完成语义化**（`pkg/envelope/envelope.go`）：`PartialData`（207）/`Pulse`（429）替代泛型 `Partial`（保留 deprecated）；circle_list_mode/task_list 的 207 字面量与 session 冷却的 429 不再共用外观相同构造器。
- **空语义分裂消除**（`pkg/client/raw_json.go` + `cmd/nazhi/session.go`）：ActivateSessionJSON 透传 `ErrEmptyUserInfo`（对齐 GetMyInfoJSON），session activate 的不可达 `ErrEmptyUserInfo` 分支与 `len(raw)==0 → get_my_info_nil` 死路径删除，两条 CLI 路径统一 `get_my_info_empty`。
- ~~**recoverx 输出可注入**~~（**第五轮已回退此项，见下节**：核实发现 `SetPanicWriter` 全仓零调用点，且其 nil 复位分支会静默丢弃 panic stack，已整条删除）
- **CLI 与 flexnum 数值差异显式锚定**（`cmd/nazhi/task_payload_json.go`）：big.Rat 判定提为具名 `normalizeTaskInputNumericCode` 并声明「与 flexnum 的合法集合差异是有意设计」，`TestTaskInputNumericCodeDivergenceFromFlexnum` 锁定（CLI 接受 2^63+、flexnum 拒绝）。
- **典型案例 attachmentId 收口 flexnum**（`pkg/types/typical_case.go`）：手写 `strconv.ParseInt` 分支改经 `NormalizeInteger`；键缺失不清零语义保留在调用点。
- **宽松类型族收口归一**（`pkg/types/flexjson.go`）：PlayRoleCode number 分支改经 `NormalizeInteger`、IntList 字符串元素改经 `NormalizeIntegerText`；`flex_fields.go` 三份 hours UnmarshalJSON 骨架收敛为 `normalizeHoursField` 单点。
- **归一补前缀适配器**（`pkg/types/flexnum.go` 的 `NormalizeIntegerField`）：FlexInt/flexStringFromNumber 的「归一 + 补字段前缀」样板收为单点；firstInt64/honorMapInt64 保持薄适配器（一行归 + 一行失败处理，再收是过度抽象）。
- **翻页四道闸集中**（`pkg/client/pagination_bounds.go`）：四个上限常量（页数/字节/条数/维度）+ `maxSubmittedCapacityCeiling` 收为单点；`maxSubmittedRecords` 从 fetchAllCirclePages 内联局部提升为包级具名闸。
- **writeOpMode applyFlags 泛型收敛**（`cmd/nazhi/write_op_runner.go`）：task 四个分支的逐字重复闭包收为 `taskApplyAddressLevelFlags[T]` 一行委托（约束经 `SetAddressLevel` 接口方法）。
- **stdin 读取单点化**（`cmd/nazhi/self_eval_submit.go` 的 `readCommentFromFlagOrStdin`）：self-eval submit 与 grad-submit 的「空或 - → 终端检测 → prompt → 超时读取」块收敛为单点，60 秒超时提为 `stdinTimeoutSec` 常量；判空参数错误留在调用点。

### 测试

- `TestTaskInputNumericCodeDivergenceFromFlexnum`（CLI/flexnum 差异锚定）；`session_nil_guard_test.go` 守卫更新为识别 `err != nil`（新空语义契约）；`cmd/nazhi/session_nil_guard_test.go` 的 AST 扫描 switch 补 `nolint:exhaustive`（只关心 EQL/NEQ 两类守卫操作符）。全部新增/更新测试经变异验证或行为矩阵确认，非恒绿。

### 第四轮之后的复扫（五个候选全部经可运行证据复核后否决）

同属 `improve-codebase-architecture` 全量扫描，但**结论与上一节相反**：五个候选逐一用探针程序与变异测试复核后，**无一达到改行为的门槛**，净落地仅两条契约测试与两处注释订正（commit `597e512`）。逐条记录以免后续轮次重复提出：

- **「写实编辑回填断裂」证伪**（本轮最贵的误判）：`CircleRecord.Level`（`int`）与 `TaskEditInput.Level`（`string`）的类型反转属实，但 `cmd/nazhi/task_payload_json.go` 的 `normalizeTaskInputJSON` 已在 CLI 边界把 `hours`/`level`/`checkResult`/`playRole` 的数字形态归一为字符串，其注释本就写明「兼容前端编辑回填的数字字段」。走真实 CLI 路径复测，`{"level":5,"checkResult":1}` → `Level="5"`、`CheckResult="1"`，解码零错误。**用户侧无断裂**，无需转换器。
- **「`FetchTasksJSON` 字节闸位置与写实路径相反」降级为非缺陷**：预算判在 `assemble()` 内确在所有维度请求发完之后，但维度闸是**前置**的（`collectDims` 在 errgroup 启动前截断维度列表），叠加单维 4MiB 限读，驻留有 512MiB 常数上界，属「有界资源放大」而非无界 OOM。实测同等恶意服务端下写实路径堆峰值 11.1MiB（发 1 次请求）、该路径 776MiB（发 128 次）。
- **`writeOpMode` 泛型化不做**：`applyFlags` 已走泛型带类型约束（编译期有保障），`call` 的断言与 `decode` 相距 3 行；泛型化会迫使 `runWriteOp` 一并泛型化，传播成本高于收益。
- **读命令 runner 覆盖缺口范围远小于初判**：13 个文件不走 `runReadOp`，但其中登录/会话/版本/上传下载等语义上本就该走别的路径；真实不一致仅 `self_eval status` 与 `self_eval grad status` 一对。未新增 AST 守卫——为一个尚未发生的偏离引入新维护面，得不偿失。
- **「4 步激活链测试知识泄漏 20+ 处」为数量级错误**：实测真正重复的只是 4 个语义相近的 warmup helper（根因是内外测试包分裂），「几乎每份都带 schoolId 注释」不成立（同时含 `getMenu` 与 `schoolId` 的测试文件为 0 个）。

### 第五轮架构深化（8 项候选，落地 5 项、订正 3 项、否决 1 项）

同属 `improve-codebase-architecture` 全量扫描。八个候选经八路子代理独立核实后，**主代理逐条亲自复核**（含探针程序、变异测试与真实守卫注入），最终落地五项代码改动、三项注释订正、否决一项。commit `b81957b`。

**用户可见行为变更**：无。全部为内部收敛与订正，现有命令的输出与退出码不变。

- **读命令空语义判据修正**（`cmd/nazhi/read_op_runner.go`）：`normalizeEmptyList` 原按「是不是 nil 切片」反射判空，会把不透传的 `json.RawMessage`（底层同为 `[]byte`）误当记录列表。实测后果：nil RawMessage 被换成非 nil 零长值，既让调用点 `== nil` 判据失效，又非合法 JSON，信封序列化报 `unexpected end of JSON input`，stdout 空白且退出码 1。判据改为「是不是记录列表」（`isRecordList`），RawMessage 原样返回交由各命令 success 闭包按自身载荷语义决定空形态。当前无线上可达路径（两道独立兜底），属静态风险收口。
- **输出通道守卫修漏网 + 补 stdout 对称面**（`cmd/nazhi/stderr_discipline_test.go`）：旧判据有两处叠加缺陷——`os.Stderr.Write` 的接收者是 `SelectorExpr` 而非 `Ident`，被类型断言整类跳过；case 内又重复要求方法名为 `Stderr`，对 `Write` 恒假。实测注入 `os.Stderr.Write`/`WriteString`/`fmt.Fprintf` 三形态确认修复前漏网、修复后全部报出。两侧现共用 `scanChannelWrites`，并补「未扫到文件即失败」的正向断言。**stdout 侧此前完全无守卫**，而「错误一律写 stderr」这条纪律历史上被违反 29 处。
- **删除恒绿失实测试**（`cmd/nazhi/main_test.go`）：`TestMain_NoDoubleErrorOutput` 复制生产那一行 `printEnvelope` 并断言 stdout 含 error，而生产早已改走 `printParamError` → stderr。变异验证：换成真实调用即报「stderr 应为空，实际含 error 信封」。改为直接验证 `printParamError` 本身，不再复制 `main.go` 控制流。
- **会话回退门控清理**（`pkg/client/session.go`）：出口门控两支 `return` 字面相同、CAS 布尔被丢弃，收敛为一次无条件置位（零行为变化）。注释承诺的「仅在缓存实际补全后置位」在代码中不存在，且其描述的跨 token 交错在当前锁结构下**不可达**（`RecordSuccess` 只在持锁路径内被调，回退全程在锁外），属虚构场景，已删除该论证。
- **业务码判定收敛**（`pkg/client/request.go`）：`CheckCode→ErrBusinessRejected` 段在同文件相距约 90 行处逐字符重复，收为 `checkBizCode` 单点。订正 `decodeOrInvalidResponse` godoc（自称「五处」却只列四个名字、实际仅两处调用）与 `doBizGetRaw` godoc 的残句。上传域刻意不合并（`ErrUploadRejected` 语义不同）。
- **三处失实注释订正**：`pkg/types/self_eval.go` 与 `pkg/client/self_eval.go` 互相声明「口径一致」实则不同（id 严格失败 vs 容忍归零、空白串处理有别），但三条容器的实际产出都是「成功但 ID=0」，非用户可见缺陷，仅订正注释；`pkg/client/raw_json.go` 补页号槽位的调用方不变式说明（经核实「无边界防护」反而是本仓主流形态，且服务端只能抬高页号、已被双重钳制，不加静默截断以免把显式契约变成隐式容忍）。
- **删除 `recoverx` 死缝**（`internal/recoverx`）：`SetPanicWriter` 全仓零调用点，其「传 nil 恢复默认」分支存入动态值 nil 的 interface（类型断言成功但接收者为 nil），使此后 panic 摘要与 stack 静默丢失。整条删除并订正包注释「3 条 panic-recover 路径」为实际的 2 条。`SetQuiet` 保留（`pkg/client` 无法感知 CLI flag 是真实跨层依赖）。
- **否决：24 个活动字段映射表化**。核实证明 `buildTaskPayload` 的 Trim 写法不对称零行为差异、字段集各处逐行一致且零漂移；改映射表将丢失编译期字段名检查并付运行代价，收益为负。


### 测试

- 新增 `pkg/client/fetch_tasks_residency_test.go` 两条契约测试，锁住任务维度取数的驻留上界：维度闸必须**前置**（判据是实际发出的请求数而非结果条目数——闸若后置，结果仍是 128 条但服务端已被索取 200 次），以及单维响应体受 4MiB 限读约束。**补上此前的缺口**：单维限读只在上传路径有测试，任务维度路径无人断言，缺它则无法排除「维度数合规但单维体积无界」。两条均经变异验证：注入「闸后置」后报「实际发出 200 次」，放大限读阈值后报 5325891 字节。

### 文档

- 订正两处停留在旧世界的失实注释：`WithSubmittedPageSize` 的 godoc 仍把「乘法回绕致 make 负容量」描述为现行成因（`submitted.go` 早已改为除法比较加饱和兜底，该 Option 的真实价值是防容量上界膨胀）；`FetchTasksJSON` 字节预算注释自称「对齐 `getCirclesJSON` 纪律」，实测风险差 70 倍，「对齐」仅指共用常量与截断形态。

第三轮（历史）：

### 修复

- **空列表归一漏掉具名切片类型**（`cmd/nazhi/read_op_runner.go`）：`normalizeEmptyList` 被写成「这条契约的唯一实现处」，但判据是**枚举已知切片类型**——类型 switch 只列了 `[]map[string]any` 与 `[]any`。本仓读命令的返回值绝大多数是**具名切片类型**（`[]types.HonorSelectOption` / `[]types.Dimension` / `[]types.HonorType`），一个都不命中；而已收编的四个命令恰好返回 `[]map[string]any`，使该守卫对现有使用者退化为恒等函数，真正会 nil 的具名类型命令则全部未被收编、各自手抄了一份归一。**守卫收编了用不上的类型，漏掉了需要它的类型。** 现改用 `reflect` 识别任意 nil 切片并构造同类型空切片，新增返回类型自动受保护；非 nil 切片原样透传。
- **典型案例审核状态缺前置校验**（`pkg/client/typical_case.go`）：四个状态常量此前是**无类型 `int` 常规量，且全仓没有任何调用点**——既没约束任何东西，CLI 的 `--status` 也因此零校验直发服务端。`status` 直接驱动 `getTypicalCase` 的列表过滤，与写实列表类型（`CircleListType`）是同一类风险：传错值不报错，只静默返回一份意料之外的记录集合。现按同一范式收为具名类型 `TypicalCaseStatus` + `Valid()`，在发请求前判定，非法值归 `ErrInvalidPayload`（400 / 退出码 3）且不发出任何业务请求；变参缺省仍取 3（全部），合法值原样透传。
  - CLI 此前刻意不校验，理由写的是「避免破坏可能用 `-1` 表达全部的用户脚本」。**该理由无据**：全仓、文档与前端 `classiccanter.vue` 的下拉均无 `-1` 取值。据此收口并订正了该注释。
  - 顺带把结构化与透传两个入口各自拼装的 `getTypicalCase` 查询串收为 `typicalCaseListPath` 单点。

### 测试

- `TestNormalizeEmptyList_NamedSliceTypes` / `TestNormalizeEmptyList_PreservesNonNilContent`（具名切片的归一与非 nil 内容不被清空）；`TestGetTypicalCaseList_InvalidStatusRejectedBeforeRequest`（穷举越界值并断言**不发出任何业务请求**，非法值以区间穷举而非逐个列举，将来往枚举加值仍能覆盖）/ `TestGetTypicalCaseList_ValidStatusPassedThrough`。
- **本轮两处新测试均经变异验证**，其中一处变异最初存活并促使补强了测试设计：
  - **「只断言非 nil」的守卫会被退化实现骗过**：把 `normalizeEmptyList` 的 `IsNil` 判据删掉（退化成「凡切片即换成空切片」）后，仓库原有三条测试**全部通过**——因为它们只断言结果非 nil，从不检查长度与内容。一份清空所有列表的实现同样能让它们通过，而那比原来的 `data:null` 更危险（命令静默输出空列表且不报错）。补 `TestNormalizeEmptyList_PreservesNonNilContent` 断言长度与元素内容后，该变异转红。**教训：谓词写成类型枚举或只判「非 nil」时，测试必须断言内容而非形态。**
  - 典型案例的校验变异（跳过合法性判定）转红，7 个越界值全部触达服务端被逐条捕获。


### 深度核实（第三轮候选逐条对抗式验证）

七个候选经**可执行实验**（而非源码推理）逐条核实后，**只落地一处**、一处顺带订正、**五处经证据降级或证伪**。证伪结论已记入 CLAUDE.md「已证伪误报」节，复发需新证据。

- **已修：`--quiet` 静默契约复发**（`cmd/nazhi/main.go`）：该承诺此前已修复过一次（`assembly.go` 注释记录了首次收敛 timeout/log-level/log-format 三处直写），随后 `main.go` 中关闭日志文件失败的三处 `fmt.Fprintf(os.Stderr, ...)` 又绕过统一的 `warnToStderr` 出口**复发第二次**。两次根因相同：新增告警时照抄 `fmt.Fprintf` 而未查统一出口。现三处收敛到 `warnToStderr`，`--quiet` 下不再有任何 stderr 泄漏（黑盒验证：`NAZHI_TIMEOUT=-1` 在 quiet 与非 quiet 下行为相反且符合契约）。
- **顺带订正**（`pkg/types/flexnum.go`）：原注释称旧写法 `v != float64(int64(v))` 会因溢出回绕「恰好相等」造成静默错误解码——**探针实测不成立**。Go 中 `int64(2^63)` 回绕为负数，往返比较为 `false`，旧写法**正确拒绝**了 2^63。该写法真实的放行缺口是**负向越界字面量**（如 `-2^63-1` 被 float64 舍入到合法下界），注释已按实测事实改写。代码逻辑未变。
- **证伪/降级五处**（均不构成缺陷，故不改）：平台数值归一两套口径行为**完全一致**（逐值对比 6 组用例）；版本号 grep 耦合的实际后果**仅是两处 echo 可能显示空白**（`VERSION` 不参与产物命名与 tag，CI 侧有 `exit 1` 保护）；`test/e2e` token 缓存**已被 gitignore 覆盖且有常驻门禁**；`.golangci.yml` **并无 cmd/ 豁免**（CLAUDE.md 旧记错误已订正）；文档门禁与 PII 守卫覆盖面问题属低危。