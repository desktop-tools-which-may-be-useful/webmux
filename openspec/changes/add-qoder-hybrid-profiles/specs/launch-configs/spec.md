# launch-configs Delta Spec

## New Requirements

### Requirement: Qoder 与 QoderCN 预置配置

新安装的 WebMux SHALL 在 `launch-configs.json` 中预置以下两个启动配置：

| 名称 | 图标 | 命令 | 混合 |
|------|------|------|------|
| Qoder | `qoder` | `qoder --dangerously-skip-permissions`（bashrc 包装） | `true` |
| QoderCN | `qoder` | `qodercn --dangerously-skip-permissions`（bashrc 包装） | `true` |

已有 `launch-configs.json` 的安装 SHALL NOT 被改写。

### Requirement: Qoder 图标

`normalize()` 图标白名单 SHALL 新增 `"qoder"`。前端图标下拉、自动检测与 Svelte 组件 SHALL 同步注册。

#### Scenario: 新安装预置

- **WHEN** WebMux 首次安装且 PATH 中存在 `qoder` 与 `qodercn`
- **THEN** 启动配置列表包含 "Qoder" 与 "QoderCN"，均标记为混合模式

#### Scenario: 图标自动检测

- **WHEN** 配置的 argv 首个可执行文件为 `qoder` 或 `qodercn`
- **THEN** 图标自动检测为 `qoder`
