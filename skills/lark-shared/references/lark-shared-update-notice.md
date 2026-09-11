# 更新与 `_notice`

lark-cli 命令执行后，如果检测到新版本，JSON 输出中会包含 `_notice.update` 字段（含 `message`、`command` 等）。

除非用户正在询问更新、版本或 notice，否则不要把 `_notice` 原样复制为当前任务的主要答案，也不要为了 notice 中断当前任务去反复查 help。

需要稳定 JSON 给脚本或机器读取时，可以在命令前设置：

```bash
LARKSUITE_CLI_NO_UPDATE_NOTIFIER=1 LARKSUITE_CLI_NO_SKILLS_NOTIFIER=1 <lark-cli command>
```

当你在输出中看到 `_notice.update` 时，先完成用户当前请求；如仍相关，再简短告知可运行：

```bash
lark-cli update
```

本 fork 的 `lark-cli update` 默认只处理 CLI，不安装或同步 AI Skills。用户明确需要同步官方 Skills 时，使用 `lark-cli update --with-skills`；首次同步会安装完整官方列表。`--force` 本身不启用 Skills 同步。

本 fork 目前通过源码分发；保留 fork 修改时，应拉取自己的分支并执行 `make build`。现有 npm 包和二进制下载链接仍属于上游。

另外两类 notice：
- `_notice.skills`：本地 Skills 与当前 CLI 不同步；只有用户需要同步时，才执行 `command` 中的显式同步命令。
- `_notice.deprecated_command`：本次使用了兼容保留的旧命令；后续调用改用 `replacement`。如果同时提供 `action: "lark-cli update --with-skills"`，同样建议升级。
