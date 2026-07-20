# [CQ:at]（Markdown 专用）

## 说明

`[CQ:at,qq=<虚拟用户ID>]` **仅在 Markdown 消息中生效**，普通文本中不进行转换。

## 行为

在 Markdown 内容（`msg_type=2`）中写入 `[CQ:at,qq=<虚拟用户ID>]`，Gensokyo 会在发送前将其转换为 QQ 官方 @ 标签：

```text
<qqbot-at-user id="<真实OpenID>" />
```

此外，`messageText`（文本段）中的 `[CQ:at,qq=<虚拟用户ID>]` 也会被一并转换，并自动合并到 Markdown 内容头部。这意味着即使 `[CQ:at]` 写在 Markdown JSON 之外（如作为数组段中的 `{"type":"at","data":{"qq":"..."}}`），也能正确渲染为 @ 标签。

## 写法

```text
[CQ:at,qq=<虚拟用户ID>][CQ:markdown,data=base64://<base64-json>]
```

Markdown JSON 中也可以写：

```markdown
你好 [CQ:at,qq=123456]
```

## 入站方向

QQ 平台发送的 `<@OpenID>` 会被自动转换为标准的 `[CQ:at,qq=<虚拟ID>]` 格式，并建立 OpenID 与虚拟 ID 的映射。

## 限制

- `qq` 必须能通过 idmap 反查到 OpenID；失败时保留原 CQ 码。
- 普通文本中不支持 @ 渲染，请使用 Markdown 消息（`msg_type=2`）。
