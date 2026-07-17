# 统一图床服务

统一图床模块按配置顺序尝试可用后端，并返回第一个成功的公开图片 URL。

当前保留 6 种后端：腾讯云 COS、Bilibili、QQ 频道、ChatGLM、Ukaka 和星野。上游 Nature 后端因在公开源码中包含内置对象存储凭据，已在本分支禁用。

## 安全默认值

建议所有后端默认保持关闭，仅启用自己明确了解并接受其数据处理方式的服务。图片可能包含聊天截图、头像或其他敏感内容，启用第三方图床意味着图片会被上传到对应服务。

ChatGLM、Ukaka 和星野属于无需用户凭据的第三方上传服务。除配置中的 `enabled: true` 外，还必须设置以下环境变量才会启用：

```text
GENSOKYO_ENABLE_THIRD_PARTY_IMAGE_HOSTS=1
```

旧配置即使仍将这些后端写为 `enabled: true`，未设置环境变量时也不会上传。

## 配置示例

```yaml
image_hosting:
  cos:
    enabled: false
    secret_id: ""
    secret_key: ""
    region: "ap-guangzhou"
    bucket: ""
    domain: ""
  bilibili:
    enabled: false
    csrf_token: ""
    sessdata: ""
    bucket: "openplatform"
  qq_channel:
    enabled: false
    channel_id: ""
    token: ""
  chatglm:
    enabled: false
  ukaka:
    enabled: false
  xingye:
    enabled: false
  nature:
    enabled: false
```

`nature.enabled` 仅为兼容旧配置保留；该后端不会再执行上传。

## 输入限制

- 单张图片最大 10 MiB。
- 仅接受 JPEG、PNG、GIF 和 WebP。
- JPEG、PNG 和 GIF 会验证图片结构及尺寸。
- 最大像素数量为 4000 万。
- 文件名会移除路径和控制字符，并按实际图片格式修正扩展名。
- 图床 HTTP 请求统一设置 15 秒超时。
- 第三方响应体最多读取 1 MiB。

## 后端优先级

| 优先级 | 图床 | 启用条件 |
|---|---|---|
| 1 | 腾讯云 COS | 配置 SecretId、SecretKey、地域和存储桶 |
| 2 | Bilibili | 配置 Cookie 凭据 |
| 3 | QQ 频道 | 配置频道 ID 和 Authorization |
| 4 | ChatGLM | `enabled: true` 并设置显式授权环境变量 |
| 5 | Ukaka | `enabled: true` 并设置显式授权环境变量 |
| 6 | 星野 | `enabled: true` 并设置显式授权环境变量 |

## 集成点

- `images/upload_api.go` 中的 `UploadBase64ImageToServer` 优先尝试图床链，失败后回退传统模式。
- `handlers/message_parser.go` 中的 `ResolveMarkdownImages` 可通过图床链获取公开 URL。

## 代码结构

```text
imagehosting/
├── hosting.go       # 统一调度、输入校验和 HTTP 辅助函数
├── cos.go           # 腾讯云 COS
├── bilibili.go      # Bilibili
├── qq_channel.go    # QQ 频道
├── chatglm.go       # ChatGLM
├── signed.go        # Ukaka 和星野
├── nature.go        # 已禁用的兼容占位实现
├── utils.go         # JSON 等辅助函数
└── hosting_test.go  # 离线单元测试
```
