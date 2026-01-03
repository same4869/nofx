# todo/ 目录规则（计划与进度维护）

`todo/` 是“执行层”资料，用于规划与验收；`WIKI/` 是“知识层”资料，用于长期沉淀。

## 目录结构约定

- `todo/ROADMAP.md`：里程碑总览（M0~M4），只描述目标与阶段依赖  
- `todo/PROGRESS.md`：唯一“打勾”进度表（以任务为单位更新）  
- `todo/README.md`：如何使用 todo 的说明  
- `todo/phases/phase-XX-*.md`：每个阶段的：
  - 你需要确认的点（参数/边界）
  - 详细任务拆分（PXX-1...）
  - 验收方式（Verification）
  - 当前实现补充（What’s landed）

## 更新规则

1) 每落地一个子任务：
   - 在 `todo/PROGRESS.md` 勾选并写日期/备注
   - 同步在对应 `todo/phases/phase-*.md` 补充 “What’s landed”
2) `todo/` 不放长期设计与大段背景知识：
   - 长期知识写进 `WIKI/`
   - todo 只保留“下一步要做什么/怎么验收”

## 命名与风格

- 任务编号保持稳定（P01-1/P02-3…），避免重排导致历史难追踪  
- 验收步骤尽量可复制（命令、UI 操作步骤、预期现象）  

