# GBase 8s dolphin 连接兼容问题与排查

本文面向需要用 Go 连接 GBase 8s dolphin/MySQL 协议端口的开发者。读完应能选择正确的接入方式、建立连接，并用一组最小用例判断问题位于客户端还是服务端。

## 1. 先确认产品路线，不要只看“GBase 8s”名称

同样名为 GBase 8s，可能对应不同底座：

- **openGauss/dolphin 路线**：可以通过 dolphin 插件暴露的 MySQL 协议端口连接，使用 MySQL 驱动，但要处理本文中的协议差异。
- **Informix 路线**：需要相应的原生驱动与客户端库，不能把 dolphin 的 DSN 和兼容方案直接套过去。

开始改造前，至少确认服务端完整版本、兼容模式、监听端口和认证方式。端口不是身份依据，`3308` 只是本次实践中的 dolphin 配置。

## 2. 数据库名、schema 和认证不是 MySQL 原样复制

本次实践中，目标库使用 B 兼容模式，MySQL 连接里的“数据库名”映射到已创建的 schema。账号除常规建号和授权外，还需要设置 MySQL native password 认证。

准备逻辑如下，账号和名称按实际环境替换：

```sql
CREATE DATABASE appdb DBCOMPATIBILITY 'B';
-- 连接 appdb 后执行：
CREATE SCHEMA app;
CREATE USER app_user PASSWORD 'replace-me';
GRANT ALL ON SCHEMA app TO app_user;
SELECT set_native_password('app_user', 'replace-me');
```

这些语法和函数是否可用取决于目标版本。若认证失败，先区分网络不可达、普通数据库密码认证失败和 native password 未设置，不要只反复调整 DSN。

## 3. `Ping` 成功不是前提：兼容端口可能不支持 `COM_PING`

已观察到的服务端对 `COM_PING` 返回 `Error 2600 (HY000): dolphin server protocol not supported.`。因此以下做法会持续误报数据库不健康：

- 直接调用 `database/sql` 的 `Ping` 或 `PingContext`；
- ORM 建连后的自动 Ping；
- 复用默认 MySQL 健康检查的第三方组件。

通用替代方式是执行真实查询并消费结果：

```go
var value int
err := db.QueryRowContext(ctx, "SELECT 1").Scan(&value)
```

注意要修改所有探活入口，而不只是应用启动处。使用 GORM 时同时关闭自动 Ping，并将已经建立好的 `*sql.DB` 传给它。

## 4. 服务端预处理不可用时，启用客户端参数内插

同一兼容端口还被观察到不支持 `COM_STMT_PREPARE`。即使普通查询正常，带 `?` 参数的调用仍可能失败。

Go 驱动需要设置：

```go
config.InterpolateParams = true
```

这不是让调用方自己拼接 SQL。驱动仍负责按参数类型编码和转义，调用方继续使用占位符。除非确有独立需求，不要同时开启多语句执行；本项目默认不开启。

其他语言的对应思路也是关闭服务端预处理。例如 JDBC 通常需要 `useServerPrepStmts=false`，具体参数以所用驱动为准。

## 5. 参数内插后，含单引号的写入可能报错

MySQL 驱动默认按反斜杠规则把单引号转成 `\'`。本次 dolphin 环境按标准 SQL 解析字符串，`\'` 不能表达预期语义，可能造成语法错误或字符串提前结束。

通用修正是在内插路径中使用两个单引号：

```text
输入：it's
SQL 字符串字面量：'it''s'
```

`''` 对 MySQL 和标准 SQL 都合法。字符串与 JSON 字符串参数都应走这一规则；不要通过全局替换 SQL 文本实现，因为无法区分字符串内容、标识符和其他语法位置。

## 6. 二进制值不能沿用 `_binary'...'` 写入方式

在本次 dolphin 环境中，BLOB 对应 `BYTEA` 行为：

- 写入时，将 `[]byte` 编码为十六进制，并使用 `decode('<hex>', 'hex')`；
- 读取时，BLOB 列可能返回带 `\x` 前缀的十六进制文本，需要还原为原始字节；
- 只有 BLOB 类型列且完整匹配 `\x` + 偶数长度合法十六进制时才解码，其他文本保持原样；
- `nil` 字节切片仍应写入 SQL `NULL`，空字节切片则是空值，两者不能混淆。

不要对普通字符串做 `\x` 解码，否则合法的用户文本可能被悄悄改写。

## 7. `LastInsertId` 为 0 时，问题可能在 OK 包而非表定义

本次实践中，自增插入执行成功，但 dolphin 返回的 OK 包没有填充 MySQL 客户端需要的 insert ID，调用 `Result.LastInsertId()` 得到 `0`。

适配层只在以下条件同时满足时补查：

- 当前语句是单条 `INSERT`；
- 服务端返回的 insert ID 为 `0`；
- 补查 `SELECT LAST_INSERT_ID()` 在**同一物理连接**上执行。

最后一点不能忽略。换用连接池里的另一条连接查询，可能得到其他会话的值。多语句、服务端已返回非零 ID、非 INSERT 语句不应触发补查。

## 8. NULL 列错位或客户端 panic，应优先检查服务端协议实现

已在 GBase 8s V8.8.5 S5.0.0B29 的 dolphin 文本协议中观察到：NULL 列没有写入 MySQL 协议要求的 `0xfb` 标记，而是完全没有字节。

可能后果：

- NULL 位于行中间时，后续值左移，造成静默列错位；
- NULL 位于行尾时，行包提前结束，Go 客户端可能 panic；
- 整行均为 NULL 时，无法按预期解析结果。

最小只读验证：

```sql
SELECT 'a', NULL, 'b', NULL;
```

期望得到四列：`'a'`、`NULL`、`'b'`、`NULL`。还可以分别验证 `SELECT NULL`、`SELECT NULL, NULL, NULL`，以及同一查询经 PostgreSQL 协议端口的对照结果。

这是**服务端序列化问题**。客户端无法可靠推导缺失字节的位置，本项目也不包含二进制补丁。应向厂商确认修复版本；不要通过忽略 panic 或重排列名掩盖数据完整性风险。

## 9. 按层次定位，避免一次改多个变量

建议按以下顺序执行：

1. 使用 `SELECT 1` 验证网络、认证和最基本查询。
2. 查询 `version()`，记录实际服务端版本和兼容路线。
3. 执行 NULL 矩阵，确认结果集协议是否完整。
4. 用参数化 `SELECT ?` 往返单引号、反斜杠和中文。
5. 在专用测试库验证二进制、NULL 和自增 ID 写入回读。
6. 最后接入 ORM 或连接池，确认其自动建连、健康检查没有重新引入 `COM_PING` 或服务端预处理。

本项目默认冒烟测试只读；`-write` 模式才创建并删除独立测试表。任何真实环境验证都应先确认账号权限和写入边界。

## 10. 常见问题对照

| 现象 | 优先检查 | 通用处理 |
| --- | --- | --- |
| 建连或就绪检查返回 Error 2600 | 是否调用了 `Ping` | 改为 `SELECT 1`，关闭框架自动 Ping |
| 带参数语句报协议不支持 | 是否使用服务端预处理 | 启用驱动参数内插 |
| 含 `'` 的参数写入报语法错误 | 内插后是否使用 `\'` | 使用 `''` 标准 SQL 转义 |
| BLOB 回读出现 `\x...` 文本 | 列类型与字节解码 | 只对 BLOB 的合法十六进制形式解码 |
| 插入成功但 ID 为 0 | OK 包 insert ID | 在同一物理连接补查 `LAST_INSERT_ID()` |
| NULL 导致列错位或 panic | 服务端文本结果集编码 | 用 NULL 矩阵复现并寻求服务端修复 |

以上问题具有版本相关性。升级或更换 GBase 构建后，应重新运行完整冒烟测试，而不是把某个版本的规避方案永久硬编码为所有数据库的通用行为。
