# 需求文档：Instances 列表搜索功能

## 1. 概述

在首页 Instances 列表区域的标题和列表项之间，新增一个始终可见的搜索框，支持对实例的模糊搜索过滤。

## 2. UI 设计

- **位置**：Instances 标题行正下方、第一个列表项上方，始终占据一行
- **宽度**：与列表等宽
- **占位符文字**：`/ Search...`（提示 `/` 快捷键）
- **高度**：1 行
- **聚焦样式**：圆角边框高亮（紫色 accent 色 `99`），光标闪烁
- **失焦样式**：圆角边框变暗（`3C3C3C`），placeholder 文字可见

## 3. 交互行为

| 操作 | 行为 |
|------|------|
| 按 `/` 键 | 聚焦搜索框，进入搜索态 |
| 输入文字 | 同步过滤实例列表（无需防抖，纯内存操作） |
| 按 `Esc` | 退出搜索框聚焦，**保留搜索词和过滤结果**，焦点回到列表 |
| 按 `Enter` | 退出搜索框聚焦，**保留搜索词和过滤结果**，焦点回到列表 |
| 按 `Ctrl+C` | 清空搜索词、取消过滤、退出搜索态 |
| 搜索态中按 `↑/↓` 方向键 | 移动列表选中项 |
| 搜索态中输入文字 | 转发给搜索框，实时过滤 |

## 4. 搜索逻辑

- **匹配字段**：Title、Description、Branch
- **匹配方式**：大小写不敏感的子字符串模糊匹配
- **空搜索**：搜索框为空时显示全部实例（`filteredItems == nil` 表示未过滤状态）
- **无匹配**：列表区域显示 "No matching instances" 提示文字
- **高亮匹配**：匹配的子串使用粗体+下划线+紫色前景色（`rgb(91,74,138)`）高亮，而非背景色变更
- **选中状态**：
  - 搜索过滤导致当前选中实例被过滤掉时，自动选中第一个匹配项（`selectedIdx = 0`）
  - 当前选中实例仍在过滤结果中时，`selectedIdx` 更新为该实例在过滤结果中的新位置

## 5. 状态持久化

- 应用重启后搜索框始终为空，不跨会话保留搜索词

## 6. 技术实现要点

### 6.1 架构

- 应用状态机新增 `stateSearch` 状态，位于 `app/app.go`
- 搜索框使用 Bubble Tea 的 `textinput` 组件，集成在 `ui/list.go` 的 `List` 结构体中
- `List` 结构体新增字段：`searchInput textinput.Model`、`searchFocused bool`、`filteredItems []*session.Instance`
- `InstanceRenderer` 新增 `searchQuery string` 字段，用于渲染时高亮匹配子串

### 6.2 过滤逻辑

- `filteredItems == nil` 表示"未过滤，显示全部"；`len(filteredItems) == 0` 表示"无匹配"
- `visibleItems()` 方法统一返回当前可见列表（过滤后或全部），所有遍历操作均通过此方法访问
- `updateFilteredItems()` 在搜索词变化时重新计算过滤结果，同时：
  - 保存当前选中实例引用
  - 过滤后查找该实例在 `filteredItems` 中的新索引，更新 `selectedIdx`
  - 若当前选中实例不在过滤结果中，`selectedIdx` 重置为 0

### 6.3 高亮实现

- `HighlightMatch(text, query, restoreFg)` 函数在纯文本上标记匹配子串
- 高亮使用直接 ANSI 序列而非 lipgloss Style，避免 `\x1b[0m` 重置所有样式
- 高亮序列：`\x1b[1;4;38;2;91;74;138m`（粗体+下划线+紫色前景色）
- 高亮恢复：`\x1b[22;24;39m`（清除粗体、下划线、前景色）+ `restoreFg`（恢复外层前景色）
- `restoreFg` 参数：
  - 选中态：`\x1b[38;2;26;26;26m`（深色前景色，与 `selectedTitleStyle` 一致）
  - 非选中态：`\x1b[39m`（默认前景色）
- 使用前景色高亮而非背景色变更，规避终端对双宽度字符（如中文）背景色渲染的固有限制

### 6.4 键盘事件处理

- `handleSearchState()` 处理搜索态下的所有键盘事件
- `Ctrl+C`：清空搜索、取消过滤、回到默认态
- `Esc`/`Enter`：退出搜索框聚焦，保留搜索词和过滤结果
- `↑/↓`：在搜索态中移动列表选中项
- 其他按键：转发给 textinput 组件

### 6.5 搜索框宽度

- 搜索框宽度跟随列表宽度变化，在 `SetSize()` 中更新：`searchInput.Width = AdjustPreviewWidth(width) - 4`
