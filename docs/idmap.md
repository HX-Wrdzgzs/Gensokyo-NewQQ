# idmap 数据库

## 架构

三库分离设计，各自独立的事务和文件锁，互不干扰。

| 文件 | 桶 | 用途 | 特点 |
|------|-----|------|------|
| `idmap-identity.db` | `ids` | OpenID ↔ 虚拟数字 ID | **永久数据**，固定大小 |
| `idmap-identity.db` | `config` | 运行时配置 | **永久数据**，应用级存储 |
| `idmap-identity.db` | `UserInfo` | 用户信息缓存 | **永久数据**，应用级存储 |
| `idmap-msg.db` | `cache` | 真实 message_id ↔ 虚拟 message_id | **临时缓存**，可安全删除 |
| `idmap.db`（旧） | — | 旧版单库 | 迁移源，迁移完成后可手动删除 |

## 核心逻辑

### 存储引擎：bbolt（嵌入式 KV）

- 每个 `db.Update()` 是一个**原子事务**，要么全部写入，要么全部回滚
- 写入使用 Write-Ahead Log（WAL），断电后自动恢复
- 读取使用 MVCC 快照，读不会阻塞写，写不会阻塞读
- 三个 DB 各有独立的文件锁，互不影响

### 写入路径

#### 迁移完成前（双写兼容期）

```go
StoreIDv2("OpenID") → 123
  ├── 旧库 ids 桶写入 ✅  ← 双写，两边都写
  └── 新库 identity DB 写入 ✅

StoreCachev2("msgID") → 456
  ├── 旧库 cache 桶写入 ✅
  └── 新库 msg DB 写入 ✅

// 全新函数（仅写新库，双写旧库）
StoreGroupID("GroupOpenID") → 123
StoreUserID("UserOpenID")   → 456
StoreMsgID("realMsgID")     → 789
```

#### 迁移完成后（纯新库）

```go
StoreIDv2("OpenID")
  └── storeIdentity() → 仅写 identity DB ✅  ← 旧库写入已跳过

StoreCachev2("msgID")
  └── StoreMsgID() → 仅写 msg DB ✅

// 运行时配置 / 用户信息（路由至新库）
WriteConfig("section", "key", "value") → identity DB ✅
StoreUserInfo("rawID", userInfo)       → identity DB ✅
```

> **关键保证**：同一条映射**一旦创建就不会修改**。同一个 OpenID 永远对应同一个虚拟 ID。

### 读取路径

```go
读取 "123" 对应的 OpenID
  ├── 新 identity DB → 有则直接返回（微秒级）
  └── 新库没有 → 回退旧 idmap.db → 查到时写回新库（惰性迁移）
```

## 后台静默迁移

### 迁移流程（4 阶段）

```
启动 Gensokyo
  ├── 第一步：同步计数器（阻塞）
  │     ├── 旧库 ids 桶 counter → identity DB
  │     └── 旧库 cache 桶 counter → msg DB
  │     → 返回后新库 counter ≥ 旧库，storeIdentity 不会碰撞
  │
  ├── 第二步：后台迁移（4 个桶，goroutine 异步）
  │     ├── 第一阶段：ids    → identity DB
  │     ├── 第二阶段：cache  → msg DB
  │     ├── 第三阶段：config → identity DB
  │     └── 第四阶段：UserInfo → identity DB
  │
  ├── 第三步：数据完整性校验
  │     ├── 游标流式对比（无 OOM）
  │     ├── 逐条检查 key 存在 + value 一致
  │     └── 所有 4 个桶全部校验
  │
  └── 第四步：收尾
        ├── 校验通过 → 写入 _migration_complete_v1 标记
        │              → 日志提示安全删除旧库
        └── 校验失败 → 自动修复（旧库覆盖）→ 重校验
```

### 计数器同步（防碰撞关键）

迁移开始前有**阻塞计数器同步**步骤：

```go
func StartMigration() {
    initNewDBs()
    if hasOldDB() {
        syncMigrationCounters()  // ← 阻塞，同步完成才返回
        go backgroundMigration()
    }
}
```

为什么需要：

```
❌ 无计数器同步：
  storeIdentity(OpenID_A) → 新库 counter = nil → 分配 ID=1
  迁移批次 1              → 写入 row-1 = OpenID_B（来自旧库）
  → row-1 冲突！OpenID_B 的反向映射丢失！

✅ 有计数器同步：
  同步：新库 counter = 500（来自旧库）
  storeIdentity(OpenID_A) → 新库 counter = 500 → 分配 ID=501（安全）
  迁移批次 1              → 写入 row-1...row-100（来自旧库）
  → 无冲突！501 不在 1-100 范围内
```

### 断电安全

| 断电时间点 | 影响 | 恢复 |
|-----------|------|------|
| 计数器同步完成，迁移协程未启动 | 无影响，新库已有正确 counter | 重启后检测到未迁移，重新迁移 |
| 旧库 `StoreID` 成功，新库 `newDBStore` 未执行 | 新库缺一条映射 | 热路径回退读旧库，或惰性迁移补回 |
| 后台迁移批次写入中 | 该批次未提交（bbolt 原子性） | 重启后跳过已迁移条目，继续 |
| 校验完成，marker 写入中 | 标记未写入 | 重启后重新校验，通过后再次写标记 |
| config/UserInfo 迁移中 | config/UserInfo 路由返回旧库 | routing 函数检查 `isMigrationComplete()` |
| 新 DB 写入完成，旧 DB 未写入 | 不存在此场景（新 DB 写入在旧 DB 之后） | — |

### 关键设计原则

1. **映射不可变**：`OpenID → 123` 一旦建立，永远不变。
2. **热路径只读**：读取操作从不写入任何数据库。
3. **新库优先**：读先查新库，新库没有再查旧库。
4. **写入不覆盖**：`writeBatchToNewDB` 检查 key 是否存在，已存在的绝不覆盖。
5. **迁移不删旧库**：旧库完整保留，用户确认稳定后手动删除。
6. **计数器先同步**：迁移前同步 counter，防止并发写入与迁移条目 ID 碰撞。
7. **线程安全**：`sync.Once` 保护初始化，`atomic.CAS` 保护迁移启动。

### 并发安全性

- 迁移与热路径写入**同时运行**时，`writeBatchToNewDB` 跳过已存在的 key，互不干扰
- `StoreIDv2` 迁移完成后切换至 `storeIdentity`（纯新库），不再碰旧库
- `StoreCachev2` 同理切换至 `StoreMsgID`
- `WriteConfig`/`StoreUserInfo` 等通过 `configAndUserInfoDB()` 路由：迁移未完成走旧库，完成后走新库

## 故障恢复

| 故障 | 影响 | 恢复方式 |
|------|------|---------|
| `idmap-msg.db` 损坏/膨胀 | 消息 ID 回执数字可能重复 | 停止 → 删除文件 → 重启（自动重建） |
| `idmap-identity.db` 损坏 | 群/用户映射、配置、用户信息丢失 | 停止 → 删除文件 → 保留 `idmap.db` → 重启（后台迁移自动恢复）|
| 两个新库都坏了 | 全部映射丢失 | 停止 → 删除新库 → 保留 `idmap.db` → 重启（完整迁移）|
| 旧 `idmap.db` 丢失 | 迁移中断，新库中已有数据 | 不影响，已迁移的数据继续可用 |
| 迁移失败（校验不过） | 新库可能部分数据 | 日志提示手动修复命令，不删旧库 |

> **建议**：定期备份 `idmap-identity.db`，这是永久数据。

## 迁移完成后的旧库清理

迁移完成后日志会提示：

```
======== 迁移全部完成 ========
  ✓ idmap-identity.db/ids       ── 永久身份映射
  ✓ idmap-msg.db/cache          ── 消息 ID 缓存
  ✓ idmap-identity.db/config    ── 运行时配置
  ✓ idmap-identity.db/UserInfo  ── 用户信息缓存

  ◉ idmap.db     ── 旧库（所有数据已迁出，可安全删除）

旧库安全删除方法: 停止 Gensokyo → 删除 idmap.db → 重启
```

## 参考

- 实现文件：`idmap/service.go`、`idmap/new_service.go`
- 旧 API → 新 API 映射：

| 旧函数 | 迁移完成后切换至 | 用途 |
|--------|-----------------|------|
| `StoreIDv2` | `storeIdentity` | 身份映射写入 |
| `StoreCachev2` | `StoreMsgID` | 消息 ID 缓存写入 |
| `RetrieveRowByIDv2` | `newDBLookup` → 旧库回退 | 身份映射读取 |
| `RetrieveRowByCachev2` | `newDBMsgLookup` → 旧库回退 | 消息 ID 查询 |
| `WriteConfig` / `ReadConfig` | `configAndUserInfoDB()` 路由 | 运行时配置 |
| `StoreUserInfo` / `ListAllUsers` | `configAndUserInfoDB()` 路由 | 用户信息 |


