package com.packpilot.android.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.AssistChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.packpilot.android.MainViewModel
import com.packpilot.android.UiState
import com.packpilot.android.data.AgentTask

/** 渲染服务器地址和页面密码登录页。 */
@Composable
fun LoginScreen(state: UiState, model: MainViewModel) {
    var password by remember { mutableStateOf("") }
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Text("PackPilot", style = MaterialTheme.typography.headlineLarge, fontWeight = FontWeight.Bold)
        Text("你的私有 Arch Linux 打包工作区", Modifier.padding(top = 8.dp), color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(32.dp))
        AppCard {
            Text("登录工作区", style = MaterialTheme.typography.titleLarge)
            Spacer(Modifier.height(18.dp))
            AppTextField(state.serverURL, model::updateServerURL, "服务器地址")
            Spacer(Modifier.height(12.dp))
            AppTextField(password, { password = it }, "页面密码", password = true)
            Spacer(Modifier.height(18.dp))
            AppButton(if (state.busy) "正在验证…" else "进入工作台", { model.login(password) }, enabled = !state.busy && password.length >= 12)
        }
        MessageBanner(state, model)
        Text("密码仅用于本次登录，不会保存到设备。", Modifier.padding(top = 16.dp), fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/** 渲染并行状态、任务搜索筛选列表和选中任务详情。 */
@Composable
fun TasksScreen(state: UiState, model: MainViewModel) {
    var search by remember { mutableStateOf("") }
    var filter by remember { mutableStateOf("all") }
    val tasks = state.tasks.filter { task ->
        val matchesFilter = filter == "all" || (filter == "active" && task.isActive()) || (filter == "finished" && !task.isActive())
        val matchesSearch = search.isBlank() || "${task.package_name} ${task.source_url} ${task.id}".contains(search, true)
        matchesFilter && matchesSearch
    }
    LazyColumn(Modifier.fillMaxSize(), contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        item {
            AppCard(Modifier.fillMaxWidth()) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Agent 工作台", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
                        Text("Docker ${state.health.docker} · 容量 ${state.health.worker.concurrency}", fontSize = 12.sp)
                    }
                    TextButton(onClick = model::logout) { Text("退出") }
                }
                Spacer(Modifier.height(12.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    Text("${state.health.worker.running} 运行", fontWeight = FontWeight.SemiBold)
                    Text("${state.health.worker.queued} 排队")
                    Text("${state.tasks.size} 任务")
                }
            }
        }
        item { AppTextField(search, { search = it }, "搜索包名、仓库或任务 ID") }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                FilterChip("全部", filter == "all") { filter = "all" }
                FilterChip("执行中", filter == "active") { filter = "active" }
                FilterChip("已结束", filter == "finished") { filter = "finished" }
            }
        }
        items(tasks, key = { it.id }) { task -> TaskCard(task, state.selected?.id == task.id) { model.selectTask(task) } }
        if (tasks.isEmpty()) item { Text("没有匹配的任务", Modifier.padding(24.dp), color = MaterialTheme.colorScheme.onSurfaceVariant) }
        item { MessageBanner(state, model) }
        state.selected?.let { task -> item { TaskDetail(task, state, model) } }
    }
}

/** 渲染一个任务卡片及其实时状态。 */
@Composable
private fun TaskCard(task: AgentTask, selected: Boolean, onClick: () -> Unit) {
    AppCard(Modifier.fillMaxWidth().clickable(onClick = onClick)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(task.package_name.ifBlank { "正在识别软件包" }, fontWeight = FontWeight.Bold)
                Text(task.source_url, fontSize = 11.sp, maxLines = 1, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (task.version.isNotBlank()) Text("v${task.version}", fontSize = 12.sp)
        }
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Text(statusLabel(task.status), color = MaterialTheme.colorScheme.primary, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
            if (task.attempts > 0) Text("${task.attempts} 次构建", fontSize = 11.sp)
            if (selected) Text("已选中", fontSize = 11.sp)
        }
    }
}

/** 渲染任务操作、实时日志和生成文件预览。 */
@Composable
private fun TaskDetail(task: AgentTask, state: UiState, model: MainViewModel) {
    var tab by remember(task.id) { mutableStateOf("agent.log") }
    var confirmAction by remember { mutableStateOf("") }
    AppCard(Modifier.fillMaxWidth().padding(bottom = 16.dp)) {
        Text("任务 ${task.id}", fontSize = 11.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(task.package_name, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
        Text(statusLabel(task.status), color = MaterialTheme.colorScheme.primary)
        if (task.error.isNotBlank()) Text(task.error, color = MaterialTheme.colorScheme.error, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
        Row(Modifier.padding(vertical = 12.dp).selectableGroup(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (task.isActive()) AppButton("取消", model::cancel, primary = false)
            if (task.status == "failed" || task.status == "canceled") AppButton("重试", model::retry, primary = false)
            if (task.status == "succeeded") AppButton("发布到 AUR", { confirmAction = "publish" })
            if (!task.isActive()) AppButton("删除", { confirmAction = "delete" }, primary = false)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            listOf("agent.log" to "日志", "PKGBUILD" to "PKGBUILD", "SRCINFO" to "SRCINFO", "build.log" to "构建").forEach { (name, label) ->
                FilterChip(label, tab == name) { tab = name; model.loadFile(name) }
            }
        }
        val content = if (tab == "agent.log") task.logs.joinToString("\n") { "[${it.level}] ${it.message}" } else if (state.fileName == tab) state.fileContent else "点击当前标签加载内容"
        SelectionContainer {
            Text(
                content.ifBlank { "暂无内容" },
                Modifier.fillMaxWidth().heightIn(min = 220.dp, max = 480.dp).verticalScroll(rememberScrollState()).padding(top = 12.dp),
                fontFamily = FontFamily.Monospace, fontSize = 11.sp, lineHeight = 17.sp,
            )
        }
    }
    if (confirmAction.isNotBlank()) {
        AlertDialog(
            onDismissRequest = { confirmAction = "" },
            title = { Text(if (confirmAction == "delete") "删除任务？" else "发布到 AUR？") },
            text = { Text(if (confirmAction == "delete") "将删除任务、日志和构建目录，此操作不可恢复。" else "将 PKGBUILD 和 .SRCINFO 推送到服务端配置的自建仓库。") },
            confirmButton = { TextButton(onClick = { if (confirmAction == "delete") model.delete() else model.publish(); confirmAction = "" }) { Text("确认") } },
            dismissButton = { TextButton(onClick = { confirmAction = "" }) { Text("取消") } },
        )
    }
}

/** 渲染新建打包任务表单。 */
@Composable
fun CreateTaskScreen(state: UiState, model: MainViewModel, onCreated: () -> Unit) {
    var sourceURL by remember { mutableStateOf("") }
    var packageName by remember { mutableStateOf("") }
    var publish by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
        Text("新建打包任务", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        Text("Agent 会发现源码、计算校验和并在 Arch 容器中迭代构建。", Modifier.padding(vertical = 8.dp), color = MaterialTheme.colorScheme.onSurfaceVariant)
        AppCard {
            AppTextField(sourceURL, { sourceURL = it }, "GitHub 仓库或软件官网")
            Spacer(Modifier.height(12.dp))
            AppTextField(packageName, { packageName = it }, "包名（可选）")
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 12.dp)) {
                androidx.compose.material3.Checkbox(checked = publish, onCheckedChange = { publish = it })
                Text("验证通过后自动发布到自建 AUR")
            }
            AppButton(if (state.busy) "正在创建…" else "开始构建", {
                model.createTask(sourceURL, packageName, publish)
                sourceURL = ""; packageName = ""; publish = false; onCreated()
            }, enabled = !state.busy && sourceURL.startsWith("http"))
        }
        MessageBanner(state, model)
    }
}

/** 渲染服务器与 MD3/Miuix、主题预设、深色模式设置。 */
@Composable
fun SettingsScreen(state: UiState, model: MainViewModel) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Text("设置", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        AppCard {
            Text("组件风格", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(10.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                UiStyle.entries.forEach { style -> FilterChip(style.label, state.style == style.name) { model.updateAppearance(style.name, state.preset, state.darkMode) } }
            }
        }
        AppCard {
            Text("主题预设", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(10.dp))
            ThemePreset.entries.forEach { preset ->
                Row(Modifier.fillMaxWidth().clickable { model.updateAppearance(state.style, preset.name, state.darkMode) }.padding(vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                    androidx.compose.material3.Surface(color = preset.seed, shape = androidx.compose.foundation.shape.CircleShape, modifier = Modifier.width(24.dp).height(24.dp)) {}
                    Text(preset.label, Modifier.padding(start = 12.dp))
                    if (state.preset == preset.name) { Spacer(Modifier.weight(1f)); Text("已选择", color = MaterialTheme.colorScheme.primary) }
                }
            }
        }
        AppCard {
            Text("显示模式", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(10.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                DarkMode.entries.forEach { mode -> FilterChip(mode.label, state.darkMode == mode.name) { model.updateAppearance(state.style, state.preset, mode.name) } }
            }
        }
        AppCard {
            Text("服务器地址", fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(10.dp))
            AppTextField(state.serverURL, model::updateServerURL, "http://10.0.2.2:8080")
            Text("新地址将在下次登录时生效。密码和会话不会保存在设置中。", Modifier.padding(top = 8.dp), fontSize = 11.sp)
        }
    }
}

/** 渲染统一样式的筛选按钮。 */
@Composable
private fun FilterChip(label: String, selected: Boolean, onClick: () -> Unit) {
    AssistChip(onClick = onClick, label = { Text(label, fontSize = 11.sp) }, border = if (selected) androidx.compose.material3.AssistChipDefaults.assistChipBorder(true, MaterialTheme.colorScheme.primary) else null)
}

/** 渲染可关闭的错误和成功通知。 */
@Composable
private fun MessageBanner(state: UiState, model: MainViewModel) {
    val message = state.error.ifBlank { state.notice }
    if (message.isNotBlank()) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 10.dp)) {
            Text(message, Modifier.weight(1f), color = if (state.error.isNotBlank()) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary, fontSize = 12.sp)
            TextButton(onClick = model::dismissMessages) { Text("关闭") }
        }
    }
}

/** 判断任务是否仍处于可取消的活动状态。 */
private fun AgentTask.isActive(): Boolean = status in setOf("queued", "discovering", "generating", "building", "publishing")
