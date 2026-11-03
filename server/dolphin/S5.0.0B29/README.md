# dolphin NULL 结果集临时二进制补丁

本目录包含针对 **GBase 8s V8.8.5 S5.0.0B29（openGauss 底座）**的 `dolphin.so` 临时修复，解决 MySQL 文本协议结果集丢失 NULL 列的问题。

> 高风险提示：二进制修改可能使该文件失去原厂支持状态。仅在版本和哈希完全匹配、已备份并获准停库维护时使用。正式环境应优先采用厂商发布的修复版本。

## 文件与适用范围

| 文件 | 用途 | SHA-256 |
| --- | --- | --- |
| `dolphin.so.patched` | 修复后的 x86-64 ELF 共享库 | `c61238fde0e8d7384becbdf693cae9b4ba9df3f117d1910563a6138080810ab0` |

仅适用于以下服务端构建：

```text
gaussdb (single_node GBase8sV8.8.5 S5.0.0B29 build 9f0c40cd) compiled at 2024-07-26 09:15:11 ...
```

替换前，原文件的预期 SHA-256 为：

```text
3fc562406434cb3b211569c6c34f4675486f9091ae84fa103921bc2960eccaac
```

构建号、CPU 架构或原文件哈希不匹配时，**不要替换**。本目录不包含未修改的原厂共享库；回滚使用替换前创建的备份。

## 修复内容

该构建的 dolphin 插件在生成 MySQL 文本结果集时，对 NULL 列没有写入任何字节。按 MySQL 协议，每个 NULL 列必须编码为一个 `0xfb` 字节。缺失该标记会导致后续列错位、行包提前结束，甚至使客户端越界崩溃。

补丁仅调整 NULL 分支，使其写入协议要求的 `0xfb` 标记。它不能修复其他 SQL 方言或协议兼容问题，也不替代本项目的连接层适配。

## 1. 核对版本和文件

在数据库主机执行：

```bash
gaussdb -V
```

找到实际安装目录并核对当前文件：

```bash
sha256sum <安装目录>/lib/postgresql/dolphin.so
sha256sum dolphin.so.patched
```

只有服务端构建完全一致，且当前文件哈希等于上述原始哈希时才继续。如果当前文件已经等于补丁哈希，说明文件已替换，可直接执行后面的结果集验证。

## 2. 停库并备份

在批准的维护窗口内停止目标实例：

```bash
systemctl stop <实例服务名>
```

备份原文件，记录生成的备份路径：

```bash
cp -p <安装目录>/lib/postgresql/dolphin.so \
  <安装目录>/lib/postgresql/dolphin.so.orig.$(date +%Y%m%d-%H%M%S)
```

不要省略备份，也不要从其他构建复制所谓“原版”文件用于回滚。

## 3. 替换并启动

保持数据库运行用户所需的属主和权限：

```bash
install -m 0755 -o <数据库用户> -g <数据库组> dolphin.so.patched \
  <安装目录>/lib/postgresql/dolphin.so
systemctl start <实例服务名>
```

确认数据库进程正常、应用所需端口已监听，并检查启动日志。dolphin 端口可能比主数据库端口稍晚就绪。

## 4. 验证 NULL 结果集

使用任意 MySQL 协议客户端连接 dolphin 端口。例如 Python/PyMySQL：

```python
import pymysql

conn = pymysql.connect(
    host="<数据库主机>",
    port=3308,
    user="<用户>",
    password="<密码>",
    database="<schema>",
)
cur = conn.cursor()
cur.execute("SELECT 'a', NULL, 'b', NULL")
print(cur.fetchall())
```

期望结果：

```text
(('a', None, 'b', None),)
```

同时建议验证：

```sql
SELECT NULL;
SELECT 'a', NULL, 'b';
SELECT NULL, NULL, NULL;
```

如果列数不正确、结果错位、客户端异常或服务端启动失败，立即回滚。验证成功后，再运行项目根目录的 Go 冒烟测试。

## 回滚

停止实例，用第 2 步生成的备份覆盖回原路径，保持原属主和权限，然后重新启动：

```bash
systemctl stop <实例服务名>
cp -p <备份文件> <安装目录>/lib/postgresql/dolphin.so
systemctl start <实例服务名>
```

回滚会恢复该版本的 NULL 协议缺陷，依赖修复的应用可能再次出现列错位或崩溃。

## 许可与责任边界

`dolphin.so` 是第三方数据库组件，本仓库的 MPL 2.0 许可证不代表对该组件重新授权。其使用、修改和分发仍受 GBase 原有许可与支持条款约束。

该文件是特定构建的临时修复，不是可从源码重建的通用补丁。不要将它用于其他版本，也不要把哈希匹配之外的“看起来相同”视为兼容依据。
