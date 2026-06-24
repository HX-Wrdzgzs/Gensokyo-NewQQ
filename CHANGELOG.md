# Changelog

## Release 001

### 新增
- 支持非@群消息接收 (GroupMessageEventHandler)
- 自动转换按钮权限中的虚拟ID为QQ官方OpenID
- 静态编译，兼容旧版 GLIBC

### 修复
- ProcessGroupNormalMessage 中 @bot 未正确移除的问题
- RevertTransformedText BotID→AppID 替换顺序bug
- GroupMessageEventHandler 入口日志
- ParseAndHandle 添加错误日志与 panic 恢复

### 优化
- UPX 压缩改为 -7 平衡速度与体积
