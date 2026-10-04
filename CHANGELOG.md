# CHANGELOG

## [Unreleased]

## [1.10.0] - 2026-10-04

发布链接：[v1.10.0](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.10.0)

### 架构评审与文档治理

- 完成五个架构候选的源码、前端参照源码和现有测试交叉核实。
- 保留已足够深且受契约保护的任务提交、CLI 写操作、结构化/透传双路径与活动类型字段 module，不引入破坏性抽象。
- 将会话学校信息回退的竞态修复列为后续独立工作：必须先具备不可复用缓存代次、取消与异常清理、A→B→A 迟到结果隔离和真实并发测试；未完成前不宣称已修复。
- 更新版本记忆、源码地图、发布门禁与架构执行记录。

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
- **已修：限读纪律两处漏网**（`pkg/client`）：逐处核实七个 `LimitReader` 出口后坐实两点。① **`auth.go` Login validate 是唯一超限后不 Close 的出口**——`httpDo` / `doBizGet` / 上传成功体都直 Close 放弃 keep-alive，唯独它只 return，让 `defer drainAndClose` 的 `io.Copy` **无上限续读**剩余 body。实测无限流服务端下 drain 续读 **264MB 直到服务端 EOF**，正是 `request.go` 注释警告的「恶意无限流下拖到超时才兜底」形态。② **`session.go` 的 `doGetMenu` 曾是完全无上限的 `io.ReadAll`**，为全 SDK 唯一漏网出口，且在每个新会话的激活步骤 2/3 必经（真实调用已实证）。修复后该路径的续读量为 0。
  - 顺带把 `file.go` 两处裸字面量 `64*1024` 提为 `maxErrorBodySize` 单点常量，与 `maxResponseBodySize` 并列。限读纪律现按「**body 用途**」分两档（需完整 body 参与解码 → 4MiB；仅供错误文案摘要 → 64KB），**刻意不抽公共 helper**——统一签名只能是多分支参数形式，比各调用点直写三行更难读，且两档差别是「body 用途」这一调用点知识，不该被参数化。
  - **不改变任何用户可见行为**：超限仍归各自的 `ErrInvalidResponse` / `ErrLoginRejected`，仅内部不再无上限续读。

### 测试

- **`cmd/nazhi/stderr_discipline_test.go`**（新增守卫）：从 **AST 层面**扫描 `cmd/nazhi` 全部非测试文件，禁止任何以 `os.Stderr` 为目标的写操作（`fmt.Fprintf` / `Fprintln` / `os.Stderr.Write` 等），白名单仅 `output_sink.go`——通道实现必须现取全局指针，缓存会使既有猴补 `os.Stdout` 的测试读到空串。**既有 `TestQuiet_SuppressesConfigWarnings` 只验 `warnToStderr` 自身行为，对「有无调用方绕过它」零约束，这正是复发两次仍全绿的根因**；本守卫补上该缺口。
  - **变异验证通过**：注入 `fmt.Fprintf(os.Stderr, "warn: 变异验证探针\n")` 后守卫变红并精确报出 `main.go:134:3`，移除后转绿。非恒绿测试。

- **`pkg/client/auth_login_drain_test.go`**（新增守卫）：`TestLogin_ValidateOversizedBody_ClosesWithoutDraining` 用一个「限读上限 + 持续推送」的服务端，断言 Login 超限后**服务端写入量不超过阈值**。**既有 `TestLogin_ValidateOversizedBody_Rejects` 只断言返回 `ErrLoginRejected`，而无论是否 Close 都得到同一错误——该断言对「是否无上限续读」完全无区分力**，属恒真。
  - **变异验证异常强烈**：移除修复后该测试不是变红，而是**直接 300 秒超时**——客户端无上限 drain、服务端持续写入，两者僵持至超时。这精确复现了实测的 264MB 续读现象。恢复修复后 **0.03 秒通过**。


- **`pkg/client/comment_ref_guard_test.go`**（新增守卫）：扫描仓库根 `pkg` / `cmd` / `internal` 的全部注释行，禁止 `file.go:NNN` 形式的行号引用越过目标文件实际行数（豁免 `internal/version/version.go`——版本演进注释按设计记录历史行号）。背景是本轮审计发现 20 处行号引用中已有 1 处越界、多数漂移；行号随任何编辑漂移，符号名由编译器保证。已修两处实证：`request.go` 引用 `auth.go:353`（该文件仅 313 行），且该注释声称 `doBizGet` 有「三处调用点」而 `grep` 证实只有一处；`cmd/nazhi/output.go` 两处引用 `submitted.go:138` 而真实位置是 153 行。
  - **该守卫首版是恒绿的，靠变异验证才发现**：`filepath.Walk` 以 root 自身的 basename 作为首次回调的 `info.Name()`，传相对路径 `../..` 时该 name 就是 `..`，被「跳过以点开头的隐藏目录」判据 `SkipDir`——整个 walk 尚未展开就被跳过。注入 `auth.go:9999` 到 `cmd/nazhi/output.go` 后测试仍绿，说明扫的是空集。修法是先用 `filepath.Abs` 解析绝对路径再 walk（`mustAbs` 辅助函数），修后注入即变红并精确报出该行。**教训已记入 CLAUDE.md：靠目录遍历的守卫若自身没有正向的「确实扫到了东西」断言，可能整条空转。**

### 门禁

- 修掉一处**预先存在**的 lint 阻塞（`pkg/client/typical_case_status_test.go:39` 注释缺 `//` 后空格，gocritic `commentFormatting`）。经 `git stash` 回到基线复跑确认该问题**早于本轮存在**，非本轮引入。
- 全量门禁通过：`gofmt`、`go vet`、`golangci-lint`（退出码 0）、`go test -count=1 ./...`（全包绿）、性能门禁、`docrules`、`verify_gitignore`。

### 已否决（经证据核实，复发需新证据）

- **容器回退链顺序矛盾**：结构化自评走 `returnData → dataMap → dataList`，raw 透传走 `returnData → dataList[0] → dataMap`。**不可观测**——HAR 夹具实测两个自评端点均只有 `dataMap` 有内容，矛盾仅在「双容器并存且内容不同」时显现，该形态无任何证据存在。代码与 `self_eval status --help` 均已披露该顺序与对账口径，是有意差异。
- **页维度并发扇出两份**：提取属 YAGNI。`collectDims` 值得提取是因为它承载**会漂移的政策**（id==0 跳过、取消传播语义、维度闸、错误分类）；页级扇出两处除 fetch 闭包与槽位类型外逐行相同，且 Go 1.22+ 逐迭代循环变量使 `pn := pageNo` 亦为冗余捕获，是机械代码而非语义。
- **输出通道应改造成注入缝**：`outputSink` 解决的是「写哪条通道」而非注入；`processOutputSink()` 每次现取全局是**承重设计**（`version_test.go` 直接猴补 `os.Stdout` 后经 `printEnvelope` 观察输出，缓存指针会使断言读到空串；`output_test.go` 更用 EPIPE 制造写入失败，纯 buffer 注入会丢失该语义）。改造需动 41+ 个测试文件且削弱失败路径覆盖。
- **活动类型注册表该建模进 SDK**：与 `TaskSubmitInput` godoc 及 v1.4.0 的明确决策冲突（必填规则由调用方按活动类型自行填写）；且前端**表单显示 ≠ 表单必填**，照搬 `v-if` 分支会把规则建错。

### 变更（第二轮架构深化）

第二轮架构深化扫描把五处「同一份知识散落多处、靠注释维持一致」的区域改为单一模块承担。**不改变任何用户可见行为**（唯一例外见该轮「修复」第一条的错误文案来源），旧接口全部保留，无 BREAKING。


- **平台数值归一收为单一模块**（`pkg/types/flexnum.go`）：「平台可能把整数返回成数字 / 数字字符串 / 整值浮点 / 空串 / null，怎样才算合法整数」此前散落七处实现，防护程度各不相同——`FlexInt` 与 `PayloadPositiveIDValid` 有 2^63 上界检查，`parseFlexInt` 族与 `typicalCaseCodeString` 仍是会静默回绕的 `float64(int64(f))` 旧写法，`honorMapInt64` 与 `firstInt64` 用了 `math.Trunc` 但**缺上界**（大整数经 `int64(n)` 会溢出成负数）。四处注释互指「同口径」而代码各异。现由 `NormalizeInteger`（原始 JSON 字节）与 `NormalizeIntegerValue`（map 路径的 any 值）承载唯一实现，各调用点只保留自己的业务语义。**行为收紧两处**：`honorMapInt64` 与 `firstInt64` 顺带获得原先缺失的 2^63 越界拒绝；典型案例的字符串浮点（`"1.0"`）由报错改为识别，与平台返回裸浮点时的行为对齐。
- **输出通道收为可注入值类型**（`cmd/nazhi/output_sink.go`）：全 CLI 六处直写 `os.Stdout` / `os.Stderr` 的出口改经 `outputSink`，包级函数签名与 160 处调用点零改动。此前测试要观察输出必须猴补进程全局变量（`captureStdio` 被 41 个文件、112 处复用），最该被测的「stdout 只承载成功数据」契约被放到测试面之外，且猴补全局本身是进程级竞争类别。
- **读命令族共享 runner**（`cmd/nazhi/read_op_runner.go`）：写操作有 `runWriteOp`、列表有 `circleListMode`，最常见的「取一个对象」此前十七处 Run 各自内联同一套七步骨架。现由 `runReadOp` 承载，**「空即空数组」由构造保证**——nil 切片直塞成功信封会输出 `"data":null`，下游 `jq '.data[]'` 对 null 报错退出。
- **并发维度收集收为单一内核**（`pkg/client` 的 `collectDims`）：结构化路径与透传路径此前各写一套 fan-out 与错误聚合，靠注释约定「改动一条必须同步核对另一条」维持一致，而该约定已在事实上破裂。现两条路径共用同一内核，错误分类经 `classifyDimErrors` 收口。
- **诊断摘要收为单一入口**（`pkg/logx.RedactSnippet`）：摘要长度此前是各调用点传入的字面量 `100`，散落七处且接口上不可见；`session.go` 另有一种裸 `LimitReader` 写法，安全性仅靠「两处恰好都是 100」成立。现长度与「先脱敏后截断」的次序由模块单点持有，SDK 内私有实现已删除。

### 修复（第二轮架构深化）

- **分页上界错误文案与常量脱钩**：`honor list` 与 `typical-case list` 把 `500` 硬编码进「不能超过 500」的错误文案，而 `circle images` 用常量格式化——改 `maxPageSize` 时前两者会对用户谎报上限，而全仓无一条测试断言该文案。现三处统一走 `validatePaginationFlags`，文案由常量派生。

### 测试（第二轮架构深化）

- 新增五组守卫测试：数值归一的接受/拒绝形态穷举与越界不回绕、输出通道的跨出口归属对照、读命令的列表归一与分页边界、并发收集内核的保序与取消传播开关、脱敏摘要的跨截断边界不泄漏。
- **本轮全部新增测试均经变异验证**（注入生产缺陷确认测试变红），其中四处变异最初存活并促使补强了测试设计，记录如下以备复发参考：
  - **等价性测试的天然盲区**：`collectDims` 的保序用例最初无法证伪「按完成序落槽」——补了完成顺序夹具自检，确保完成顺序确实与声明顺序不同。
  - **夹具长度决定测试是否恒绿**：脱敏的「跨截断边界」用例连续三轮变异不红。根因是输入总长未超过摘要上限，截断根本没触发；修正后仍不红，因为敏感键名本身也跨界、两种顺序输出完全相同。最终夹具需同时满足三个条件才有判别力。
  - **变异不红 ≠ 测试无用**：`NormalizeIntegerFloat` 的 `math.Trunc` 判定退回旧写法后测试仍绿，经探针验证这是**正确结果**——两种写法在所有可达输入上等价，差异仅存在于上界判定已拦下的区间，真正的防线是上界检查（有独立守卫）。
  - **既有测试的存废需自行核实**：扫描报告建议删除的两条测试（`TestUpdateHonor_NonIntegralTypeIDDoesNotTriggerLookup` / `TestFirstInt64_RejectsFractionalFloat`）经核实均为有效测试——前者是端到端业务行为，后者覆盖归一模块之外的调用点特有语义（多 key 短路、`float32` 支持），**本轮未删除任何既有测试**。


## [1.8.0] - 2026-09-26

发布链接：[v1.8.0](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.8.0)

本轮由架构深化扫描驱动（`improve-codebase-architecture`），把三个反复出错的区域从「知识靠注释和文档维持」改为「知识由代码承担」。全部改动不改变任何已抓包验证的线协议行为，旧 SDK 接口全部保留为薄壳转发。

> **升级提示**：本版含一处用户可见行为变更——**参数错误从 stdout 改为写 stderr**（详见下方「变更」第二条）。依赖旧通道的脚本需同步调整；退出码语义不变。

### 变更

- **写实列表类型成为具名类型**（SDK 接口重构）：新增 `client.CircleListType` 与 `CircleListPublic` / `CircleListTeacher` / `CircleListSubmitted` / `CircleListWithdrawn` 四个常量。此前该概念在代码里只是一串裸 `int`，名字只活在注释块中——而这个数字直接驱动服务端的列表过滤，**传错不报错、只会静默返回另一个列表的数据**。新增统一入口 `ListCirclesJSON` / `ListCirclesLimitJSON` / `PeekCircleTotal` / `ListCircleRecords`；原 12 个按类型各设一份的入口（`GetPublicCirclesJSON` 等）保留为薄壳转发，行为逐字不变。非法类型在发请求前即被拒绝（400 / 退出码 3）。CLI 侧四个写实列表命令的模式配置从 52 行转发闭包塌缩为 4 行声明。
- **参数错误统一写 stderr**：此前参数错误被劈成两个输出通道（`printParamError` 走 stderr 47 处 vs `printEnvelope(envelope.Error(400))` 走 stdout 29 处），规则不存在于任何可学的地方——同一个命令的 6 步控制流里第 1 步与第 6 步走 stdout、中间 4 步走 stderr。现统一为 **stdout 只承载成功数据，错误一律写 stderr**；login 的 401/429/502 保留各自错误码但同样改走 stderr。**这是一处用户可见的行为变更**：过去从 stdout 读参数错误的脚本需改从 stderr 读。
- **写操作命令的 `--help` 列出 payload 允许键**：未知键拒绝的文案一直承诺「允许键见 nazhi <cmd> --help」，而帮助文本里从未有过键名清单——用户被指向一条死路。现在 9 个写操作命令的 `--help` 均按用户书写形态（驼峰原样）列出全部允许键并稳定排序。

### 修复

- **limit 裁剪扫描器补齐分支级回归防护**：`--offset/--limit` 的客户端裁剪由一套手写 JSON 词法扫描器（深度计数 + 字符串边界感知 + 转义跳过）实现，此前全仓对该函数的引用只有生产调用与 benchmark 两处，零个测试断言其行为；现有端到端测试的 mock 数据无法触发字符串与转义分支。已补齐六类现实输入的分支级测试（学生正文由自由填写，含花括号、引号、反斜杠是现实输入）。扫描器本身经变异验证确认当前行为正确，本次不改变其实现。
- **四处限读注释与常量漂移**：`request.go` / `auth.go` / `file.go` 的注释仍写「封顶 1MB」，而唯一真相源常量已是 4MiB；`request.go` 内部尤其自相矛盾——同一文件里既有「1MB」的过期注释，也记录着「1MB 上调到 4MiB」的事故原因。四处一律改为引用常量名而非写死数值。
- **清理悬空引用**：4 处以 commit hash 与审计编号作为代码交叉引用（指向已清空的审计台账，读代码者无从查证），3 处「锁死 ：」是删编号留下的空标签残骸。

### 测试

- 新增扫描器分支级回归测试、写实列表类型等价性与绑定测试、参数错误通道守卫（出口本身 + 逐命令）、`--help` 允许键清单一致性测试。
- 同步更新 19 个测试文件的输出通道断言——它们此前钉死的正是 stdout/stderr 不一致这一缺陷。
- 本轮全部新增与修改测试均经**变异验证**（注入生产缺陷确认测试变红）。其中 4 处最初存活的变异促使补强了测试设计：等价性测试两边同走薄壳链比不出差异、mock 服务端对所有 type 返回相同数据、`Valid()` 的非法值逐个列举会漏掉刚被加进 `switch` 的那个（改为遍历 -128..127 穷举）。

## [1.7.2] - 2026-09-26

发布链接：[v1.7.2](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.7.2)

本轮由八路并行深度审计（对照前端源码与真实抓包）驱动，修复 1 个 P0 与 3 个 P1 缺陷——它们共同的特点是**错误静默或误分类**，在脚本化调用中比崩溃更难察觉。

### 修复

- **panic 后进程以 exit 0 退出**：顶层 panic 被 recover 后，函数尾部的退出码判断与 `os.Exit` 因 defer 链中断而不可达，进程以成功状态结束——自动化脚本会把崩溃当成功而继续执行。既有守卫是 AST 静态扫描（只校验 `recover`/`printError` 字样存在），查不出这类控制流不可达。修复后 panic 一律以退出码 2 结束，并新增**真实二进制子进程**黑盒测试锁定（commit `45015a1`）。
- **写实容量钳制整数回绕致崩溃**：`maxTotalPage * pageSize` 在页长极大时回绕为负，使容量钳制与条数上界双双失效，`make` 拿到负容量直接 panic。`WithSubmittedPageSize` 是公开选项且此前无上界。修复为选项层加上界 + 除法比较 + 饱和兜底（commit `e3ccbd0`）。
- **写实「图片超限」被网络错误掩盖**：图片数量预检是纯本地判定，却排在任务元数据请求之后；元数据接口失败时用户看到的是「获取任务元数据失败」（服务端错误，退出码 2）而非「图片最多 2 张」（参数错误，退出码 3），脚本的重试决策因此被误导。预检已前移到任何网络请求之前（commit `68ab579`）。
- **无协议头的 baseURL 下 cookie 静默未写入**：`cookiejar` 对非 http/https 的协议头直接跳过写入且不报错，同步却返回成功——后续所有业务接口返回空列表，排查方向被误导到服务端。`--base-url example.com`（漏写 `http://`）即可触发。现显式校验并报出明确错误（commit `b7dd0fe`）。
- **学校 ID 七位以上被误判为非法**：服务端返回的数字经 `float64` 解码后用 `%v` 格式化会变成科学计数法（如 `1234567` → `1.234567e+06`），随后的整数解析必然失败。修复后七位以上学校 ID 可正常登录；超 2^53 的标识符也不再丢精度（commit `0305b0f`）。

### 测试

- 修复三处名实不符或恒绿的测试：上传重定向守卫的攻击者探针原指向不可解析的保留域名（断言恒真）、零断言的遗留用例、与实现相反的注释（commit `2b8f613`）。
- 新增回归测试覆盖：panic 退出码（真实二进制子进程）、图片预检顺序、容量钳制溢出、cookie 协议头校验、学校 ID 大整数，每项均经变异验证确认能捕获对应缺陷。

### 文档

- 更新核心记忆库：前端镜像关系、组件壳与业务组件二分、接口覆盖度对账结论、注释与实现的失实处（commit `1085489`、`f4795f8`）。

## [1.7.1] - 2026-09-25

发布链接：[v1.7.1](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.7.1)

### 特性

- **性能测量基础设施**：白盒基准测试套件（`bench_fixture_test.go` / `bench_hot_path_test.go`）覆盖 HTTP 响应处理、大响应体翻页合并、结构化解码、session 缓存命中、日志脱敏等 9 条热点路径；新增 `make bench` / `make bench-baseline` / `make test-perf` target，`allocs/op` 硬门禁接入 `ci-local` 与 CI check job（门禁只卡分配次数——同 Go 版本下完全确定，不受 CI 慢 runner 时间波动影响；`ns/op` 与 `B/op` 仅软记录）。

### 性能

- **日志参数提前求值**：`httpDo`/`doBizGet` 处理大响应体时，`logx.RedactBodyThenTruncate` 作为函数实参在日志级别检查前被求值，内部 `string(body)` 对 4MB 响应体分配等大字符串并跑两遍全量正则，而默认 LevelWarn 下 Info 日志永不输出。新增未导出 `logEnabled` helper 在调用点先判级别再拼参，错误路径摘要改用 `redactSnippet`（先按 4096 字节粗截再脱敏，不破坏 HTTP-1 先脱敏后截断契约）——`HTTPDo_LargeBody` B/op 15.4MB→4.7MB（-69%），allocs/op 205→140（commit `29612b0`）。
- **dataList 校验轻量化**：`fetchCirclePageJSON` 二次校验从全量 `json.Unmarshal` 改为首字符判定（`json.Valid` 已保证整体合法性）——allocs/op 675→158（-76%），B/op 2.2MB→1.6MB（commit `2c58a3a`）。
- **合并路径精确预分配**：`assembleCirclesJSON` 按各页实际长度求和精确预分配，减少 bytes.Buffer 倍增扩容的多次整块拷贝（commit `5ff9ca1`）。

### 修复

- **门禁 `-race` 假阳性**：CI check 的 `go test -race ./pkg/...` 步骤下四个 `TestPerfBudget_*` 门禁 FAIL——race 检测桩本身显著增加每次操作的堆分配，预算（以非 race 实测为准）在 race 下失去意义。新增 `//go:build race` / `//go:build !race` 约束文件注入 `raceEnabled`，门禁在 race 下优雅跳过；非 race 的独立性能步骤（`make test-perf` / CI 性能门禁步骤）照常严格断言（commit `4482392`）。

## [1.7.0] - 2026-09-23

发布链接：[v1.7.0](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.7.0)

### 特性

- **免验证码登录**：`Login()` 改走五育活动端 `POST /uiActivityLogin/studentLogin`（密码本地 MD5 计算，与官方 APK `hex_md5` 一致）；schoolId 留空时自动通过匿名接口推断；签发 token 与主站 `X-Auth-Token` 同认证体系，可直接访问业务接口（commit `2b47968`）。
- **移除验证码识别**：删除 `nazhi-captcha-sdk` 依赖、`CaptchaRecognizer` 接口、`WithCustomOCR`、`c.ocr` 字段、内置识别器及 `ErrOCRNotConfigured`/`ErrOCRPanic` 哨兵（commit `a97d07d`、`d07ea18`）；`nazhi login` 不再需要任何验证码识别配置。

### 修复

- 写实翻页容量按条数上界钳制防 OOM；`assembleCirclesJSON` 预分配钳制防攻陷服务端超大页数（commit `b1b0cbb`、`7df116c`、`7259fe5`）。
- 原始 JSON 写实列表 `dataList` null 形态归一空列表防 jq 破坏（commit `840107e`、`0bd551e`）。
- 任务拉取全失败按哨兵优先级映射 502/503/429，服务端宕机不再误报业务拒绝（commit `4cae3f7`）。
- `Login` cookie 同步失败升级为返回错误防静默空数据（commit `6bc8c90`）。
- `newCleanClient` 回退分支补 `Proxy:nil` 防环境代理劫持；上传图片路径去重防重复上传孤儿附件（commit `30c2d3f`、`ab47cae`）。
- FlexInt 大整数字面量拒绝防 int64 回绕静默误解码（commit `fff2c80`）。
- 写实/荣誉/自评输入 rune 上限显式拒绝：典型案例 198/1500、自我评价 700、honor typeId 非整值反查（commit `8b0daa4`、`26a3b9c`、`c97bf3b`）。
- `DownloadFile` 流式写补 50MB 字节上限防受信子域无限流写满磁盘（commit `c54b686`）。
- `FetchTasksJSON` 补维度数与累积字节双钳制防单请求内存放大（commit `5df15d3`）。
- CLI 参数与载荷校验收敛到严格拒绝语义；`--quiet` 顶层 panic 不再写 stderr stack（commit `599afca`、`defe12c`）。
- `printEnvelope` 消息统一过 `RedactBody` 消除 stdout 通道脱敏缺口（commit `b4c5d7d`）。

### 工程

- 测试环境变量清理统一 `t.Setenv` 自动恢复；raw_json 取消测试窗口放宽防 CI flaky（commit `68a8ed4`、`b021a01`）。
- 清理三处免验证码迁移后的陈旧 OCR/验证码文案（commit `a1d62eb`）。

## [1.6.5] - 2026-09-08

发布链接：[v1.6.5](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.5)

### 文档与工程

- 清理全模块深度审查报告临时文件，收敛仓库交付物。
- 完善发布与文档治理规范，更新核心记忆库与版本索引。

## [1.6.4] - 2026-09-05

发布链接：[v1.6.4](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.4)

### 修复

- 修复结构化和原始 JSON 写实列表在 `totalPage` 虚低或为 0 时的漏页与索引越界，限制分页同步按 `totalNum` 推导页数。
- 原始 JSON 写实列表拒绝非数组 `dataList`，统一返回 `ErrInvalidResponse`。
- 文件上传遇到无法安全 Clone 的自定义 `RoundTripper` 时回退到无状态默认传输器，避免认证拦截器状态进入上传通道。
- stdin 读取在取消或超时后关闭实际句柄，唤醒后台阻塞读取。

## [1.6.3] - 2026-09-05

发布链接：[v1.6.3](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.3)

### 修复

- 修复文档规则仍读取已删除的 `docs/cli/README.md` 与 `docs/sdk/README.md`，改为校验当前唯一的 `docs/README.md` 源码地图。
- 将文档规则和仓库元数据检查接入 Makefile 与 CI，避免治理测试脱离发布门禁。
- 修复 stderr 写入失败时 `printError` 未设置最终退出码的问题，确保异常输出路径仍返回非零退出状态。

## [1.6.2] - 2026-08-28

发布链接：[v1.6.2](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.2)

### 修复

- 三个时间敏感测试适配 CI 慢 runner，修复 v1.6.1 起 CI 红灯导致 Release 链断裂（commit `a7b27fe`）：TestFetchTasks_Parallel 的 1200ms 绝对耗时断言降级为观察日志（判别力由 ConcurrentLimitBounded 的 in-flight 峰值计数承担，CI 实测 5.11s vs 本机 1.2s）；TestFetchTasks_MixedBizAndCancel 的 ctx 1.5s→4s、handler 睡眠 2s→6s 保持 ctx 先超时语义；TestGetSubmittedCircles_CancelDuringPaging 两处 5s 保护超时放宽至 15s。

## [1.6.1] - 2026-08-28

发布链接：[v1.6.1](https://github.com/Wenaixi/nazhi-cli/releases/tag/v1.6.1)

### 修复

- 移除 go.mod 本地 replace（指向本机绝对路径导致异机/CI 构建失败），改为 go.mod 声明远程版本 + 本地开发用不入库 go.work 覆盖（本地优先/远程兜底）（commit `de969b5`）。
- captcha-sdk 依赖升级 v0.2.1（含 README import 路径修复与 CI 断言收紧）。

## [1.6.0] - 2026-08-27

### 修复

- ActivateSessionJSON 改调 GetMyInfo 使学校信息 SSO 降级补全真实生效（原实现直通 sm.Activate 后 Marshal，godoc 承诺的补全从未执行）；空数据仍返回 (nil,nil) 保持原契约（commit `a621901`）。
- UpdateCachedUserInfo 显式比对 token：签名加 forToken 参数，跨 token 的迟到写入（多 goroutine 场景）不再污染新 token 的缓存（commit `9f36dea`）。
- 日志脱敏先于截断：新增 logx.RedactBodyThenTruncate，修复敏感值跨 100 字节截断边界时正则失配泄漏前缀；request.go/file.go/auth.go 七处消费点统一改调，auth.go 四处 debug bodySnippet 同步改为脱敏版（commit `cdec8b9`）。
- httpDo 响应体读取封顶 1MB：异常/被劫持服务端塞超大 body 不再整体入内存，超限归 ErrInvalidResponse（commit `07ad8a5`）。
- UpdateMyInfoStructured 全零输入视为 no-op：不再发出仅含 studentUuid 空串的空 POST 并失效本地缓存（CLI --payload '{}' 可达）（commit `85ed8f9`）。
- 本地 IO 错误归参数档：上传附件不存在 / 图片解码打开失败 / 下载目标路径不可写由 SDK 包 ErrInvalidPayload 哨兵，CLI 退出码从 500/exit2 纠正为 400/exit3，脚本不再对永久性本地输入错误无限重试（commit `f42df62`）。

### 特性

- CLI 与 SDK 内置 nazhi-captcha-sdk 本地验证码识别器，Login 零配置可用，移除外部视觉模型 OCR 依赖（commit `25e02bb` 起；本提交同步清理集成测试与文档）。

### 文档

- GetDate wire 形态披露：前端 el-date-picker 无 value-format 实际提交 ISO 8601 时间戳，纯日期是否被服务端接受以平台裁决为准（types/honor.go + honor 命令 Long）。
- 典型案例 status 合法集合注释修正为 0/1/2/3（原漏 0=未审核）。
- GetSubmittedCirclesJSON godoc 修正为「恒为合法 JSON 数组」（原「可能为 null」失实）。

### 工程

- user update 测试 helper 单次 Body.Read 改 io.ReadAll 消除理论欠读（commit `b229b7d`）。
- gofmt 对齐 honor Long 与 client.go 空行（commit `b520bf2`）。

## [1.5.3] - 2026-08-26

### 修复

- 写实 remark 关键词强制传图分支无回归测试锁定：task.go:320-325（SDK 单方面发明的校验——前端 remark 仅作展示无此校验）的「备注含照片/图片/pdf + pictureList 为空 → ErrInvalidPayload」逻辑原本仅靠实现存在，重构误删不会有任何失败信号。新增 task_remark_image_required_test.go 表驱动测试 10 例覆盖（commit `897f9ac`）。
- 典型案例批删空切片守卫缺 pkg/client 层回归：typical_case.go:213-215 的 `len(ids)==0 → ErrInvalidPayload` 实现正确（commit 1522446 修复本体），但客户端层无任何测试断言该守卫。test/e2e:109 与 cmd/nazhi/missing_cli_capabilities_test.go:51-62 两处引用均只覆盖相邻路径。新增 typical_case_batch_empty_test.go 客户端层回归 2 例（nil + 空切片双态，httptest server 零业务请求计数）（commit `07fb3da`）。
- DownloadFile 中途传输失败缺 ErrNetwork 哨兵：file.go:435-438 copyErr 路径裸 `fmt.Errorf("写入文件失败: %w", copyErr)` 让 SDK 调用方按 `errors.Is(err, ErrInvalidResponse)` / `ErrNetwork` 判重试时不可识别（服务端 200+HTML 已被主管线拦截，但 mid-stream EOF/连接重置场景下 do() 拿不到响应头仅拿到 copyErr）；同函数 `:441` closeErr 路径同样裸包装。修复：copyErr 非 ctx 取消时包装 ErrNetwork 哨兵（用户主动 ctx 取消不归类为网络故障，避免自动重试误触发），closeErr 包 ErrNetwork。新增 file_download_midstream_test.go 回归 2 例（commit `83d1f41`）。
- 业务层四处 DecodeResponse 裸包装与主管线分叉：auth.go:49 GetSchoolID + auth.go:274 验证码预校验 + user.go:69 GetMyInfo + raw_json.go:591 fetchTasksDimensionJSON 自行调 `types.DecodeResponse` 后裸 `fmt.Errorf`，让 `errors.Is(err, ErrInvalidResponse)` 在服务端 200+HTML（WAF/维护页）场景下落空，CLI 漏斗走 default 500/exit2，与主管线 `doBizAndDecode` (request.go:234) 双 %w 哨兵口径分叉。修复：抽 `decodeOrInvalidResponse(opName, bodyBytes)` helper 接管 DecodeResponse + ErrInvalidResponse 包装；四处调用方各改一行（commit `b6b64b4`）。
- honor update 缺 payload 正数 id 校验：cmd/nazhi/honor.go:170-199 update 命令对 `payload["id"]` 零校验，会发出无 id 的业务请求，与 cmd/nazhi/typical_case.go:225-228 + 同文件 delete/levels 双重分叉。前端 performanceM.vue:489 编辑提交必然注入记录 id，此处对齐该契约。修复：平移 `typicalCasePayloadIDValid` 为共享 helper `PayloadPositiveIDValid` 到 cmd/nazhi/payload.go；honor update Run 在 json.Unmarshal 后调用该 helper，缺 id 或非正数 → envelope.Error(400) + exit 3 不发业务请求；typical-case update 改为调用共享 helper。新增 honor_update_id_test.go RED→GREEN 测试 1 例（commit `4f69402`）。
- postProcessSchoolFallback 锁外原地突变数据竞争窗口（防御纵深上沿）：ActivateSession 出口在 sm.mu 锁外对共享缓存指针（RecordSuccess 原指针入缓存）原地写 SchoolID/SchoolName，与 fast path 并发读取方形成真实数据竞争（Go 内存模型下 string 头撕裂风险）。本 API 的 godoc 明确承诺并发安全，但 `-race` detector 在 100 goroutine 并发激活测试中可复现。修复：新增 sm.fallbackDone atomic.Bool 标志区分首次激活与重入；首次激活走 `infoCopy := *info` 浅拷贝 + fallback 改副本 + `UpdateCachedUserInfo(&infoCopy)` 替换缓存指针 + fallbackDone 设 true；重入/fast path 命中直接返回缓存指针保持 DCL 同一缓存指针契约；RecordFailure 清 fallbackDone。pkg/client 全域 29 秒 -race 全绿（commit `b480538`）。
- honor list / typical-case list 分页参数缺非负校验：cmd/nazhi/honor.go:71-72 与 cmd/nazhi/typical_case.go:90-92 对 `--page`/`--page-size` 负值原样透传 SDK 查询串（`raw_json.go:749` 直拼 `strconv.Itoa(pageNo)`），对照组 cmd/nazhi/circle_metadata.go:83-90 对同形状参数有完整校验。修复：两处各加 4 行非负守卫 + envelope.Error(400) + exit 3（commit `926c897`）。
- WithHTTPClient 超时继承语义无专项回归测试：pkg/client/client.go:219-228 prevTimeout 继承是 #22 证伪后的行为加固产物，但仅由 godoc 承载；重构误删该逻辑 CI 全绿静默回归，重新引入 Option 声明顺序敏感性。新增 option_inherit_timeout_test.go 表驱动测试 2 例（双序）锁定（commit `926c897`）。

### 文档

- self_eval_submit_test.go:307 块注释「业务错误应触发 pendingExitCode=2（envelope.Error 5xx → exit code 2）」与紧随断言（:308）与 t.Errorf 文案（:309）均锁定 pendingExitCode=1（ErrBusinessRejected → 422 → exit 1）矛盾。CLAUDE.md #29 记载「同步更新 task_submit_test 与 self_eval_submit_test 两处锁定断言为 exit 1」时改了断言与 t.Errorf，漏改上一行的块注释。修正：块注释 2 改 1（commit `013a311`）。
- file.go:25 + :71 注释「前端限制 10MB」与 reference/nazhi 经典案例镜像实际提示文案「20MB」矛盾（commits ac2986a/1e34350 已同步镜像）。修正：两处注释「前端限制 10MB」改「前端镜像文案 20MB」（commit `013a311`）。
- task.go:407 EditCircle godoc 披露范围不足：原 godoc 点名回填例外仅 hours 与图片两项，首句「用户字段空串原样发送」在编辑语境下构成误导。前端 openEdit→getCircleTypeByTaskId 把列表记录 26 个活动字段（name/hostName/circleDate/rank/level/circleBeginDate/circleEndDate/checkResult/patentType/patentNum/address/termName/各类型专属字段/playRole/likeSpecialty1-3 等）整体回填 JSON.stringify 后整包提交；SDK 编辑路径若只填 `{id,taskId,content}`（CLI 官方示例正是如此引导），上述字段全部以空串上线。修正：godoc 扩写披露「前端编辑是 26 字段全量回填模式——任何留空的专属字段 SDK 均发空串，要保留原值请从 CircleRecord 对应字段回填」（commit `013a311`）。
- user.go:30 GetMyInfo godoc fast path 描述与实现相反：原注释「session 已激活（fast path）时返回 nil,nil」与实际不符——sm.Activate 持锁 fast path 命中返回 `(sm.cachedUserInfo, nil)` 非 nil，全链路不存在 (nil,nil) 返回。复用机制正因 fast path 返回非 nil info 被 :35-38 直接采纳。修正：注释「返回 nil,nil」改「返回缓存指针（非 nil）」（commit `013a311`）。
- cmd/nazhi/output.go:101 rejectLoneOffset godoc 缺调用次序披露：四写实列表命令允许在 buildBizClient 之后调用（task_teacher/public/submitted/withdrawn），与 honor delete / typical-case delete 等先校后建派两派并存。修正：godoc 加披露段说明「重构如欲收敛到先校后建，需同步四调用点的位置；当前两派并存是历史累积的有意保留」（commit `013a311`）。

### 工程

- gofmt 对齐 pkg/client/request.go + cmd/nazhi/payload.go 两个主树文件（commit `8b9620c`）。

## [1.5.2] - 2026-08-26

### 修复

- 文件下载 404/403 被误判为网络故障：DownloadFile 非 2xx 分支的 default 哨兵此前挂 ErrNetwork，确定性失败（附件已删/风控拦截）映射 502/exit2，脚本按「可重试」对永久失败无限重试；改归 ErrInvalidResponse（429→限流、5xx→服务端不可用分支不变），与 httpDo/doBizGet/doGetMenu/UploadFile 四个兄弟分支同口径，退出码纠正为 422/exit1（commit `563d1d1`）。
- 会话激活锁窗口可被放大到秒级：学校信息 SSO 回退补全（getMyInfo 缺 schoolId/schoolName 时）曾在 sm.mu 持锁路径内同步发起真实 SSO 域 POST，最坏把多 goroutine 并发激活的阻塞时间从数百毫秒放大到 HTTP 超时秒级，违背 ActivateSession 并发契约；回退已移至解锁后执行（幂等，字段已齐零开销），className 清理留锁内（commit `7a8d014`）。附带治理：两个单测夹具此前每次运行都向生产 SSO 域发真实请求，现已注入本地测试服务器。
- 荣誉 typeName 自动反查在大 id 下失效：反查比较用平台相关 int 宽度，typeId 超 2^31 时 32 位编译目标静默截断导致必不命中、退化为空 typeName 提交；统一 int64 比较（commit `730b3df`）。
- honor add 缺 --payload 的报错顺序与同族命令分叉：先建客户端后校验 payload，双参数缺失时报 token 配置错误而非参数错误；已收敛为先校验后建客户端的统一规范（commit `afc9806`）。
- task preview 同款校验顺序分叉：「先校验 payload 后建客户端」不变式的漏网第三处（submit/edit 已于上轮修复），位置对齐并补回归（commit `5467ce3`）。
- session --help 出现两行相同的 activate 条目：activate 子命令被注册两次而 cobra 不去重，删除重复注册点（commit `22b4093`）。

### 文档

- .env.example 补 NAZHI_LOG_LEVEL / NAZHI_LOG_FORMAT / NAZHI_LOG_FILE 三个环境变量示例，并披露 file download 与 upload 同为 30 秒超时档（commit `59b546d`）。
- typical-case submit 示例去除手填 typeName：示例引导用户手填展示名会使 SDK 自动补全链路失效，改由代码映射自动生成（与 honor add 示例同款修正）（commit `59b546d`）。
- 自我评价 *JSON 透传方法补充对账口径披露：前端唯一读取通道是 dataMap，双容器并存时透传内容可能与网页所见不一致（commit `59b546d`）。
- UserUpdateInput.Seat 注释明确字面 "0" 视为跳过不发送；如需强制清零走裸 map 路径（commit `59b546d`）。
- newCleanClient godoc 披露上传/下载通道超时下限 30s（小于该值静默上浮并告警）与无超时时 5 分钟兜底（commit `59b546d`）。

## [1.5.1] - 2026-08-25

### 修复

- 上传被服务端拒绝（文件类型不收/风控）误判为可重试的服务端故障：errors.go 16 个哨兵中 `ErrUploadRejected` 此前漏配 `mapSentinelToHTTPCode`，落 default 500/exit2；本轮补 case 归 422/exit1（与 ErrFileTooLarge 同族）。新增 TestMapSentinelToHTTPCode_UploadRejected 锁定（commit `1522446`）。
- GetSchoolID 断网/超时场景下学号泄漏：`request.go:327`（ErrTimeout）与 `:329`（ErrNetwork）两条 do() 网络层失败分支嵌入裸 `url`，与同文件其他六处已正确使用 `logx.RedactBody(url)` 脱敏不一致；修复后错误消息形如 `"请求 ...?userName=*** 失败:"`，参数名保留、值掩蔽。回归测试 `TestLogin_GetSchoolID_NetworkError_DoesNotLeakUsername` 锁定（commit `90ccd64`）。
  - **已知上限**：stdlib `*url.Error` 内嵌原始 URL 是 net/http intrinsic 行为，超出 SDK request.go 可控范围；更上层防御纵深挂在 cmd 层 printError 出口。

### 新增

- UploadFile 非图片直传白名单加入 .pdf（原样直传，与 doc/zip 同路径）；新增 file_upload_pdf_test.go 锁死「字节不改写、文件名保留、超限本地拒绝」三行为。用户需求：典型案例附件需支持 PDF（commit `f1a28d1`）。
- 非图片附件直传上限放宽至 20MB：服务端实测无 2MB 硬限（真实上限约 46.86MiB，2026-08-25 字节级二分探测），SDK 上限取用户决策的 20MB；前端仍限 10MB，CLI 直传不受前端约束（commit `2e69e21`）。

### 文档

- 参考镜像 `reference/nazhi/src/components/classic/classiccanter.vue` 两处上传提示（:147/:206）随 SDK 上限放宽同步插入 pdf、把「大小不超过2MB」改为 20MB（commits `ac2986a`、`1e34350`）；`reference/nazhi` 是镜像源、不是产品代码，但保持与上游线上文案一致便于用户对照。

## [1.5.0] - 2026-08-24

### 修复

- 上传图片按 EXIF Orientation 自动摆正：decodeImage 改用 imaging.AutoOrientation，竖拍照片经 CLI 上传不再横置（对齐前端 canvas drawImage 的现代浏览器默认行为）。
- FetchTasks 聚合结果按维度声明顺序稳定输出：ParallelDims 原按 goroutine 完成序追加导致同账号两次 task list 顺序抖动，现与 FetchTasksJSON 的保序策略对齐。
- 写实 content 超过 200 字显式拒绝：前端 el-input maxlength=200 为浏览器硬截断、线上恒发不超过 200 字；SDK 不再放行超长原文，返回 ErrInvalidPayload。
- 任务提交状态判定改子串匹配：「已结束 未提交」等自由文案变体不再误判为已提交。
- CircleRecord.LikeList JSON 键名修正为 likeList（真实 API 返回 camelCase），字段恢复可解码。
- 自我评价查询别名链收窄：移除无前端依据的投机键 content/teacherRemark，统一 snake 主读、camel 兼容。

### 破坏性变更

- AddHonorPayload.CertImgAttachmentID 类型 string → int64：出站对齐前端裸 number、无附件省略键；入站继续兼容 number/数字字符串/空串/null。直接以字符串字面量赋值该字段的 Go 调用方需改为数字。
- AddHonorPayload.Name 加 omitempty：前端 addHonor 表单不含 name 键，空 Name 不再出现在请求体。

### 加固

- 非图片附件上传先 os.Stat 预检大小再读入内存；CLI @file payload 与 stdin 对齐受 16 MiB 上限保护。
- tokenparse 补充 JWT exp 提取的实现注释；panic recover 退出码契约注释修正为实际值 2。

## [1.4.1] - 2026-08-24

### 变更

- `task submit` / `task edit` 新增图片数量上限校验：pictureList 合并后超过 2 张返回 ErrInvalidPayload，对齐前端 el-upload `:limit="2"` 约束。
- CLI 帮助文本与错误文案专业化修正：task preview 全文中文化；file download 的 jq 批量下载示例兼容全量/limit 双模式输出形状；login 补充 NAZHI_OCR_BASE_URL / NAZHI_OCR_MODEL 可选变量披露。

### 文档

- README/源码指引对照前端源码全面复核修正：envelope 双层 code 判成功语义澄清、哨兵错误数量对齐源码、荣誉功能证据文件指向 performanceM.vue。
- 工程注释治理：清除全部审计编号标记（注释与测试标识符）、历史修复叙事、失效引用；doc comment 与实现一致性修正。

## [1.4.0] - 2026-08-24

### 新增

- SDK `PreviewSubmitPayload` / `PreviewEditPayload` 与 CLI `nazhi task preview [--edit]`：与 SubmitTask/EditCircle 共用 buildTaskPayload 组装链路、不发请求，如实暴露任务预设（circleTaskId/circleTypeId/dimensionId/hours/pictureList），空 address/orgName/level 保持空串不发明默认值；预览为纯组装不上传 ImagePaths。
- `types` 新增写实等级常量 TaskLevelNational..Grade（1..6）+ TaskLevelName、审核情况 CheckResultExcellent..Poor + CheckResultName，对齐原生字典 cateCode=23。
- `AddTypicalCasePayload.UnmarshalJSON` 兼容前端表单回传的 type/role/level 数字与 "1.0" 浮点格式（flexStringFromNumber），attachmentId 兼容空字符串。
- FetchTasks 迁移至 ParallelDims 泛型并发 helper（行为等价由回归测试锁定）；CLI 组装层收敛为 assembly 深 Module（ProcessScope 统一进程级资源管理）。
- 集成测试真读链路注入 OCR（env/Nazhi-auto 配置 fallback），SubmitTask HAR 场景改用动态生成图片夹具。
- 日志系统增强（全流程可追踪）：新增 pkg/logx 薄封装（基于 stdlib slog，零新依赖）提供 Level/Format/File 解析、脱敏与 traceId 上下文；CLI 新增 --log-level debug/info/warn/error、--log-format text/json、--log-file 路径三旗标及对应 NAZHI_LOG_LEVEL/FORMAT/FILE 环境变量，兼容旧 --verbose（等价 debug，仅当未显式传 --log-level 时生效）；--quiet 仅静默 stderr，文件仍落盘便于 CI 留痕；SDK 在 request.go 统一 HTTP 生命周期打点并经 context 透传到 auth/session/file 全链，错误按 ClassifyError 定级；敏感字段统一脱敏，验证码原文永不落地。

### 破坏性变更

- SDK 移除本地验证码识别器、相关模型/原生运行库及构建选项；所有 `Login` 调用方必须通过 `WithCustomOCR` 注入视觉识别器。CLI 默认使用硅基流动 Qwen3-Omni，纯 Go 构建不再需要 CGO 或额外模型文件。

### 文档

- 同步 README、CLI/SDK 分册、CI、Makefile 与 `CLAUDE.md`，明确验证码识别依赖注入契约和纯 Go 构建矩阵。

### 本轮审计与删除

- 前端源码复核后深度删除违规功能：移除 `ViolationRecord`/`ViolationType`、SDK 客户端方法、CLI 命令及专属测试；前端历史调用点不再属于当前 SDK 契约。
- 完成一次脱敏云端登录冒烟：CLI 使用本机运行时注入的 SiliconFlow Qwen3-Omni 密钥成功返回 200 envelope 和 token；密钥、账号和 token 未写入输出或仓库。

- CLI `nazhi honor update`：保留 SDK `UpdateHonor` 能力，对象 payload 走 `parseJSONObjectPayload`，自动空 typeName 反查（`GetHonorTypeOptions`）；典型案例批量删除 `nazhi typical-case delete-batch --payload '[1,2,3]'`：保留 SDK `DeleteBatchTypicalCase` 能力，纯 ID 数组 payload 校验非空/正整数。
- SDK `types.UserUpdateInput` 新增 `Birthday` 字段（对应前端 `updateMyInfo.birthday` 键）；`UpdateMyInfoStructured` 写入 wire key `birthday`，`Birthday` 优先、`BirthdayStr`（兼容旧调用）仅在 `Birthday` 为空时生效。SDK 原样透传，**不**做日期或时区转换（前端实际发送 ISO 8601 UTC）。
- CLI `nazhi self-eval grad-status` / `grad-submit`：透传前端毕业评价查询与提交。查询走 `QuerySelfGradEvaluationJSON` 保留 `dataMap.student_comment` / `isGrad` 原始字段；提交走 `SubmitSelfGradEvaluation` 单层 `{studentComment}`。
- CLI `nazhi honor levels --type-id`：透传 SDK `GetHonorLevel`，对齐前端按荣誉类型联动加载级别。
- CLI `nazhi honor type-options` / `level-options`：分别透传 SDK `GetHonorTypeOptions` 的 `dataList` 类型选项与 `GetHonorTypeForSelect` 的 `returnData` 通用等级选项，避免两种下拉语义混用。
- CLI `nazhi task dimensions`、`task circle-type --task-id`：`nazhi task dimensions` 透传 SDK `GetDimensions`；`task circle-type` 透传 SDK `GetCircleTypeByTaskID`，自动拒绝非正整数 `--task-id`，不发请求。
- CLI `nazhi circle types --dimension-id [--pid]`、`circle tasks --type-id`、`circle images [--page] [--page-size]`、`circle dict --cate-code`：分别透传 SDK `GetCircleTypes`/`GetCircleTasks`/`GetCircleImages`/`GetDictList`，正整数 flag 在非法时立即走参数错误路径（退出码 3）。
- CLI 登录可接入 Nazhi-auto 同款硅基流动 Qwen3-Omni：设置 `NAZHI_SILICONFLOW_API_KEY`（兼容 `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY`）后通过 `WithCustomOCR` 注入；密钥不入库。

### 测试

- 集成守卫 `TestNoRealPII` 抽出 `piiSkipDir` 辅助函数并新增 `TestPiiSkipDirSkipsNestedGitRepo` 回归：自动跳过嵌套 git 仓库（worktree / 子模块等），避免旧 worktree 中残留的早期 PII 夹具持续误报 `go test ./...`；主仓库 `.git`、经典 `vendor` / `node_modules` 跳过规则保持不变。

### 修复

- SDK `GetMyInfo` 的 `className` 后处理与前端 `userBox`、`modifyBox`、`header` 对齐：只移除首个“级”字，不再按 `gradeName` 删除前缀。
- CLI stdin payload 超过 16 MiB 时返回参数错误，避免读取限制造成静默截断。
- CLI 写实 payload 将 `level`、`checkResult`、`playRole` 的合法整数 number 规范为标准十进制代码字符串，同时继续拒绝小数、非有限值和溢出值。
- CLI `self-eval submit --payload=` 显式空值立即返回参数错误，不再误走 stdin/纯文本模式。
- 修复写实列表分页测试夹具的并发计数竞态，`go test -race` 不再因测试自身的共享计数器误报。

### 文档

- **docs 重组**：删除设计类单页（architecture / login-flow / OCR / env-vars / har-testing / migration_v2 / api-coverage）；SDK 按功能域分册（`docs/sdk/*.md`）；CLI 精简为单文件并并入环境变量速查
- 根 README / 文档中心仅链 CLI + SDK 分册
- **自动补全总表** [`docs/sdk/autofill.md`](docs/sdk/autofill.md)：对照源码列出 Login 按学号查学校、GetMyInfo 用学号补 schoolId/schoolName、写实 hours/元数据/图片、荣誉 typeName/name/score、典型案例 *Name、用户中文映射与禁止发明默认等；各域分册补「用户输入 vs SDK 自动」；CLI 增加「SDK 自动补全对照」节
- 文件域 SDK 文档对齐 `UploadFileResult` 的 `attachmentID` / `attachmentName` 输出，并将 `DownloadFile` 文案改为“跟随 HTTP 重定向”
- CLI 文件上传示例统一使用实际 flag：`nazhi file upload -f ...`

### 破坏性变更 (BREAKING)

- SDK 写实列表 `*JSON` 方法签名新增 `key string`：`GetSubmitted/Teacher/Withdrawn/PublicCirclesJSON`、`*LimitJSON` 及 `getCirclesJSON`/`getCirclesLimitJSON` 内部贯通关键字筛选（此前硬编码 `key=""`）
- SDK `GetHonorList` / `GetHonorListJSON` 签名新增 `key string`（此前硬编码 `&key=` 空值）
- SDK `CircleRecord` 结构化字段 JSON tag 对齐真实 API 混用命名（见「修复」）：依赖错误 snake_case tag（如 `img_list`/`is_my_self`）的调用方需改用真实键或字段访问
- `CircleRecord.IsMySelf bool` 重命名为 `IfMySelf int`（前端 `ifMySelf==1`）
- `GetTypicalCaseList` / `GetTypicalCaseListJSON` 增加可选 `status ...int`（默认 3=全部）；三参数旧调用仍兼容
- `CircleRecord.PlayRole` 类型由 `string` 改为 `PlayRoleCode`（JSON 数字/字符串均可解码；比较请用 `string(rec.PlayRole)` 或 `.String()`）
- 写实提交：空 `Address`/`OrgName` **不再**回落学校名；空 `Level` **不再**默认 `"5"`。依赖旧便利默认的调用方须显式传值

### 新增

- CLI `nazhi task submitted|done|teacher|public|withdrawn` 支持 `--key` 关键字筛选（含 Peek 总数路径）
- CLI `nazhi honor list` 支持 `--key` 关键字筛选
- CLI `nazhi typical-case list --status`（0 未审 / 1 通过 / 2 驳回 / 3 全部，默认 3）
- `TaskInput` / `TaskSubmitInput` / `TaskEditInput` 新增独立 `TermName` 字段（不再误用 `CircleDate` 填 `termName`）
- CLI 新增 `printParamError`：参数错误固定 `envelope.Error(400)` → 退出码 3
- `HonorRecord.Status int`：荣誉列表审核状态码（前端 `scope.row.status`）
- 常量 `TypicalCaseStatusPending/Approved/Rejected/All`
- `HonorType.Score` + snake tag；`AddHonorPayload.Score`（默认 0 随请求发出）
- `AddTypicalCase` / `UpdateTypicalCase`：空 `typeName`/`roleName`/`levelName` 时按 code 自动补全（对齐 classiccanter 下拉）

### 修复

- CLI 各现有对象型 `--payload` 入口拒绝顶层 `null`、数组等非对象 JSON，统一走参数错误路径（退出码 3），避免零值请求或错误请求发出
- `AddTypicalCasePayload.UnmarshalJSON` 遵循标准部分解码语义：缺失字段保留实例原值，仅对 JSON 明确提供的空 `attachmentId` 归一为 0；避免 SDK 调用方复用 payload 时已有字段被意外清零
- `AddTypicalCasePayload` 兼容前端初始 `attachmentId:""`：空字符串/null 归一为零值并省略，无附件的前端原始表单可直接提交
- `nazhi typical-case list` 默认 `--page-size` 从 20 调整为前端一致的 10
- 修正 `honor.go` 顶部 `deleteHonorById` 端点注释为真实 GET
- `httpDo` 对非 2xx 走 `classifyHTTPStatus`，主业务路径可识别 429/5xx/4xx 哨兵错误
- `hasHostSuffix` 要求 exact 或以 `.`+suffix 结尾，防止 `evilnazhisoft.com` 受信绕过
- `assembleCirclesJSON` 空首页不再产生 leading comma 非法 JSON
- `getCirclesLimitJSON` 只请求 offset/limit 覆盖页，避免全量翻页再截断
- `FetchTasksJSON` cancel 路径对齐 `ErrRetryable`（与 `FetchTasks` 对称）
- `parseHours` 非法输入返回 `ErrInvalidPayload`，不再静默回退 meta.Hours
- `TaskAddCirclePayload.ID` 加 `omitempty`，新增写实不再发 `"id":null`
- `UpdateMyInfo` 成功后失效 `sm.cachedUserInfo`，避免同进程 `GetMyInfo` 返回更新前快照；新增 `InvalidateCachedUserInfo`
- `nazhi user update` 解析 `UserUpdateInput` 并调用 `UpdateMyInfoStructured`，友好键（genderName 等）正确 remap
- `QuerySelfEvaluation` 未提交评价时返回 `(nil, nil)`，不再把空成功误判为「所有解码器均失败」
- `UpdateHonor` 对称补全 `typeName`（与 `AddHonor` 共用 ensureHonorTypeName）
- `AddHonor` 空 `Name` 时回落 `TypeName`（对齐前端新增表单不传 name）
- `AddHonorPayload.UnmarshalJSON` 部分解码时保留未提供字段，证书附件 ID 继续兼容 number/string，并区分缺失与显式 null
- `GetCircleTypes` 对 `pid` 做 `url.QueryEscape`
- 历史识别并发 Option 不再覆盖 `WithCustomOCR` 注入的识别器
- 参数错误改用 `printParamError(400)`→exit 3（缺 token / payload 解析失败等）
- 写实列表 `Get*CirclesJSON` 部分页失败时输出 `envelope.Partial(207)` 保留已合并数据
- **CircleRecord 混用命名解析**：`imgList`/`imgPreViewList`/`commentList`/`likeStatus`/`ifMySelf`/`auditRemark`/`creationTimeStr`/`showName`/`imgPath`/`studentId` 对齐平台真实 JSON（此前 snake_case tag 导致结构化 API 静默丢字段；CLI `*JSON` 透传不受影响）
- **GetTypicalCaseList 注释与能力**：status=3 为前端「全部」而非「已提交」；支持按审核状态筛选
- **HonorType JSON tag**：`dimension_name`/`level_name`（前端德育说明表；此前 camel 导致空字段）
- **UpdateMyInfoStructured**：忽略 `NationalStudentNumber`（前端只读，防止误写学籍）
- **输入暴露原则**：用户手填字段进 Input；前端自动填字段由 SDK 补全（典型案例 *Name、荣誉 typeName/name/score）
- **CircleRecord.PlayRole**：类型改为 `PlayRoleCode`，兼容列表 API 的 number 与表单 string（前端 `switch(map.play_role) case 1/2/3`）；序列化统一为字符串码
- **SelfEvalStatus**：`UnmarshalJSON` 主解码 `student_comment`/`teacher_comment`（前端 mainLeft/selfgaintloss），兼容 camelCase；`QuerySelfEvaluation` 的 normalize 兜底仍保留
- **荣誉/自评协议文档**：补充荣誉各方法的真实 HTTP method/path、删除荣誉的 GET+`id` 查询参数，以及自评/毕业评价的 endpoint、请求体层级、`dataMap` 字段和当前 CLI 能力边界
- **荣誉/自评回归测试**：覆盖 `DeleteHonor` 的 GET+`id` query、结构化自评的双层 `studentComment` JSON 请求体，以及荣誉证书附件 ID 的 number/string 输入兼容
- **parseHours / TaskSubmitInput.Hours**：对齐前端 `hoursStatus`——任务元数据 hours>0 时用户可空（SDK 用预设）；hours≤0 且用户空 → `ErrInvalidPayload`（不再静默提交 0）；显式 Hours 始终优先
- **写实 Address/OrgName/Level**：去掉 SDK 发明的默认（空 Address/OrgName→学校名、空 Level→`"5"`）；与前端一致，空串原样提交；调用方须按活动类型自行填写
- **典型案例 *Name 映射**：对齐 classiccanter el-option——type `"2"`→「社会调查报告」、level `"1"`→「国际」（此前误为「社会实践报告」/「国家」）
- **UpdateTypicalCase 数字 code**：`fillTypicalCaseDisplayNamesMap` 支持 type/role/level 为 number 或 string（列表回填常为 number；此前仅 string 能自动补 *Name）
- **写实 CLI payload 类型兼容**：CLI 私有 JSON 解码 helper 兼容前端编辑回填的 `hours` number/string（保留小数），并仅接受 `level`、`checkResult`、`playRole` 的有限整数 number；可接受数字在进入 client 前统一为提交字段使用的字符串，字符串按原值保留。同时兼容 `circleTaskId` → `taskId`、`pictureList` → `imageIDs`，规范字段优先；`TaskSubmitInput` / `TaskEditInput` 公开 Go 字段和标准 JSON 解码语义保持不变
- **写实提交默认行为说明**：历史版本中的 Address/OrgName 学校名回落与 Level 默认 `"5"` 仅代表旧行为；当前版本保持前端语义，空值不自动替换

## [1.3.0] - 2026-07-18

### 新增

#### 类型定义扩充

- `CircleRecord` 扩充 30+ 字段：补齐前端 getStudentCircle 所有原始字段（hostName、rank、level、checkResult、patentType、activityName、sportsName、teamName、orgName、resultsName、obtainTime、specialtyTechnology、playRole、likeSpecialty1-3、operatorName、creationTimeStr、circleTaskName、showName、isMySelf、auditRemark、likeStatus、commentList 等）
- `CircleComment` — 新增写实评论类型
- `UserInfo` 扩充 8 字段：telephone、genderName、birthdayStr、youthLeagueFlag、nation、familyAddress、hobbies、idCard、idType
- `ExamResult`、`TermInfo`、`ExamInitInfo`、`ExamType`、`Course` — 新增成绩管理类型
- `ViolationRecord`、`ViolationType` — 历史版本曾新增的违规类型，当前版本已删除
- `Notification`、`NotificationListResult` — 新增通知消息类型
- `BonusInfo`、`BonusRank`、`BonusDetail` — 新增积分商城类型
- `DemocraticActivity`、`SelfEvaluationItem`、`MutualEvaluation`、`DemocraticResult`、`MutualPersonInfo`、`ClassStudent` — 新增民主评价类型

#### 新增 SDK 方法（8 个新文件）

- `circle.go` — 写实管理扩展：DeleteCircle、AddCircleComment、SetCircleLike、GetCircleImages、GetCircleTasks、GetCircleTypes、GetDimensionsBySchool、GetDictList
- `exam.go` — 成绩管理：GetExamInitInfo、QueryStudentExam（**v1.3.0 已删除，不再维护**）
- `democratic.go` — 民主评价：GetDemocraticActivities、GetDemocraticActivityByID、GetSelfEvaluationData、GetMutualPersonInfo、GetDemocraticResult、GetMutualEvaluationDetail、AddOrUpdateSelfEvaluation、AddOrUpdateMutualEvaluation（**v1.3.0 已删除，不再维护**）
- `violation.go` — 历史版本的违规记录实现，当前版本已删除文件、方法和测试
- `notification.go` — 通知管理：GetUnreadNotifications、GetNotificationByID、ReadNotification、GetAllNotifications（**v1.3.0 已删除，不再维护**）
- `bonus.go` — 积分商城：GetMonthBonus、GetHistoryBonus、GetBonusRank、GetBonusDetail（**v1.3.0 已删除，不再维护**）
- `file_bag.go` — 档案查看：GetTermList、GetStudentInfoForTerm（**v1.3.0 已删除，不再维护**）
- `user_update.go` — 个人信息更新：UpdateMyInfo

#### 新增 CLI 命令（6 个父命令）

- `nazhi circle` — 写实管理（delete、comment、like）
- `nazhi exam` — 成绩管理（query）（**v1.3.0 已删除，不再维护**）
- `nazhi violation` — 历史版本的违规查询命令，当前版本已删除命令树和专属测试
- `nazhi notification` — 通知管理（unread、read）（**v1.3.0 已删除，不再维护**）
- `nazhi bonus` — 积分管理（month、rank）（**v1.3.0 已删除，不再维护**）
- `nazhi user` — 用户管理（update）

### 构建

- 版本号：`1.3.0`

## [1.2.4] - 2026-07-18

### 新增

- SDK `EditCircle` — 修改已提交的写实记录
- CLI `nazhi task edit` — 修改已提交的写实记录
- SDK `TaskEditInput` — 修改写实记录的最小输入（与 TaskSubmitInput 对齐，新增 id 字段）

### 构建

- 版本号：`1.2.4`

## [1.2.3] - 2026-07-18

### 新增

- SDK `GetTeacherCircles` / `GetTeacherCirclesJSON` / `GetTeacherCirclesLimitJSON` — 获取教师代写的写实记录（type=2）
- SDK `GetWithdrawnCircles` / `GetWithdrawnCirclesJSON` / `GetWithdrawnCirclesLimitJSON` — 获取被撤回的写实记录（type=3）
- SDK `GetPublicCircles` / `GetPublicCirclesJSON` / `GetPublicCirclesLimitJSON` — 获取公示的写实记录（type=4，全班）
- SDK `PeekTeacherTotal` / `PeekWithdrawnTotal` / `PeekPublicTotal` — 轻量获取对应类型记录总数
- CLI `nazhi task teacher` — 获取教师代写的写实记录
- CLI `nazhi task withdrawn` — 获取被撤回的写实记录
- CLI `nazhi task public` — 获取公示的写实记录（全班）
- SDK `EditCircle` — 修改已提交的写实记录
- CLI `nazhi task edit` — 修改已提交的写实记录

### 重构

- `pkg/client/submitted.go` 提取通用 `fetchCirclePage` / `fetchCirclePageJSON` 辅助函数，支持按 `type` 参数查询不同类别的写实记录
- 原有 `GetSubmittedCircles` / `PeekSubmittedTotal` / `GetSubmittedCirclesJSON` / `GetSubmittedCirclesLimitJSON` 保持向后兼容

### 文档

- `docs/cli/README.md` 命令树 + 新增命令详细文档
- `docs/sdk/README.md` 方法签名表 + CLI 输出对照表更新

### 构建

- 版本号：`1.2.3`

## [1.2.2] - 2026-07-17

### 新增

- SDK `PeekSubmittedTotal` — 轻量获取已提交写实记录总数（内部 `pageNo=1&pageSize=1`，只拉 1 条）
- CLI `task submitted --count` / `task done --count` 改用 `PeekSubmittedTotal`，不再经过 `GetSubmittedCirclesLimitJSON`

### 文档

- `docs/sdk/README.md` 方法表 + 完整文档小节 + 代码示例

### 测试

- 4 个单元测试覆盖正常/零数据/业务错误/请求参数验证

### 构建

- 版本号：`1.2.2`

## [1.2.1] - 2026-07-17

### 新增

- `typical-case submit` CLI 命令 — 提交一条典型案例，payload 支持 `@file.json` 和 `-`（stdin）两种来源
- `typical-case list` CLI 命令 — 获取当前用户已提交的典型案例记录（分页），CLI 透传 SDK 原始 JSON 1:1 对齐
- SDK `AddTypicalCase` — 调用 `/api/studentCircleNew/addTypicalCase` 提交典型案例
- SDK `GetTypicalCaseList` / `GetTypicalCaseListJSON` — 调用 `/api/studentCircleNew/getTypicalCase` 获取已提交列表

### 类型

- `AddTypicalCasePayload` — 13 字段提交请求体（HAR 确认 type/role/level 为 JSON 字符串）
- `TypicalCaseRecord` — 16 字段列表记录（与提交 payload 不同的 Go 类型，列表响应中 type/role/level 为整数）
- `TypicalCaseListResult` — 列表统一返回对象（Records + Page）
- `TypicalCaseRoleHost` / `TypicalCaseRoleParticipant` 角色常量

### 构建

- 版本号：`1.2.1`

## [1.2.0] - 2026-07-15

### 特性

- `task list` 输出扩展：Task 结构体新增 18 个服务端原始字段（全部 omitempty，完美兼容已有输出）
- `TaskSubmitInput` 新增 14 个可选字段，暴露前端 addCircle 请求体全部参数（零值空串时保持原有 fallback 行为）

### SDK

- `Task` 新增字段：`schoolId`, `circleTypeId`, `creator`, `modifier`, `modifyTime`, `roleId`, `auditorSubjectId`, `stateType`, `areaId`, `areaTaskId`, `upPic`, `evaluatedNumber`, `unEvaluatedNumber`, `unsubmittedNumber`, `submitNumber`, `pictureList`, `classId`, `gradeId`
- `TaskSubmitInput` 新增字段：`Name`, `HostName`, `CircleDate`, `Rank`, `ActivityName`, `SportsName`, `TeamName`, `OrgName`, `ResultsName`, `ObtainTime`, `SpecialtyTechnology`, `LikeSpecialty1~3`
- `buildTaskSubmitPayload` 将新输入字段映射到 `TaskAddCirclePayload`，`OrgName` 空串时 fallback 学校名

### 文档

- docs/cli/README.md：task list 字段计数更新为 40 字段，新增 v1.2.0 变更说明段落
- docs/sdk/README.md：TaskSubmitInput 代码示例补充新字段展示

### 重构（BREAKING）

- **OCR 移除 primary/fallback 级联退避**：`c.ocr` 作为唯一识别通道，不再分 primary 降级两道循环。`ocrRecognizeWithRetry` 从 3 返回值改为 2（移除 `fallbackUsed`）。`LoginResponse` 移除 `FallbackUsed` 字段。`buildLoginResponse` 移除 `fallbackUsed` 参数。

### 移除

- 历史后备识别 Option 不再有两阶段 cascade
- `Client.fallbackOCR` / `Client.fallbackConc` 字段 — `Client` 结构体减负
- `defaultFallbackOCR` / `safeFallbackRecognize` — 死代码删除
- `ocr_fallback_test.go` — 5 个 cascade 测试全部删除
- `LoginResponse.FallbackUsed` 字段 — `LoginResponse` 精简到 2 字段

### 文档

- 全量文档同步 v1.2.0：architecture.md 架构决策 #5 移除 fallback 级联描述；Option 表删除 fallback 两行；Login 流程示例更新（移除 fallbackUsed）；login-flow.md LoginResponse 结构体同步；CLI/SDK 参考的登录响应示例更新；版本号全量同步至 v1.2.0。

### 构建

- 版本号：`1.2.0`

## [1.1.5] - 2026-07-12

### 文档

- task submitted / GetSubmittedCircles 示例更换为真实脱敏 JSON，注明含同班同学姓名/学号
- CLI 描述改为"含同班同学的姓名和学号"
- SDK 描述改为"含姓名、学号、正文、图片、审核状态"
- docs/README.md 版本表同步

## [1.1.4] - 2026-07-12

### 文档

- 全量版本号同步至 v1.1.4（README.md / docs/README.md / docs/cli/README.md 版本表、徽章、命令概述）
- docs/sdk/README.md CLI↔SDK 对应表修正：task list 改回 `FetchTasks`（非 `FetchTasksJSON`）
- 方法速查表新增 `GetSubmittedCirclesLimitJSON`

### 清理

- 移除误跟踪的 `test_get_school_id.go`（`go:build ignore` 调试文件）
- 补充 `.gitignore` 防止再次误跟踪

## [1.1.3] - 2026-07-12

版本号占位。未单独发布变更。

## [1.1.2] - 2026-07-12

### 特性

- `task submitted` 新增 `--limit` / `--offset` / `--count` 分页控制
- SDK 新增 `GetSubmittedCirclesLimitJSON(ctx, token, offset, limit)` 方法
- `--limit` + `--offset` 超出实际数据量时返回空数组，不报错
- `--count` 只拉第一页获取 TotalNum，不拉列表数据

### SDK

- 新增 `GetSubmittedCirclesLimitJSON` — 用深度扫描逐条分割 JSON 数组，不反序列化为 Go struct
- 翻了够 offset+limit 就停，不等 totalPage 全部翻完

## [1.1.1] - 2026-07-11

### 修复

- task list 恢复输出 SDK 最终业务模型（`FetchTasks`）而非原始 JSON（`FetchTasksJSON`），
  字段重新包含 `submitted` / `needPic` / `dimensionName` 等业务语义字段
- 修复 `FetchTasksJSON` 聚合时 `dimErrs` 的并发写入 data race
- 修复 `FetchTasksJSON` 跳过 `id=0` 维度后收发次数不一致导致卡死
- 修复 `FetchTasksJSON` raw JSON 拼接时多余逗号导致序列化失败

## [1.0.0] - 2026-07-10

重大破坏性更新：types 全面精简 + JSON tag 统一 camelCase + CLI 输出 envelope 化。
升级前请阅读 [MIGRATION.md](MIGRATION.md)。

### Breaking Changes

- types: UserInfo 51→10 字段 (删除 initials/pinyin/seat/gender/birthday/telephone/creationTime 等 41 字段)
- types: Task 18→11 字段 + 新增 ScopeClass/ScopeGrade/ScopeStage 常量 (删除 upPic/pushNum/score/creatorName/roleName/termID)
- types: CircleRecord 15→9 字段, CircleImage 5→1 字段 (Approved bool + 仅 AttachmentID)
- types: HonorType 8→5 字段, HonorRecord 17→9 字段 (approved bool 替代 status int)
- types: SelfEvalStatus 10→3 字段 (id + studentComment + teacherComment)
- types: LoginResponse 删 RawData 字段 (3 字段)
- 命名: 全 SDK 统一 camelCase JSON tag
- 时间: 时间字段改 time.Time (ISO 8601 + 时区序列化)
- CLI: 删 `nazhi school` 命令 (从 UserInfo 获取)
- CLI: 新增 `nazhi task done` 别名 (替代 `task submitted`)
- CLI: 退出码三分 (0/1/2/3)

### Added

- pkg/envelope/envelope.go (统一 envelope 包装)
- pkg/client/internal/convert.go (helper: MapTaskStatus/MapSchoolID/ParseServerDate/MapCircleApproved)
- ScopeClass/ScopeGrade/ScopeStage 常量
- SDK `DownloadFile(ctx, attachmentID, dst)` — 按附件 ID 下载图片到本地。
  入口 `ssoBaseURL/common/attachment/getImg?id=X`，跟随 302 到 FastDFS 真实存储；
  CheckRedirect 同域白名单（nazhisoft.com）+ 5 次上限；不发任何鉴权头。
- CLI `nazhi file download` — 按附件 ID 下载图片。
  ```
  nazhi file download --id 5006375 --output ./photo.jpg
  ```
  不接受 `--token`（公开服务）；urlType=`sso` 走 SSO 域名。

### Removed

- types/score 字段 (Task/HonorType/HonorRecord)
- types/initials/pinyin 字段 (UserInfo)
- 全部 omitempty 派生字段 (UserInfo seatSort/telephone/email 等)
- 重复字段 (UserInfo.studentName/SelfEvalStatus.schoolId 等与 UserInfo 重复)
- nazhi school CLI 命令

### Changed

- UserInfo/Task/CircleRecord/HonorRecord/SelfEvalStatus 字段重命名 (status→submitted/approved)
- circleTaskStatus 字符串 → submitted bool (简化版)
- circleDate/getDate 字符串 → time.Time (自动序列化 ISO 8601)
- `GetMyInfo` 学校信息 SSO 降级条件放宽 — 原来仅 `schoolId==0` 时触发，现放宽到 `schoolId==0 || schoolName==""` 任一缺失时通过 `GetSchoolID` 公开 API 补全。学校名缺省但 ID 存在时也能自动补上学校名称。

### 文档

- 全量脱敏示例刷新 — `docs/cli/README.md` 与 `docs/sdk/README.md` 全部 CLI 命令和 SDK 方法的输出示例替换为真实测试验证后的脱敏响应（含 login / session activate / whoami / task list / task submit / task submitted / self-eval / honor / file 全系列）。

### 修复

- Windows flaky timing 测试：3 个 `TestFetchTasks_*` 在 Windows 慢机器偶发失败
  - `TestFetchTasks_MixedBizAndCancel_FailedCountAccurate`: ctx 500ms → 1.5s
  - `TestFetchTasks_ContextCancel_ReturnsErrBusinessRejected`: ctx 1s → 1.5s
  - `TestFetchTasks_Parallel`: bounds 重写为 `warmupOverhead+perDimDelay+slack`
    （warmupOverhead=800ms，反映 Windows session warmup 真实耗时）
  - handler sleep 300ms → 2s（确保 dim 必被 ctx cancel 而非正常完成）
  30 次连跑零失败。

### 清理

- `.golangci.yml` 精简 — 移除 godot/godox linter（减少 lint 噪音），清理注释冗余。
- `errors.As` 替换裸类型断言 — `switch e := err.(type)` 改为 `errors.As`，兼容 wrapped error。
- switch exhaustive 补全 — `image_prep` 中 `format` switch 补 `default` 分支防漏。

## [0.6.0] - 2026-07-04

### 特性

- OCR primary+fallback 双策略降级 — primary OCR 全部失败后自动降级到内置 ddddocr 重新识别新图，提高验证码识别成功率（v0.5.0 引入，本篇正式记录）。

### 修复

- ocr_fallback_test.go 格式修复 — `gofmt` 对齐不一致导致 lint 失败。

### 构建

- 版本号: `0.6.0`

## [0.5.2] - 2026-07-04

### 修复

- validateCaptcha 重试修复 — 之前 OCR 识别成功后只调一次 `validateCaptcha`，校验失败直接退出 Login，浪费剩下 8 次重试预算。修复后将 `validateCaptcha` 移入 `ocrRecognizeWithRetry` 循环内部，校验失败时 `continue` 换图重试，用完 9 张图预算为止。错误链保持 `errors.Is(err, ErrLoginRejected)` 兼容性。`Login()` 不再单独调 `validateCaptcha`，外层流程精简。
- warnIfExpiresAtFallback nil 守卫 — 两处 `c.logger.Warn` 未检查 `c.logger` 是否为 nil，可能导致 nil 指针 panic。
- GetSubmittedCircles ctx 取消返回 error — 翻页过程中 context 取消时，之前返回 `(all, nil)` 掩盖错误，改为 `(all, err)` 让调用方感知截断。
- self-eval ReadString 错误传播 — `ReadString(0)` 的 error 被 `_` 丢弃，真实 I/O 错误被掩盖。
- honor 死代码删除 — `AddHonor` 中 `_ = resp` 无意义，改为 `_, err :=`。
- GetSubmittedCircles merge 误改修复 — 正常路径应返回 nil error，merge 冲突错误地改成了 `return all, err`。

### 重构

- cmd payload 抽取公共 `parsePayloadFromArg` — task_submit/honor 重复的 `@file.json`/stdin 读取逻辑归一到同一 helper。
- opt_builder switch case 替换为 map 查找 — 3 处相同的 switch 结构简化为一次 map 构建 + O(1) 查找。

### 清理

- OCR/types 注释与参数清理 — NewPool preload 废弃标记、BusinessError 注释代码示例清理、LoginResponse 死字段历史注释清理。

### 测试

- TestGetSubmittedCircles_CancelDuringPaging 竞态修复 — 同步点从 page 1 handler 响应后移到 page 2 handler 被调用时，消除 goroutine 调度不确定性。

### 构建

- 版本号: `0.5.2`
- .gitignore: 补充 `.git-rewrite/`、``、`angle5-findings.json` 忽略规则。

## [0.5.1] - 2026-07-03

### 修复

- commit 消息 @ 前缀违规 — `feat(task):` 的 commit 消息以 `@` 开头，违反 Conventional Commits 约束，changelog 生成器解析异常。rebase 修复。
- honor.go 注释缩进 — `deleteHonorById` 行多一个制表符缩进，已对齐。
- CLAUDE.md 版本/OCR 参数同步 — 仍引用 v0.4.1 版本号和 `maxOCRImagesTotal=99`，更新为 v0.5.0 和 9。

## [0.5.0] - 2026-07-03

40 个 commit，自 v0.4.1 以来的完整变更。

### 新增

- 荣誉申报 SDK（honor.go） — 5 个方法：GetHonorTypes / GetHonorTypeForSelect / GetHonorLevel / GetHonorList / AddHonor。通过 TDD 驱动开发，11 个单元测试全部通过
- nazhi honor CLI 命令 — `nazhi honor types` / `nazhi honor list` / `nazhi honor add` 三个子命令，支持 `@file.json` 和 `-`（stdin）两种 payload 来源
- nazhi task submitted CLI 命令 — 调用 `GetSubmittedCircles` 获取已提交写实记录，自动翻页合并输出
- `--payload -`（stdin 读取） — task submit 和 honor add 都支持从 stdin 读取请求体 JSON

### 改进

- docs SDK 参考 — 新增 submitted.go 和 honor.go 完整章节正文（含代码示例、错误说明、分页策略）
- docs CLI 参考 — 新增 `nazhi task submitted` 和 `nazhi honor {types,list,add}` 完整文档章节
- 文档同步 — README.md / docs/README.md / docs/architecture.md / docs/env-vars.md 同步 honor + submitted 相关内容
- `parsePayload` 抽取 — task_submit.go 将从文件/从 stdin 读取 payload 的逻辑抽取为独立 helper，honor add 复用相同模式

## [0.4.1] - 2026-07-02


### 新增

- `parallel.go` — CLAUDE.md 候选 #6：`ParallelDims[T any]` 泛型并发维度查询 helper，含 `ParallelDimsResult` + 错误聚合。81 行（vendor 化），`FetchTasks` 后续可迁移
- `error_category.go` — CLAUDE.md 候选 #7：`ClassifyError(err) ErrorCategory` 枚举（`ContextCancel` / `ContextTimeout` / `NetworkTimeout` / `BusinessError` / `Unknown`）。80 行，`task.go` `isContextError` 已使用
- `internal/recoverx` 包 — 统一 panic recover 策略，`RecoverPanic(recovered, sentinel, name)` 输出 `debug.Stack()` 到 stderr。auth.go / session.go / main.go 3 处调用点全部收敛
- `tokenparse` 3 个哨兵错误 — `ErrTokenReturnDataEmpty` / `ErrTokenTypeMismatch` / `ErrTokenFieldMissing`。`ExtractFromReturnData` 调用方可精确区分 3 种解析失败
- 5 个 HTTP 状态码哨兵错误：`ErrRateLimited`（429）/ `ErrServiceUnavailable`（5xx）/ `ErrTimeout`（超时）/ `ErrInvalidResponse`（4xx-其他）/ `ErrRetryable`（ctx cancel 可重试）。SDK 用户通过 `errors.Is` 精确识别 HTTP 层 / 业务层错误
- `doBizGet` 按 StatusCode 自动包装 sentinel：429 → `ErrRateLimited`，5xx → `ErrServiceUnavailable`，4xx → `ErrInvalidResponse`，不再笼统 "ErrNetwork"
- `isTimeoutError` helper：`c.do` 内部识别 `context.DeadlineExceeded` / `*url.Error.Timeout()` / `net.OpError.Timeout()`，用 `ErrTimeout` 包装

### 修复

- 顶层 panic recover exit code 1 回归 — 之前 `pendingExitCode=0` 走 exit 0，与正常 error 不一致。修复 `printError` 递归 fallback 设 `pendingExitCode=1`
- OCR Pool 加 `sweepStaleTempDirs` 启动时清扫 — `nazhi login` 顺手 best-effort 扫 `%TEMP%` 历史残留，能删的删
- `fetchTasksForDimension` panic recover 错误链保留 — `defer recover` 用 `%w` 包装原始 error
- `SetLimit(0)` 死代码修复 — `errgroup.SetLimit(0)` 在 `len(dimensions)==0` 时死路径，调最小为 1
- PII 守卫 AST 自检盲区修复 — 字符串拼接绕过的扫描覆盖
- `task list` cancelledCount 虚高修复 — 占位 error 不计入 `failedCount`
- `cookie_sync` partial decode 防御 — `dec.More()` 检查 reader 残留，partial 时 RawData 置 nil
- `ErrFileTooLarge` 错误链修复 — `errors.Join(ErrFileTooLarge, ErrImageTooLarge)`，errors.Is 单一识别所有"文件过大"路径
- `Login` body 摘要 — 非预期状态码错误消息附 `logSafeBody(bodyBytes)` 100 字节截断

- 注释中文化 — magic bytes sniff、`multipartBufPool` Grow 等
- `getQualitySteps` 内联为 `qualityAfterOptimization` 常量
- 结构化日志 — `warnIfExpiresAtFallback` 改为结构化 slog 字段
- `logSafeBody` 提取变量消除重复
- `isContextError` helper 消除 3 处重复
- self_eval 空值兜底 guard 删除（无消费者）

- `ErrTimeout` 包装 — `isTimeoutError` 出口处用 `ErrTimeout` 包装而非裸 fmt.Errorf
- `doBizGet` 包装 body 摘要 + ErrInvalidResponse fallback
- `atomic.Pointer[url.URL]` race 修复 — `c.baseURLParsed` 全部访问原子化

- `image_prep` 缩放级联简化 — 7 轮 resize 改为单次缩放（0.7^7 ≈ 0.082 常量计算），`getScaleFactors` 删除
- `decodeImage` 改用 `image.Decode` — 删除手写 magic bytes switch，stdlib 自动识别格式
- `prepareImageForUpload` 加 `ctx` 参数 — 支持超时取消
- `defaultOCR` 惰性预热 — 从同步 `NewPool(min(NumCPU,4))` 改为 `sync.Once` + `atomic.Pointer` 懒加载
- `Close()` 清理 sessionManager backoff 状态 — 避免复用 Client 时误触发冷却
- `New()` `url.Parse` 静默吞错改为 warn 日志
- `withURLGuard`/`withNilGuard` Option 工厂 — 消除 6 处 Option 重复守卫逻辑
- `--timeout 0` warn 回退 — 之前静默覆盖为正数超时，现在 warn 并保留默认值
- `valueToString` float64 精度保留 — 改用 `FormatFloat` 替代 `FormatInt` 截断
- `writeModelFile` 失败走 `cleanupTempDir` — 复用 DLL 占用降级逻辑
- `sweepStaleTempDirs` case-insensitive FS 下 `EqualFold` — 避免 Windows 大小写误判
- `initOnce` panic `%v` → `%w` — 保留 error chain
- `tryDecodeFallback` 删除 — 被 `doBizGetDecode` 吸收，不再需泛型 fallback helper
- `maxOCRAttemptsPerImage` 常量删除 — 设计意图代之以代码注释（架构深化后单图 OCR 1 次策略已稳定）
- `getScaleFactors` 删除
- `countTasksByType` `int` → `float64` — 泛型 `sumValues[T int | float64]` 改为 `T int64` + `json.Number` 兼容
- `Close()` 末尾清 `sm.clearBackoff()` 
- `maps.Clone` 删除 — `doGetMenu` 直接修改 map 而非 Clone，减少一次分配
- DCL fast path `cachedUserInfo` nil guard

### 改进

- `c.logger.Warn` 资源警告统一走用户注入 slog — 不依赖 cmd 通道，SDK 纯净
- `go.mod` 模块单一 — 仓库只有一个 `module github.com/Wenaixi/nazhi-cli`
- 文档全面升级到 v0.4.1 — CLI / SDK / 架构 / 登录流程 / OCR / HAR / 环境变量全部同步

### 构建

- 版本号：`0.4.1`
- make build 仍缺 `-tags=ddddocr`（已知坑不变）
- 新增 `internal/recoverx` 包 — 零依赖，无测试文件（由 3 个客户端隐式覆盖）

## [0.4.0] - 2026-06-30

v0.3.5 → v0.4.0 之间合入 305 个 commit，172 个文件改动。

### 新增

- session 激活 4 入口收口为 1 个公开方法 + 1 个内部 fast-path；`sessionManager` 封装 `SetBackoff`（d≤0 守卫）与 `tryActivate`
- HTTP helper 私有化（`doRequest` → `httpDo`、`doRequestWithResp` → `rawDoWithResp`），公开 API 保持不变
- `pkg/types/response.go` 新增 `DecodeUnified()` 原语（组合 `DecodeResponse` + `CheckCode`）
- 新建 `pkg/tokenparse/` 包封装 SSO token 解析（Location 头 + returnData）；泛型 `DerefOr[T]` 升到 `pkg/types/deref.go`；auth.go 瘦身约 40%
- `extractModels` 建好本进程目录后扫一遍 `%TEMP%` 下其他 `nazhi-cli-ocr-*` 残留，能删的删（已退出进程）、删不动的跳过（其他运行实例），绝不误删其他程序目录。Windows 登录后 `%TEMP%` 不再无限堆积

### 修复

OCR Windows 三轮 TDD 修复：

- `5ff0ea8` Windows DLL 占用降级：`Close` 时删 `onnxruntime.dll` 因 `LoadLibrary` 句柄未释放被拒（`Access is denied`），抽 `cleanupTempDir` 对 Windows 两类 errno（`ERROR_ACCESS_DENIED` / `ERROR_SHARING_VIOLATION`）降级返 nil。stderr 不再被权限错误污染
- `a81c9f3` GOOS 守卫：上一轮注释承诺「非 Windows 永远 false」但代码不保证（Linux errno 5=EIO、32=EPIPE 也会命中），加 `goosFn` 注入点 + `runtime.GOOS == "windows"` 守卫，降级只在 Windows 生效
- `7d5dd65` 启动时清扫：见新增段最后一条

`SetBackoff` race 修复；`main` panic recover 走 `closeAllClients` LIFO；`--output` 死代码删除；`Login` 并发（CallStep 改 mutex）；顶层 panic recover 输出 `debug.Stack()` 到 stderr；`buildLoginResponse` 非法 JSON 保底 `RawData` 为空 map（Finding #8）+ RawData 置 nil 消除二次解析；`image_prep` 缩放级联优化（resize N 次后统一 encode）；Transport 加 `TLSHandshakeTimeout=10s` 防网络挂起；`tryActivate` 先检查 `ctx.Err()` 再检查 backoff（避免被掩盖）；`bizURL` helper 集中处理裸 baseURL 拼接；`noRedirect` 共享变量；OCR Pool `o.mu` panic defer Unlock 防死锁；引用已删接口的文档清理；中文注释规范化。

### 兼容性

- `client.New(...)` 仍返 `(*Client, error)`（v0.3.1 起的契约不变）
- `Login` / `ActivateSession` / `FetchTasks` / `SubmitTask` / `SubmitSelfEvaluation` / `QuerySelfEvaluation` / `GetMyInfo` / `UploadFile` / `GetSchoolID` 业务方法签名与 v0.3.5 完全一致
- 环境变量清单（`NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN` / `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL` / `NAZHI_TIMEOUT`）与 v0.3.5 一致
- `file upload` 仍不接受 `--token`（独立公共服务，不需要业务域 token）

### 构建

- 版本号：`0.4.0`
- 所有路径显式 `-tags=ddddocr`；Makefile `build` target 仍缺 tag（已知坑，需手动 `go build -tags=ddddocr`；CI 已正确）
- 测试：45+ OCR 测试 race + ddddocr 双 tag 全绿；新增 `ocr_win_cleanup_test.go` / `ocr_sweep_test.go`
- 跨平台：5 平台（linux/darwin/windows × amd64/arm64）vet 通过；macOS x86_64 不支持（Microsoft 已停发）

## [0.3.5] - 2026-06-26

### Features

- OCR 可选构建 — 新增 `-tags ddddocr` 编译标签。不加标签时编译为纯 Go 二进制（无 CGO），`login` 命令会返回明确提示指导使用 `WithCustomOCR` 或下载预编译 release。CGO-free 嵌入式场景不再被 onnxruntime 强制依赖阻塞。
- 新增 3 个错误哨兵 — `ErrOCRNotConfigured`、`ErrEmptyUserInfo`、`ErrSessionBackoff`。SDK 用户可用 `errors.Is` 精确区分 OCR 缺失、空用户信息、session 背压三种场景。

### Fixed

- 文件上传 multipart 缺少终止边界 — 修复 upload 请求体尾部缺 `--boundary--\r\n` 导致服务端解析失败。
- GIF 上传背景变黑 — 修复透明 GIF 合成白底时走特殊路径导致的回归。
- 图片压缩失败死循环 — 修复 JPEG 编码失败时无限重试导致 CPU 100%。
- CLI 退出泄漏 ONNX 资源 — 修复 `os.Exit(1)` 跳过 defer 导致临时目录永久残留。
- 上传命令误读 NAZHI_TOKEN — 修复 `file upload` 将 `NAZHI_TOKEN` 环境变量误写入 sso 域 Cookie 的问题。
- OCR 并发关闭泄漏 — 修复池关闭后新创建的 OCR 实例不被清理、临时目录泄漏。
- 不同 token 共享 session 背压状态 — 修复 A 登录失败导致 B 也被误判为激活失败。
- session 激活并发安全 — 修复 `ActivateSession` 无 mutex 保护导致并发请求数据污染。
- 空用户信息被当错误处理 — 修复 `GetMyInfo` 返回空数据时误报错误。
- 任务列表部分维度失败时空白 — 修复部分评价维度请求失败时整个列表不输出成功数据。
- 空消息导致日志 panic — 修复 `resp.Msg` 为 nil 时无保护解引用。
- 维度抓取 panic 崩溃进程 — 修复某维度请求异常时整个 CLI 进程退出。
- 11 处 PII 残留 — 替换测试文件和文档中残留的真实姓名和学号。
- HTTP 连接池限制 — 默认 `MaxIdleConnsPerHost=2` 不够用，改为 16 避免高并发反复 TLS 握手。
- Debug 日志无谓分配内存 — 非 Debug 级别不再为日志参数做 `fmt.Sprintf` 分配。
- Base URL 拼接不统一 — 3 处直接拼接改为 `bizURL()` helper 集中处理。
- token flag 空字符串覆盖环境变量 — 用 `flagChanged()` 区分"没传"和"传了空值"。
- 顶层 panic 无保护 — 加 recover 统一 exit code 1，不打 stack trace。
- Session 背压无提示 — 捕获 `ErrSessionBackoff` 时输出冷却提示等待时长。
- context cancel 被任务抓取吞掉 — `FetchTasks` 的 goroutine 闭包检查 `gctx.Err()`。
- 文档残留已删接口引用 — 同步清理 `login-flow.md` 中已删 `GetCaptcha` 的说明。

### Changed

- OCR 可选构建 — `pkg/client` 不再强制导入 `internal/ocr`。无 `-tags ddddocr` 时编译为纯 Go，Login 返回 `ErrOCRNotConfigured`。
- 错误哨兵体系 — 新增 4 个哨兵，覆盖 Location 解析、OCR 缺失、session 背压、空数据场景。

### Build

- 版本号：`0.3.5`
- 新增构建变体：`go build -tags ddddocr`（含 OCR）/ `无 -tags`（纯 Go 无 CGO）
- CI 增加双构建变体验证

## [0.3.4] - 2026-06-26

### Fixed

- Token 过期时间不准 — 之前 200 路径始终用 `now+24h` 兜底，现在会解析服务端返回的 `exp`/`expires_in` 字段。
- GetSchoolID 死分支 — 删除了一个永远不会触发的 else-if 分支（服务端只返回 NAME 字段）。
- `derefOr` helper 简化 — nil-safe 字符串解引用，5 行变 3 行。
- `LoginResponse.RefreshAfter` 字段删除 — 从未被服务端填充过，删掉免得误导调用方。
- `UnifiedResponse` 6 个孤儿字段删除 — DataString、PageBean、Note、InsertID、UpdateCount、IsAttendance 全仓库 0 引用。
- drain+close 全部统一 — 所有 HTTP 请求的 body 关闭前都会先 drain 再 close，保持 keep-alive 连接可重用。
- 5+1 处业务错误用统一哨兵包装 — `SubmitSelfEvaluation`、`QuerySelfEvaluation`、`QuerySelfGradEvaluation`、`GetMyInfo`、`fetchDimensions` 的 CheckCode 改用 `ErrBusinessRejected` 而不是之前的各种散装错误。
- 维度抓取不静默吞错误 — 之前 `fetchTasksForDimension` 遇到业务错误只 logDebug 就返回 nil，现在会返回 error 让调用方知情。
- 上传客户端 50 次握手回归 — 修复新创建的 clean client 没复用 Transport 导致批量上传反复 TLS 握手。
- 6 个 Option 加校验守卫 — `WithSSOBase`/`WithBaseURL`/`WithUploadURL`/`WithHTTPClient`/`WithOCRConcurrency`/`WithToken` 遇到空值或负值时 warn + 保留原值。
- CLI 自动获得 Client 清理 — school 和 file_upload 改用统一 `buildClient` helper 后，自动获得 `trackClient(c)` 注册，退出时不再泄漏 ONNX 临时目录。
- `whoami` 空数据不报错 — 当 `GetMyInfo` 返回 `(nil,nil)` 时输出 `{"status":"empty"}` 而不是裸 `null`，区分"空响应"和"激活失败"。
- Session 激活失败背压 — 失败后缓存 + 5 秒冷却窗口，防止 N 个并发请求同时触发激活。
- 任务列表部分维度失败不吞成功数据 — 全失败返回全部错误；部分失败返回成功维度 + 错误信息；全成功正常返回。

### Changed

- `LoginResponse.RefreshAfter` 和 `UnifiedResponse` 6 个字段删除 — BREAKING API，全仓库确认 0 引用。旧 API 响应 JSON 反序列化兼容（Go 忽略未知字段）。
- OCR 进程级单例删除 — 不再有 `GetDefault`/`defaultOCR`/`defaultOnce`，由 Pool 替代。
- trackInit 改用 sync.Map — 99 次串行锁写 map 改为 `LoadOrStore`，key 已存在时 lock-free 跳过。
- 新增 `printPrompt` 函数 — 终端交互提示（如 self-eval 的"请输入评价"）走独立通道，不受 verbose 守卫，受 quiet + TTY 检测守卫。

### Build

- 版本号：`0.3.4`

## [0.3.3] - 2026-06-25

### Fixed

- HAR 测试数据含真实姓名和学号 — `self_eval.json` 的 `student_number`/`studentName` 仍有真实信息，替换为占位值，新增自动化扫描防止再出现。
- 图片处理 69 行死代码 — `prepareImageWithStats`、`prepResult`、`PrepStats` 结构体（14 字段）、`CompressionRatio` 方法全部未用，删除后 inline 到 `prepareImageForUpload`。
- syncCookieToken URL 解析失败静默 — 之前只有 Jar 类型断言失败会报错，URL 解析失败只打一条日志就返回 nil，现在统一返回 error。
- SubmitTask 业务错误用了错误的错误哨兵 — 业务 code≠1 时包装成 `ErrLoginRejected`，误导 SDK 用户走重新登录流程。新增 `ErrBusinessRejected` 哨兵专门用于业务拒绝场景。
- 上传客户端污染业务连接池 — `newCleanClient` 复用业务 Client 的 Transport，调用 `CloseIdleConnections` 时会误关业务请求的 keep-alive 连接。改用 `Transport.Clone()` 创建独立 idle 连接池。

### Changed

- `LoginResponse.UserInfo` 字段删除 — BREAKING API。登录响应从未填充过这个字段，用户信息请通过 `GetMyInfo()` 获取。

### Build

- 版本号：`0.3.3`

## [0.3.2] - 2026-06-25

### Fixed

- 集成测试编译 break — `client.New()` 签名改 `(*Client, error)` 后集成测试没适配，CI 编译失败。
- CLI 错误信息重复输出 — cobra 和 main 同时输出错误，终端看到两遍错误信息。统一由 `printError` 输出 JSON 格式。
- 200 登录路径缺少 token 过期告警 — 302 fallback 路径有兜底 warn，200 路径没有，不对称。
- Referer 头里的 token 没做 URL 编码 — 虽然 JWT 是 URL-safe 的，但防御性编程应使用 `url.Values.Encode()`。
- OCR 池并发关闭不安全 — 第二个 goroutine 关闭时第一个还在释放实例，可能重复释放同一 ONNX session。
- 任务抓取并发数不限 — 之前只留了 TODO 注释，现在加 `errgroup.SetLimit(8)` 限制并发。

### Build

- 版本号：`0.3.2`
- 新增依赖 `golang.org/x/sync v0.21.0`

## [0.3.1] - 2026-06-25

### Fixed

- 登录请求后没 drain HTTP body — 多个 early-return 路径直接 close 连接，导致 TCP 连接无法归还 keep-alive 池，高频调用下反复建连。
- Token 过期告警被静默 — expiresAt 兜底应打 Warn 级别，但误用了 Debug 级别，默认配置下完全看不到。
- 200 登录路径 unmarshal 失败被吞 — 错误信息只说"未找到 token"，丢了 body 内容这个关键诊断信息。
- syncCookieToken 静默失败 — 类型断言失败只打一条 warn 就返回 nil，build client 阶段完全感知不到，后续业务接口全空时才暴露问题。改返回 error，`client.New()` 签名调整为 `(*Client, error)`。
- OCR 重试不响应 context cancel — 99 次循环顶部没检查 ctx，用户取消后还会跑完所有重试。
- Session 激活并发安全 — 检查 state 后立刻放锁，4 步激活在无锁状态下执行，并发 goroutine 浪费请求且污染 cookie。
- Session 激活第 4 步失败被掩盖 — `getMyInfoRaw` 失败只打 debug 日志，调用方收到空 UserInfo 以为激活成功。
- WithTimeout 负数/零值没阻拦 — 0 值覆盖已有正数超时，导致请求可能永久挂起。
- `whoami` 输出 null 被当错误处理 — `GetMyInfo` 返回 `(nil,nil)` 时走 `printError` + 退出码 1，误导用户。
- `printError` 直接 os.Exit 绕过资源清理 — 跳过 `defer closeAllClients()`，ONNX session + 临时目录 + keep-alive 连接全部泄漏。改为标记退出码，统一在 main 末尾退出。

### Changed

- `client.New(opts ...Option) *Client` → `(*Client, error)` — BREAKING API。`syncCookieToken` 现在返回 error，`WithHTTPClient` 传了非 CookieJar 的 Jar 时会报错。12 个 cmd 调用点已用 `c, _ := client.New(...)` 适配。

### Build

- 版本号：`0.3.1`

## [0.3.0] - 2026-06-24

### Fixed

- io.ReadAll 错误静默丢弃 — 网络闪断时读 body 失败，错误没说清楚，只给一句误导性的"未找到 token"。
- 验证码图片读取失败时没 drain — 出错了也先 drain body 再 close，保证 TCP 连接可复用。
- ExpiresAt 零值 — 200 路径的登录过期时间返回公元 0001 年，改为 `now+24h` 兜底。
- syncCookieToken 兼容性 — 类型断言失败时输出实际类型和修复提示，方便排查。
- Session 激活不感知 token — 不同 token 共享同一个 session 缓存，切换 token 后可能返回旧用户数据。
- FetchTasks 没用 session 激活 — 与其他业务方法不一致，少了 `activateSessionIfNeeded` 调用。
- getMyInfoRaw 错误传播中断 — CheckCode 错误被截断，调用方收不到准确错误。
- sync.Pool 裸类型断言 — 没有 `ok` 检查，GC 回收后可能 panic。
- 上传客户端零超时传播 — 父 client 没设超时时上传请求无限等待，兜底 30s。

### Changed

- 重构 request.go — 提取 `buildRequest()` 消除 `doRequest`/`doRequestWithResp` ~40 行重复代码。
- CLI 提取 `buildBizClient()` — 消除 6 个命令文件各 ~15 行 env fallback + Client 构造样板，统一到 `cmd/nazhi/client_builder.go`。
- 请求日志加 debug guard — 非 Debug 级别不再每次请求都遍历 header。
- version 命令输出 JSON — `nazhi version` 输出 `{"version":"0.3.0"}` 统一输出格式。

### Build

- 版本号：`0.3.0`

## [0.2.2] - 2026-06-24

### Added

- Shell 自动补全 `nazhi completion [bash|zsh|fish|powershell]`
- 版本号子命令 `nazhi version`

### Fixed

- Session 兜底 body 读取 bug — `session.go:77` 中步骤 4 失败后 body 已被 defer Close 消耗的问题。

### Changed

- 文档 emoji 清理 — 全部文档和注释移除 emoji。
- Makefile — echo 消息纯文本化。

### Tests

- `TestActivateSession_*` 系列 — 5 个 session fallback 测试覆盖。

### Build

- 版本号：`0.2.2`

## [0.2.1] - 2026-06-24

### Changed

- OCR 重试策略：`3×33` → `1×99`。同一张图 OCR 结果是确定性的，重试无意义，换图才有效。
- Makefile echo：移除所有 emoji，输出保持纯文本。
- CHANGELOG / README / 文档：全部移除 emoji，统一风格。

### Fixed

- 测试性能：`TestPrepareImage_CompressesLargeImage` 从 3000×3000 降为 1500×1500 + Pix 直接填充（29s → 3s）。
- CI 全平台修复：10+ 轮修复后，5 平台（Linux amd64/arm64, macOS arm64, Windows amd64/arm64）全部构建通过。
  - Linux arm64：`gcc-aarch64-linux-gnu` 在 amd64 runner 交叉编译
  - Windows arm64：`zig cc` 在 amd64 runner 交叉编译
  - golangci-lint：`go install` 兼容 Go 1.26.1
  - softprops release：`continue-on-error: true` 处理新 release 404
- CLAUDE.md：OCR 并发策略、CI 修复历程、发布资产全部更新。

### Build

- 版本号：`0.2.1`

## [0.2.0] - 2026-06-22

### Features

#### 跨平台 OCR（5 平台）
- 5 平台 build tag 隔离的 `onnx_*.go` 嵌入文件（win/lin/mac × amd64/arm64）
- `ocr.GetDefault()` 进程级单例 + `sync.Mutex` 并发保护
- 99 次重试机制（同一图片）提高识别准确率
- 解压到磁盘目录供 `onnxruntime_go` 加载

#### 全自动验证码流程
- 简化 `Login()` 内部流程：InitSession → GetSchoolID → OCR → validate → 302/200 提取 token
- 优先处理 200 JSON 响应（HAR 验证），fallback 到 302 Location
- 移除所有手动/交互式验证码模式
- 自动 `syncCookieToken` 同步到 SSO + 业务域 Cookie

#### HAR 对齐的 4 步 Session 激活
- 步骤 1：GET / 初始化后端 Session
- 步骤 2：GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
- 步骤 3：GET /api/studentInfo/getMenu（Referer: /home）
- 步骤 4：GET /api/studentInfo/getMyInfo（返回完整 51 字段 UserInfo）

#### UserInfo 51 字段
- 完整暴露 `getMyInfo` 返回数据
- `birthdayStr` 字符串化（Java LocalDate JSON 数组兼容）
- 移除自定义 `Birthday` 类型

#### 图片自动压缩预处理
- 任意格式 → JPG（PNG/BMP/WEBP/GIF 支持）
- 透明合成（flattenOnWhite）
- 质量级联 → 缩放级联
- 上限 5MB
- 全部在内存中完成，不写盘

#### CLI 环境变量支持
- `NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN`
- `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL`
- `NAZHI_TIMEOUT`
- 命令行标志优先于环境变量（用 `flagChanged` 检测）
- `.env.example` 模板 + `.gitignore` 排除真实 `.env`

#### HAR 驱动集成测试
- 5 个 fixture 文件（task_flow、self_eval、military、class_meeting、labor）
- 6 个 HAR 驱动测试覆盖 FetchTasks、SubmitTask（4 种类型）、SubmitSelfEvaluation
- 真实环境 10 步端到端 `TestReal_FullChain`
- 4 个回归测试

#### 完整文档体系
- `docs/README.md` — 文档中心索引
- `docs/cli/README.md` — CLI 命令参考
- `docs/sdk/README.md` — Go SDK API 参考
- `docs/architecture.md` — 架构总览
- `docs/login-flow.md` — 登录流程详解
- `docs/cross-platform-ocr.md` — 跨平台 OCR 设计
- `docs/env-vars.md` — 环境变量参考
- `docs/har-testing.md` — HAR 驱动测试架构

### Fixes

#### Security
- 历史凭据泄露已修复（v0.1.0 之前）：通过 `git-filter-repo` 重写所有分支和 tag 历史
- CLI `--token` Cookie 同步：新增 `WithToken()` Option，CLI 传 token 时同时写 Header + Cookie
- UploadFile 禁用重定向：cleanClient.CheckRedirect 防止 302 跳转到攻击者主机

#### Bugs
- Task.StartDate 字段错配：从 `startDate`（数组）改为 `startDateStr`（字符串）
- extractTokenFromLocation URL 解析：从 `strings.Index` 改为 `net/url.Parse`，支持 fragment
- session.go 步骤 1/2 Body 泄漏：defer + io.Copy 模式
- QuerySelfGradEvaluation 错误被吞：所有路径失败时返回明确 error
- FetchTasks 静默失败：用 `c.logDebug` 记录（不破坏 API）
- output.go stderr 编码失败：加 `fmt.Fprintln` 兜底
- ImagePrep 兜底大小检查：避免返回超大文件
- stdin 无 TTY 阻塞：`isTerminalStdin()` 检测

#### Dead Code 清理
- 删除未使用的 4 个哨兵错误（ErrTokenExpired、ErrSessionExpired、ErrIncompleteResponse、ErrUnexpectedStatus）
- 删除未使用的类型（SchoolInfo、SessionInfo）
- 删除未使用的函数（EnforceCode、自定义 min）
- 删除 debug 工具目录（cmd/debuglogin/、cmd/reallogin/、cmd/getcaptcha/、cmd/ocrtest/）

### CI/CD

- 5 平台 native runner 矩阵（ubuntu-latest、ubuntu-22.04-arm64、macos-latest、windows-latest、windows-11-arm）
- 新增 `integration` Job：tag 发布时跑真实环境集成测试（需 secrets）
- 新增 `gofmt` 检查
- 新增 `go mod tidy` 验证
- 新增 SHA256 校验和
- 二进制 `--version` 验证步骤

### Build

- Go 1.26.1
- 单二进制分发（内嵌 OCR 模型 + onnxruntime）
- Makefile：`build` / `test` / `test-verbose` / `test-integration` / `lint` / `vet` / `fmt` / `release` / `clean`

## [0.1.0] - 2026-06-21

初始发布 — nazhi-cli：纳智综合评价自动化 CLI + Go SDK。

### Features

- SSO 全自动登录 — InitSession → GetSchoolID → 验证码处理 → Login 全流程
- 内置 OCR 验证码识别 — ddddocr 引擎 + 模型已内嵌至二进制，无需运行时下载
- 学校 ID 查询 — 根据学号获取学校信息
- 业务 Session 激活 — 登录后激活目标平台 API Session
- 用户信息查询 — 获取当前用户 profile
- 任务管理 — 列出任务 + 提交任务（支持 `@file.json` 读取）
- 自我评价 — 提交评价 & 查询评价状态
- 文件上传 — 本地图片上传至目标平台
- 跨平台构建 — Linux / macOS / Windows 三平台二进制支持

### Tech

- Go 1.26 + cobra CLI 框架
- ddddocr（ONNX Runtime）嵌入式验证码识别
- 单二进制分发，零外部依赖