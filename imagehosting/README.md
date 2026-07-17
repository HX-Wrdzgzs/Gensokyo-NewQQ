# 统一图床服务

提供 7 种图床后端，按配置顺序依次尝试，第一个成功的返回结果。

## 配置 (config.yml)

```yaml
# 统一图床服务 — 按顺序依次尝试，第一个成功的返回 URL
# 免费图床（ChatGLM / Ukaka / 星野 / Nature）只需 enabled: true 即可使用
image_hosting:
  cos:
    enabled: false
    secret_id: ""           # 腾讯云 API SecretId
    secret_key: ""          # 腾讯云 API SecretKey
    region: "ap-guangzhou"  # 存储桶地域
    bucket: ""              # 存储桶名称
    domain: ""              # 自定义域名（留空使用 COS 默认域名）
  bilibili:
    enabled: false
    csrf_token: ""          # B站 Cookie 中的 bili_jct
    sessdata: ""            # B站 Cookie 中的 SESSDATA
    bucket: "openplatform"  # 上传 bucket
  qq_channel:
    enabled: false
    channel_id: ""          # 用于上传图片的子频道 ID
    token: ""               # Authorization 值，如 "QQBot xxx.yyy"
  chatglm:
    enabled: true           # 智谱免费图床
  ukaka:
    enabled: true           # Ukaka 免费图床
  xingye:
    enabled: true           # 星野免费图床
  nature:
    enabled: true           # Nature 免费图床（腾讯 COS 直传，密钥内置）
```

## 图床优先级

| 优先级 | 图床 | 费用 | 是否需要配置 |
|--------|------|------|-------------|
| 1 | COS (腾讯云) | 按量付费 | 需要 SecretId/SecretKey |
| 2 | Bilibili | 免费 | 需要 Cookie |
| 3 | QQ频道 | 免费 | 需要 channel_id + token |
| 4 | ChatGLM (智谱) | 免费 | 仅需 `enabled: true` |
| 5 | Ukaka | 免费 | 仅需 `enabled: true` |
| 6 | 星野 | 免费 | 仅需 `enabled: true` |
| 7 | Nature (腾讯COS) | 免费 | 仅需 `enabled: true` |

## 集成点

- `images/upload_api.go` 中的 `UploadBase64ImageToServer` 优先尝试图床链，失败后回退传统模式
- `handlers/message_parser.go` 中的 `ResolveMarkdownImages` 受益于图床链获取公开 URL

## 代码结构

```
imagehosting/
├── hosting.go       # 统一接口 + 调度器 + 辅助函数
├── cos.go           # 腾讯云 COS (HMAC 自签)
├── bilibili.go      # B站图床
├── qq_channel.go    # QQ频道图床
├── chatglm.go       # 智谱免费图床
├── signed.go        # Ukaka + 星野 (签名上传)
├── nature.go        # Nature 腾讯 COS 直传 (密钥内置)
├── utils.go         # 辅助函数
└── README.md        # 本文档
```
