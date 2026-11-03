# gbase8s-dolphin-go

面向 **GBase 8s openGauss/dolphin MySQL 协议端口**的 Go 连接示例与兼容层。

这个项目解决连接、协议和驱动层问题，不包含任何业务模型、业务 SQL、迁移文件或部署配置。经验来自 GBase 8s V8.8.5 S5.0.0B29 的对接实践；其他版本应先运行冒烟测试，不要假定存在完全相同的问题。

> Informix 底座的 GBase 8s 不适用本项目，它需要对应的原生驱动。

## 能做什么

- 使用 `SELECT 1` 探活，避开不支持 `COM_PING` 的兼容端口。
- 默认启用客户端参数内插，避开不支持服务端预处理语句的问题。
- 字符串参数使用标准 SQL 的 `''` 转义，保留引号、反斜杠和中文。
- 二进制参数使用 `decode(..., 'hex')` 写入，并解码 BLOB 结果中的 `\x` 十六进制形式。
- 当单条 `INSERT` 未返回自增 ID 时，在同一连接上补查 `LAST_INSERT_ID()`。
- 提供只读或包含写入的冒烟测试，验证实际服务端兼容性。

NULL 结果集丢字节是已观察到的**服务端协议缺陷**，不能靠本项目客户端代码修复。详见[兼容性问题与排查](docs/pitfalls.md)。

## 环境

- Go 1.24 或更高版本。
- 已启用 dolphin MySQL 协议的 GBase 8s 实例。
- B 兼容数据库，以及可用于 MySQL 协议认证的账号。
- 默认端口为 `3308`，以实际实例配置为准。

## 连接

```go
package main

import (
    "context"
    "log"
    "time"

    gbase "github.com/WormW/gbase8s-dolphin-go"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    db, err := gbase.Open(ctx, gbase.Config{
        Host:     "127.0.0.1",
        Port:     3308,
        Database: "app", // dolphin 连接通常映射到目标 schema
        Username: "app_user",
        Password: "replace-me",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 使用 database/sql 执行查询。
}
```

`Open` 会设置连接池并立即执行一次 `SELECT 1`。所有超时均可通过 `Config` 调整。不要把密码写入源码、日志或仓库。

如果上层使用 GORM，应把返回的 `*sql.DB` 作为现有连接传入，并设置 `DisableAutomaticPing: true`，避免 GORM 再发送 `COM_PING`。

## 冒烟测试

在命令行中设置环境变量，示例值不含任何真实凭据：

```bash
export GBASE_HOST=127.0.0.1
export GBASE_PORT=3308
export GBASE_DATABASE=app
export GBASE_USER=app_user
export GBASE_PASSWORD='replace-me'
```

默认只执行只读检查：连接、版本、NULL 列和字符串参数。

```bash
go run ./cmd/smoke
```

再显式启用写入检查：

```bash
go run ./cmd/smoke -write
```

写入检查会创建一个独立的临时表，插入并回读字符串、二进制、NULL 和自增 ID，最后删除该表。请只对允许写入的测试库使用。

## 本地验证

```bash
go test ./...
```

单元测试不需要数据库。真实兼容性必须通过上述冒烟测试在目标实例上确认。

## 实现与许可

驱动目录基于 [go-sql-driver/mysql v1.9.3](https://github.com/go-sql-driver/mysql/tree/v1.9.3)，保留了原始许可证和说明。项目采用 Mozilla Public License 2.0。

## 范围限制

- 不提供服务端二进制补丁；正式环境应优先使用厂商修复版本。
- 不覆盖 MySQL/InnoDB 到 dolphin 的完整 SQL 方言迁移。
- 不保证其他 GBase 版本或兼容模式具有相同行为。
- 写入路径依赖 dolphin 对 `decode(hex)`、`AUTO_INCREMENT` 和 `LAST_INSERT_ID()` 的支持，应在目标版本上验证。
