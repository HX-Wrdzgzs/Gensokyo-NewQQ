# [CQ:remove] — 撤回指定消息

## 说明

出站 CQ 码，用于撤回指定用户的**指定消息**。`msg_id` 使用 OneBot V11 事件中传递的虚拟 `message_id`，无需插件自行维护 ID 映射。

## 格式

```
[CQ:remove,user_id=虚拟用户ID,msg_id=虚拟消息ID]
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:--:|------|
| `user_id` | int64 | ✅ | 目标用户的**虚拟 ID** |
| `msg_id` | int64 | ✅ | 要撤回的消息**虚拟 ID**（取自 OneBot 事件 `message_id`） |

## 流程

```
① 后端插件从 OneBot 事件获取 message_id
   event.message_id = 5678（虚拟 ID）

② 调用 send_group_msg 携带 CQ 码
   send_group_msg(group_id=821404315, message="[CQ:remove,user_id=3607918353,msg_id=5678]")

③ Gensokyo 解析 CQ 码
   - 剥离 [CQ:remove,...]，不发送到 QQ 频道
   - 虚拟 user_id → 真实 OpenID（idmap.RetrieveRowByIDv2）
   - 虚拟 msg_id → 真实 message_id（idmap.RetrieveMsgID）

④ Gensokyo 调用 QQ API 撤回
   api.RetractGroupMessage(groupOpenID, realMsgID)

⑤ 若 messageText 剥离后为空，跳过发送
```

## 限制

| 限制 | 说明 |
|------|------|
| 时效 | 只能撤回 **6 分钟内**的消息（msg 数据库 TTL） |
| 范围 | 仅群聊 |
| 权限 | 需机器人为群管理员 |

## 后端示例（nonebot2）

```python
from nonebot.adapters.onebot.v11 import Bot, GroupMessageEvent, Message

@on_command("撤回").handle()
async def recall_msg(bot: Bot, event: GroupMessageEvent):
    msg_id = event.message_id   # OneBot 事件自带的虚拟消息 ID
    user_id = event.user_id
    await bot.send_group_msg(
        group_id=event.group_id,
        message=Message(f"[CQ:remove,user_id={user_id},msg_id={msg_id}]")
    )
```

## 内部实现

| 模块 | 职责 |
|------|------|
| `idmap.RetrieveMsgID` | 虚拟 msg_id → 真实消息 ID 转换 |
| `idmap.RetrieveRowByIDv2` | 虚拟 user_id → 真实 OpenID 转换 |
| `handlers.ProcessCQRemoveOutbound` | 解析 `[CQ:remove,...]`，剥离 CQ 码，转换 ID |

## 适用范围

🏷️ 群聊（出站）
