# 2026-11-27 上游合并验证记录

## 范围

- 本地基线：`2ced934c7`（v0.2.37 及渠道监控 Responses Skill 隔离回归检查）。
- 上游：`Wei-Shaw/sub2api` `4233cc3d8018bf2f399c4c366d94067c42eb697e`，本次合入 140 个提交。
- 合并方式：保留本地 `backend/cmd/server/VERSION` 的 `0.2.37`，并在错误透传运行时同时保留本地错误映射与上游详细规则返回逻辑。

## 本地功能核对

已核对全局、分组和模型 Skills，渠道监控 probe 跳过 Skills，错误映射，兑换码错误提示，邀请返利提现，模型白名单/映射，订阅单位，思考等级计费，会话管理和个人更新来源仍在合并结果中。上游新增的 Responses/Anthropic 转换、透传错误保留、粘性会话 CAS、审核队列按需分配、响应模型展示和更新 checksum 校验也保留。

## 验证结果

- 后端：`go test -tags unit ./... -count=1 -p 4 -timeout 180s` 通过。Windows 备份测试另外把 Git `usr/bin` 加入 PATH 后通过；这是测试进程缺少 `sh` 的环境差异，不是运行时代码修复。
- 后端嵌入构建：`go build -tags embed ./cmd/server` 通过。
- 前端：`vue-tsc --noEmit` 和 `pnpm run build` 通过。生产构建提示已有的大 chunk warning，构建本身成功。
- 本地功能定向前端回归：渠道响应模型展示、返利提现、兑换码、模型白名单共 53 项通过；模型广场和订阅进度测试在补充受上游订阅单位逻辑影响的 Pinia/mock 后 44 项通过。
- 前端全量 Vitest 复跑：389 个测试文件通过，9 个文件中的 48 项失败。失败集中在旧测试夹具未提供 Pinia、订阅单位和上游新增设置字段；未发现来自本地 Skills、渠道监控、错误映射或返利提现定向回归的失败。
- 数据库/Redis 集成测试未执行：本机 Docker Desktop Linux daemon 在本次验证期间不可用，连接 `dockerDesktopLinuxEngine` 失败。

## 性能对比

合并前后使用同一 Windows amd64、Go 1.27.4、`-cpu=1 -count=3 -benchtime=200ms`、`GOMEMLIMIT=2GiB` 参数执行基准。平均值变化如下，微基准不能替代线上压测：

| 场景 | 合并前 | 合并后 | 变化 |
| --- | ---: | ---: | ---: |
| Responses 128KiB 扫描校验 | 28.8 µs | 28.6 µs | -0.7% |
| Responses 1MiB 扫描校验 | 221.3 µs | 227.5 µs | +2.8% |
| Anthropic 1MiB 扫描校验 | 206.9 µs | 210.0 µs | +1.5% |
| 大请求透传图片意图复用（一次） | 16.46 ms | 16.55 ms | +0.6% |
| 大请求透传图片意图复用（两次） | 32.90 ms | 33.00 ms | +0.3% |
| WebSocket 入站 payload 解析 | 2.97 µs | 3.03 µs | +2.1% |
| `store=false` 69MiB 改写 | 94.38 ms | 95.04 ms | +0.7% |

这些差异在本机重复测量范围内，没有观察到明显回退；仍需在部署环境结合真实流量、数据库和 Redis 进行验证。

## 未执行事项

本次仅完成合并和本地验证，未推送、打标签或发布新版本；线上 endpoint、真实上游渠道延迟和 Docker 集成环境仍未验证。
