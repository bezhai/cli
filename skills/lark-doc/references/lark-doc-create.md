# docs +create（创建飞书云文档）

> **前置条件（MUST READ）：** 生成文档内容前，必须先用 Read 工具读取以下文件，缺一不可：
> 1. [`lark-doc-xml.md`](lark-doc-xml.md) — XML 语法规则（使用 Markdown 格式时改读 [`lark-doc-md.md`](lark-doc-md.md)）
> 2. [`lark-doc-style.md`](style/lark-doc-style.md) — 排版指南（元素选择、丰富度规则、颜色语义）
> 3. [`lark-doc-create-workflow.md`](style/lark-doc-create-workflow.md) — 从零创作工作流（Code-Act Loop、并行执行策略）
>
> **未读完以上文件就生成内容会导致格式错误。**

从 XML（默认）或 Markdown 内容创建一个新的飞书云文档。

> **⚠️ 格式选择规则：** 创建 / 导入场景下 XML 和 Markdown 都可以——用户提供 `.md` 本地文件、或明确说"导入 Markdown"时，直接用 Markdown；没有明确指示时默认 XML（表达能力更强，支持 callout、grid、checkbox 等富 block 类型）。不要在用户没要求的情况下主动从 XML 切到 Markdown，也不要在用户已给出 Markdown 时强行改成 XML。

## 命令

```bash
# 创建 XML 文档（默认格式，推荐）
lark-cli docs +create --content '<title>项目计划</title><h1>目标</h1><p>记录本周重点。</p>'

# 仅当用户明确要求导入 Markdown 时才使用；文档标题用 --title，正文标题按内容自然组织
lark-cli docs +create --doc-format markdown --title "项目计划" --content $'## 目标\n\n- 明确重点\n- 记录待办'

# 创建到指定文件夹（XML）
lark-cli docs +create --api-version v2 --parent-token fldcnXXXX --content '<title>标题</title><p>首段内容</p>'

# 创建到个人知识库（XML）
lark-cli docs +create --api-version v2 --parent-position my_library --content '<title>标题</title><p>内容</p>'

# 创建含 HTML5 block 的文档：HTML 放本地文件，XML 只保留结构
lark-cli docs +create --api-version v2 \
  --content '<title>Demo</title><html5-block path="@widget.html"></html5-block>'

# 从 docs +fetch 输出回灌创建新文档
lark-cli docs +create --api-version v2 --input @fetch.json
```

## 返回值

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "document": {
      "document_id": "docx_token",
      "revision_id": 1,
      "url": "https://xxx.feishu.cn/docx/docx_token",
      "new_blocks": [
        { "block_id": "blkcnXXXX", "block_type": "whiteboard", "block_token": "boardXXXX" },
        { "block_id": "blkcnHTML", "block_type": "html5-block", "block_token": "blk_token" }
      ]
    }
  }
}
```

- **`document.new_blocks`**：本次操作新增的 block 列表（如画板、html5-block）。`block_id` 可用于 `docs +update` 的 `--block-id` 做精确编辑；`block_token` 是资源块 token（如画板 token 或 html5-block block token）。如果后续要删除、替换刚创建的 html5-block，使用这里返回的新 `block_id`，不要复用输入 XML 中的旧 id。

> \[!IMPORTANT]
> 如果文档是**以应用身份（bot）创建**的，如 `lark-cli docs +create --as bot` 在文档创建成功后，CLI 会**尝试为当前 CLI 用户自动授予该文档的 `full_access`（可管理权限）**。
>
> 以应用身份创建时，结果里会额外返回 `permission_grant` 字段，明确说明授权结果：
> - `status = granted`：当前 CLI 用户已获得该文档的可管理权限
> - `status = skipped`：本地没有可用的当前用户 `open_id`，因此不会自动授权；可提示用户先完成 `lark-cli auth login`，再让 AI / agent 继续使用应用身份（bot）授予当前用户权限
> - `status = failed`：文档已创建成功，但自动授权用户失败；会带上失败原因，并提示稍后重试或继续使用 bot 身份处理该文档
>
> `permission_grant.perm = full_access` 表示该资源已授予”可管理权限”。
>
> **不要擅自执行 owner 转移。** 如果用户需要把 owner 转给自己，必须单独确认。

## 参数

| 参数                  | 必填 | 说明                                          |
| ------------------- | -- |---------------------------------------------|
| `--api-version`     | 是  | 固定传 `v2`                                    |
| `--title`           | 否  | 文档标题；CLI 会把它前置为 `<title>...</title>`，不传 `--content` / `--input` 时可只用 `--title` 创建标题文档 |
| `--content`         | 视情况 | 文档内容（XML 或 Markdown 格式）；与 `--input` 互斥 |
| `--input`           | 否  | 隐藏高级入口：读取 fetch JSON envelope / `data` / 裸 `document`，自动抽取 `document.content` 和 `document.reference_map`；与 `--content` / `--reference-map` 互斥 |
| `--reference-map`   | 否  | 公开高级入口：与 `--content` 搭配传结构化 `reference_map`，主要用于 `<html5-block data-ref="...">`；支持直接 JSON、`@reference-map.json` 或 `-` 从 stdin 读取 |
| `--doc-format`      | 否  | 内容格式：`xml`（默认，始终优先使用）\| `markdown`（仅用户明确要求时） |
| `--parent-token`    | 否  | 父文件夹或知识库节点 token（与 `--parent-position` 互斥）  |
| `--parent-position` | 否  | 父节点位置，如 `my_library`（与 `--parent-token` 互斥） |

## 最佳实践

- **较长文档**：参考 [`lark-doc-create-workflow.md`](style/lark-doc-create-workflow.md) 先建骨架再分段写入；短文档可一次写完整内容
- **表达形式**：由用户目标和内容决定。需要结构化表达时可参考 [`lark-doc-style.md`](style/lark-doc-style.md)，但不要默认套用固定开头、固定富 block 比例或固定图表
- **HTML5 block**：写入时使用 `<html5-block path="@relative.html"></html5-block>`，不要把 HTML 内联进标签。fetch 结果可直接 `--input @fetch.json` 回灌；如果 fetch 结果的 `reference_map` 使用 `path`，执行命令前必须保留对应 `doc-fetch-resources/...html` 文件。已有 `data-ref` 正文时，可以用 `--reference-map @reference-map.json` 搭配 `--content` 显式传引用映射。

## 参考

- [`lark-doc-create-workflow.md`](style/lark-doc-create-workflow.md) — 从零创作工作流（Code-Act Loop、并行执行策略）
- [`lark-doc-style.md`](style/lark-doc-style.md) — 文档样式指南（元素选择 + 丰富度规则 + 颜色语义）
- [`lark-doc-xml.md`](lark-doc-xml.md) — XML 语法规范
- [`lark-doc-fetch.md`](lark-doc-fetch.md) — 获取文档
- [`lark-doc-update.md`](lark-doc-update.md) — 更新文档
- [`lark-doc-media-insert.md`](lark-doc-media-insert.md) — 插入图片/文件到文档
