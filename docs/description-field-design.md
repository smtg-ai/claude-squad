# Description 字段功能设计文档

## 概述

为 claude-squad 的 Instance 新增可选的 Description 字段，允许用户在创建实例时填写中文描述性文字，不影响 git 分支名等技术标识符。

## 需求背景

当前 Title 字段有 32 字符限制，且 Title 会成为 git 分支名的一部分（`{branchPrefix}{title}`），分支名不支持中文。需要一个独立的 Description 字段供用户填写备注说明，支持中文字符。

## 设计决策汇总

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | Description 用途 | 独立备注字段，不影响 git 分支名 |
| 2 | 输入阶段 | `n` 和 `N` 都能填写 |
| 3 | `n` 键流程 | Title → Enter → Description 逐字符输入（新状态 `stateDescription`） → Enter 跳过/确认 → 启动；Esc 取消创建 |
| 4 | `N` 键流程 | Title → Enter → Prompt Overlay 中 textarea 后新增 textinput 组件 |
| 5 | 列表显示 | 有 Description 时在 title 行和 branch 行之间新增 Description 行（`☰` 图标 + 暗色文字）；`stateDescription` 输入中即使 Description 为空也提前渲染 `☰` 行 + 闪烁光标 `|`；选中时 Description 行统一背景色，前景色用暗色区分 |
| 6 | 创建后编辑 | 不支持，和 Title 一致 |
| 7 | Overlay placeholder | `"Add a description..."` |
| 8 | 菜单提示 | `"submit description (optional)"` |
| 9 | 图标 | `☰` |
| 10 | 输入中视觉 | 在最终位置实时显示，`stateDescription` 时提前渲染 `☰` 行 + 闪烁竖线光标 `|` |
| 11 | 截断 | 硬截断 + 省略号 `…` |
| 12 | 字符限制 | 128 字符，单行 |
| 13 | 持久化 | `InstanceData` 新增 `Description string` 字段，向前兼容 |
| 14 | `n` 键流程 Description 输入视觉 | 进入 `stateDescription` 时提前渲染 `☰` 行（即使 Description 为空），末尾显示闪烁竖线光标 `|` |
| 15 | Description 行选中样式 | 选中时 Description 行统一背景色（与 branch 行一致 `#dde4f0`），前景色用暗色区分（如 `#888888`），避免输入前后视觉跳变 |

---

## 详细设计

### 1. 数据模型变更

#### `session/instance.go` — Instance 结构体

在 `Prompt` 字段后新增：

```go
// Description is an optional description for the instance (supports Unicode).
Description string
```

#### `session/instance.go` — InstanceOptions 结构体

新增：

```go
// Description is an optional description for the instance.
Description string
```

#### `session/instance.go` — NewInstance 函数

在 return 中新增 `Description: opts.Description`。

#### `session/instance.go` — SetDescription 方法

新增方法：

```go
// SetDescription sets the description of the instance.
func (i *Instance) SetDescription(desc string) {
    i.Description = desc
}
```

#### `session/instance.go` — ToInstanceData 方法

在 data 赋值中新增 `Description: i.Description`。

#### `session/instance.go` — FromInstanceData 函数

在 instance 初始化中新增 `Description: data.Description`。

#### `session/storage.go` — InstanceData 结构体

新增字段：

```go
Description string `json:"description"`
```

向前兼容：老版本 `state.json` 中无此字段，JSON 反序列化时默认为零值（空字符串），无需迁移。

---

### 2. 应用状态变更

#### `app/app.go` — 状态枚举

在 `stateNew` 和 `statePrompt` 之间新增：

```go
// stateDescription is the state when the user is entering a description for a new instance.
stateDescription
```

#### `app/app.go` — home 结构体

无需新增字段。Description 在 `stateDescription` 状态下直接写入 `instance.Description`。

---

### 3. `n` 键流程变更

#### 现有流程

```
按 n → stateNew → 逐字符输入 Title → Enter → 直接启动 instance
```

#### 新流程

```
按 n → stateNew → 逐字符输入 Title → Enter → stateDescription → 逐字符输入 Description → Enter → 启动 instance
```

#### `app/app.go` — stateNew 下 Enter 键处理

当前行为（约第 411-444 行）：Title 非空检查后，若 `!promptAfterName`，直接设置 Loading 状态并启动。

变更：Title 非空检查后，若 `!promptAfterName`，**不再直接启动**，而是进入 `stateDescription`：

```go
case tea.KeyEnter:
    if len(instance.Title) == 0 {
        return m, m.handleError(fmt.Errorf("title cannot be empty"))
    }

    if m.promptAfterName {
        // N 键流程：进入 Prompt Overlay
        m.promptAfterName = false
        m.state = statePrompt
        m.menu.SetState(ui.StatePrompt)
        m.textInputOverlay = m.newPromptOverlay()
        initialSearch := m.runBranchSearch("", m.textInputOverlay.BranchFilterVersion())
        return m, tea.Batch(tea.WindowSize(), initialSearch)
    }

    // n 键流程：进入 Description 输入
    m.state = stateDescription
    m.menu.SetState(ui.StateDescription)
    return m, tea.WindowSize()
```

#### `app/app.go` — 新增 stateDescription 处理块

在 `stateNew` 的 `else if` 链之后，新增 `stateDescription` 处理，逻辑与 `stateNew` 类似：

```go
if m.state == stateDescription {
    if msg.String() == "ctrl+c" {
        // 取消整个创建流程
        m.state = stateDefault
        m.promptAfterName = false
        m.list.Kill()
        return m, tea.Sequence(
            tea.WindowSize(),
            func() tea.Msg {
                m.menu.SetState(ui.StateDefault)
                return nil
            },
        )
    }

    instance := m.list.GetInstances()[m.list.NumInstances()-1]
    switch msg.Type {
    case tea.KeyEnter:
        // 确认 Description（留空即跳过），启动 instance
        instance.SetStatus(session.Loading)
        m.newInstanceFinalizer()
        m.promptAfterName = false
        m.state = stateDefault
        m.menu.SetState(ui.StateDefault)

        startCmd := func() tea.Msg {
            err := instance.Start(true)
            return instanceStartedMsg{
                instance:        instance,
                err:             err,
                promptAfterName: false,
            }
        }
        return m, tea.Batch(tea.WindowSize(), m.instanceChanged(), startCmd)

    case tea.KeyRunes:
        if runewidth.StringWidth(instance.Description) >= 128 {
            return m, m.handleError(fmt.Errorf("description cannot be longer than 128 characters"))
        }
        instance.SetDescription(instance.Description + string(msg.Runes))

    case tea.KeyBackspace:
        runes := []rune(instance.Description)
        if len(runes) == 0 {
            return m, nil
        }
        instance.SetDescription(string(runes[:len(runes)-1]))

    case tea.KeySpace:
        if runewidth.StringWidth(instance.Description) >= 128 {
            return m, m.handleError(fmt.Errorf("description cannot be longer than 128 characters"))
        }
        instance.SetDescription(instance.Description + " ")

    case tea.KeyEsc:
        // Esc 跳过 Description，直接启动
        instance.SetStatus(session.Loading)
        m.newInstanceFinalizer()
        m.promptAfterName = false
        m.state = stateDefault
        m.menu.SetState(ui.StateDefault)

        startCmd := func() tea.Msg {
            err := instance.Start(true)
            return instanceStartedMsg{
                instance:        instance,
                err:             err,
                promptAfterName: false,
            }
        }
        return m, tea.Batch(tea.WindowSize(), m.instanceChanged(), startCmd)
    }
    return m, nil
}
```

> **注意**：`stateDescription` 下按 Esc 的行为——取消整个创建流程（与 stateNew 下 Esc 行为一致）。用户如果想跳过 Description 直接启动，可以按 Enter（Description 为空时 Enter 同样会启动实例）。

---

### 4. `N` 键流程变更（Prompt Overlay）

#### `ui/overlay/textInput.go` — TextInputOverlay 结构体

新增字段：

```go
descriptionInput textinput.Model  // 使用 bubbles/textinput 组件
```

#### `ui/overlay/textInput.go` — NewTextInputOverlayWithBranchPicker

初始化 descriptionInput：

```go
di := textinput.New()
di.Placeholder = "Add a description..."
di.CharLimit = 128
di.Width = innerWidth  // 与其他组件宽度一致
```

#### `ui/overlay/textInput.go` — 焦点站点（numStops）计算

新增 1 个焦点站点：

```go
// 无 profile: textarea + description + branch picker + enter = 4
// 有 profile: profile + textarea + description + branch picker + enter = 5
numStops := 4
if pp != nil && pp.HasMultiple() {
    numStops = 5
}
```

#### `ui/overlay/textInput.go` — 焦点判断方法

新增 `isDescriptionInput` 方法，并更新其他方法的 FocusIndex 判断：

| 组件 | 无 Profile 时 FocusIndex | 有 Profile 时 FocusIndex |
|------|------------------------|------------------------|
| Profile Picker | - | 0 |
| Textarea | 0 | 1 |
| **Description Input** | **1** | **2** |
| Branch Picker | 2 | 3 |
| Enter Button | 3 | 4 |

```go
func (t *TextInputOverlay) isDescriptionInput() bool {
    if t.profilePicker != nil && t.profilePicker.HasMultiple() {
        return t.FocusIndex == 2
    }
    return t.FocusIndex == 1
}
```

同步更新 `isBranchPicker`、`isEnterButton` 等方法的 FocusIndex 判断。

#### `ui/overlay/textInput.go` — HandleKeyPress

在 `default` 分支中增加 `isDescriptionInput` 的处理：

```go
if t.isDescriptionInput() {
    t.descriptionInput, _ = t.descriptionInput.Update(msg)
    return false, false
}
```

在 `tea.KeyEnter` 中增加 `isDescriptionInput` 时前进到下一个焦点（类似 profile picker 的行为）：

```go
if t.isDescriptionInput() {
    t.setFocusIndex(t.FocusIndex + 1)
    return false, false
}
```

#### `ui/overlay/textInput.go` — updateFocusState

增加 descriptionInput 的 focus/blur 控制：

```go
if t.isDescriptionInput() {
    t.descriptionInput.Focus()
} else {
    t.descriptionInput.Blur()
}
```

#### `ui/overlay/textInput.go` — Render

在 textarea 和 branch picker 之间渲染 descriptionInput：

```go
content += tiTitleStyle.Render(t.Title) + "\n"
content += t.textarea.View() + "\n\n"

// Description input
content += tiDividerStyle.Render("─ " + "Description") + "\n"
content += t.descriptionInput.View() + "\n\n"

// Branch picker
if t.branchPicker != nil {
    content += divider + "\n\n"
    content += t.branchPicker.Render() + "\n\n"
}
```

#### `ui/overlay/textInput.go` — SetSize

设置 descriptionInput 宽度：

```go
t.descriptionInput.Width = width - 6
```

#### `ui/overlay/textInput.go` — 新增 GetDescription 方法

```go
func (t *TextInputOverlay) GetDescription() string {
    return t.descriptionInput.Value()
}
```

#### `app/app.go` — Prompt Overlay 提交处理

在 Overlay 提交后，从 overlay 获取 Description 并设置到 instance：

```go
selected.SetDescription(m.textInputOverlay.GetDescription())
```

---

### 5. 列表渲染变更

#### `ui/list.go` — InstanceRenderer.Render

当前渲染两行：title 行 + branch 行。

变更：在 branch 行后，若 `i.Description != ""`，新增第三行。

```go
// Description 行（仅在有描述时显示）
if i.Description != "" {
    descText := i.Description
    descWidthAvail := r.width - 3 - runewidth.StringWidth(prefix) - runewidth.StringWidth("☰ ")
    if descWidthAvail > 0 && runewidth.StringWidth(descText) > descWidthAvail {
        descText = runewidth.Truncate(descText, descWidthAvail-1, "…")
    }
    descLine := fmt.Sprintf("%s ☰ %s", strings.Repeat(" ", len(prefix)), descText)
    text = lipgloss.JoinVertical(lipgloss.Left, text, descS.Render(descLine))
}
```

#### `ui/list.go` — 新增样式

为 Description 行的图标添加样式，与 `listDescStyle` 保持一致的暗色风格：

```go
var descIconStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})
```

Description 行整体复用 `descS`（即 `listDescStyle` 或 `selectedDescStyle`），图标也用同色。

#### 5a. `stateDescription` 输入中的视觉改进

**问题**：进入 `stateDescription` 后，仅底部菜单提示 `enter: submit description (optional)`，用户不知道当前在输入什么字段。Description 为空时列表中不渲染 Description 行，视觉上无法区分"正在输入描述"和"已完成创建"。

**方案**：

1. **提前渲染 `☰` 行**：当处于 `stateDescription` 状态时，即使 Description 为空，也在列表中渲染 `☰` 行。

   在 `InstanceRenderer.Render` 中，需要额外传入一个参数表示是否处于 `stateDescription` 状态。或者更简洁的方案：在 `app/app.go` 中，当进入 `stateDescription` 时，给 instance 设置一个特殊状态标记，使列表渲染时始终显示 `☰` 行。

   **推荐方案**：在 `ui/list.go` 的 `InstanceRenderer` 中新增 `descInputActive bool` 字段，由 `app/app.go` 在进入/退出 `stateDescription` 时设置。当 `descInputActive == true` 时，无论 `i.Description` 是否为空，都渲染 `☰` 行。

2. **闪烁竖线光标**：在 `☰` 行末尾显示一个周期性显示/隐藏的竖线 `|`，表示正在输入。

   利用 Bubble Tea 的 spinner 机制：spinner 已在全局使用，可以在 `InstanceRenderer` 中根据 spinner 的当前帧决定是否显示光标字符。具体做法是在渲染 Description 行时，追加 spinner 的某个帧字符（如 `|`）。

   **实现方式**：在 `Render` 方法的参数中追加 `descInputActive bool`。当 `descInputActive && descText == ""` 时，显示 `☰ |`（光标闪烁）；当 `descInputActive && descText != ""` 时，显示 `☰ 描述内容|`（光标在末尾闪烁）。光标闪烁通过 spinner 的可见性控制——spinner 每 100ms 更新一次，利用 spinner 的交替帧实现闪烁效果。

   **简化方案**：使用一个 `cursorVisible bool` 字段在 `InstanceRenderer` 上，在 spinner tick 时交替设置。渲染时根据此字段决定是否显示末尾 `|`。

#### 5b. Description 行选中样式改进

**问题**：选中 instance 时，Description 行使用 `selectedDescStyle`（前景 `#1a1a1a`，背景 `#dde4f0`），与 branch 行颜色完全一致，无法区分。且 Description 为空时该行不渲染（无背景色），开始输入后背景色突然出现，造成视觉跳变。

**方案**：为 Description 行新增专用的选中样式，与 branch 行共享背景色但使用暗色前景：

```go
var selectedDescLineStyle = lipgloss.NewStyle().
    Padding(0, 1, 0, 1).
    Background(lipgloss.Color("#dde4f0")).
    Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"})
```

未选中时仍然复用 `listDescStyle`（前景 `#777777`，无背景色）。

在 `InstanceRenderer.Render` 中，Description 行不再使用 `descS`，而是根据选中状态选择：
- 未选中：`listDescStyle`
- 选中：`selectedDescLineStyle`

---

### 6. 菜单变更

#### `ui/menu.go` — MenuState 枚举

新增 `StateDescription`：

```go
const (
    StateDefault MenuState = iota
    StateEmpty
    StateNewInstance
    StateDescription  // 新增
    StatePrompt
)
```

#### `ui/menu.go` — 新增菜单选项和按键绑定

新增 `descriptionMenuOptions`：

```go
var descriptionMenuOptions = []keys.KeyName{keys.KeySubmitDescription}
```

在 `keys/keys.go` 中新增 `KeySubmitDescription`：

```go
KeySubmitDescription // 新增
```

新增按键绑定：

```go
KeySubmitDescription: key.NewBinding(
    key.WithKeys("enter"),
    key.WithHelp("enter", "submit description (optional)"),
),
```

在 `GlobalKeyStringsMap` 中无需新增映射（enter 已映射为 KeyEnter，在 `handleMenuHighlighting` 中需特殊处理 `stateDescription` 下的 enter 键，与 `stateNew` 下的处理类似）。

#### `ui/menu.go` — updateOptions

在 switch 中新增：

```go
case StateDescription:
    m.options = descriptionMenuOptions
```

#### `app/app.go` — handleMenuHighlighting

在 `stateNew` 的特殊处理后，新增 `stateDescription` 的处理：

```go
if name == keys.KeyEnter && m.state == stateDescription {
    name = keys.KeySubmitDescription
}
```

---

### 7. 帮助屏幕

无需变更。帮助屏幕已显示 `n: new` 和 `N: new with prompt`，Description 是创建流程中的中间步骤，不需要额外帮助条目。

---

## 需修改的文件清单

| 文件 | 变更内容 |
|------|---------|
| `session/instance.go` | Instance 结构体新增 Description 字段；InstanceOptions 新增 Description；NewInstance 赋值；新增 SetDescription 方法；ToInstanceData/FromInstanceData 序列化 |
| `session/storage.go` | InstanceData 新增 Description JSON 字段 |
| `app/app.go` | 新增 stateDescription 状态；stateNew Enter 分支改为进入 stateDescription；新增 stateDescription 按键处理块；Prompt Overlay 提交时获取 Description；handleMenuHighlighting 新增 stateDescription 处理 |
| `ui/list.go` | Render 方法中新增条件渲染 Description 第三行 |
| `ui/menu.go` | MenuState 新增 StateDescription；新增 descriptionMenuOptions；updateOptions 新增 StateDescription 分支 |
| `ui/overlay/textInput.go` | 新增 descriptionInput 字段；初始化；焦点站点数 +1；焦点判断方法更新；HandleKeyPress 新增 descriptionInput 处理；Render 新增 descriptionInput 区域；新增 GetDescription 方法；SetSize 更新 |
| `keys/keys.go` | 新增 KeySubmitDescription；GlobalkeyBindings 新增绑定 |

## 数据兼容性

- **向前兼容**：老版本 `state.json` 中无 `description` 字段，JSON 反序列化默认为零值空字符串，无需数据迁移。
- **向后兼容**：新版本写入的 `description` 字段会被老版本忽略（JSON 反序列化时未知字段默认忽略），不会导致老版本崩溃。

## 测试要点

1. `n` 键流程：Title → Enter → Description 输入 → Enter → 启动
2. `n` 键流程：Title → Enter → Description 留空 → Enter → 启动
3. `n` 键流程：Title → Enter → Description 输入 → Esc 跳过 → 启动
4. `n` 键流程：Title → Enter → ctrl+c 取消整个创建
5. `N` 键流程：Title → Enter → Prompt Overlay 中填写 Description → 提交 → 启动
6. `N` 键流程：Title → Enter → Prompt Overlay 中不填 Description → 提交 → 启动
7. 列表显示：有 Description 时显示第三行，无时不显示
8. 列表截断：Description 超长时正确截断并显示省略号
9. 中文字符：Description 支持中文输入和显示
10. 持久化：重启后 Description 保留
11. 向前兼容：加载无 description 字段的老版本 state.json 不报错
