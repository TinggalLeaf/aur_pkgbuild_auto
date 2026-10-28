package com.packpilot.android

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.packpilot.android.data.AgentTask
import com.packpilot.android.data.ApiClient
import com.packpilot.android.data.ApiException
import com.packpilot.android.data.Health
import com.packpilot.android.data.SettingsStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** 汇总 Android 界面的认证、任务、文件和外观状态。 */
data class UiState(
    val checking: Boolean = true,
    val authenticated: Boolean = false,
    val busy: Boolean = false,
    val serverURL: String = "",
    val tasks: List<AgentTask> = emptyList(),
    val health: Health = Health(),
    val selected: AgentTask? = null,
    val fileName: String = "",
    val fileContent: String = "",
    val error: String = "",
    val notice: String = "",
    val style: String = "MIUIX",
    val preset: String = "EMERALD",
    val darkMode: String = "SYSTEM",
)

/** 协调后台 API 请求、任务轮询和非敏感设置持久化。 */
class MainViewModel(application: Application) : AndroidViewModel(application) {
    private val settings = SettingsStore(application)
    private val mutableState = MutableStateFlow(
        UiState(
            serverURL = settings.serverURL(), style = settings.style(),
            preset = settings.preset(), darkMode = settings.darkMode(),
        ),
    )
    val state: StateFlow<UiState> = mutableState.asStateFlow()

    init {
        checkAuthentication()
    }

    /** 检查现有 Cookie 是否仍有效。 */
    fun checkAuthentication() = launch {
        val status = ApiClient(mutableState.value.serverURL).authStatus()
        mutableState.update { it.copy(authenticated = !status.enabled || status.authenticated, checking = false) }
        if (mutableState.value.authenticated) refresh()
    }

    /** 使用输入密码登录，密码不会进入 UiState 或 SharedPreferences。 */
    fun login(password: String) = launch {
        settings.saveServerURL(mutableState.value.serverURL)
        ApiClient(mutableState.value.serverURL).login(password)
        mutableState.update { it.copy(authenticated = true, checking = false, notice = "登录成功") }
        refresh()
    }

    /** 刷新任务、worker 状态和当前详情，供 Compose 定时轮询调用。 */
    fun refresh() = launch(showBusy = false) {
        val client = ApiClient(mutableState.value.serverURL)
        val tasks = client.tasks()
        val health = client.health()
        val selectedID = mutableState.value.selected?.id
        val selected = if (selectedID == null) mutableState.value.selected else client.task(selectedID)
        mutableState.update { it.copy(tasks = tasks, health = health, selected = selected, checking = false) }
    }

    /** 更新服务器地址输入，切换账号时清除任务状态。 */
    fun updateServerURL(value: String) = mutableState.update { it.copy(serverURL = value) }

    /** 创建任务并选中其详情。 */
    fun createTask(sourceURL: String, packageName: String, publish: Boolean) = launch {
        val task = ApiClient(mutableState.value.serverURL).createTask(sourceURL, packageName, publish)
        mutableState.update { it.copy(selected = task, notice = "任务已加入并行队列") }
        refresh()
    }

    /** 选择任务并清空旧文件预览。 */
    fun selectTask(task: AgentTask) = mutableState.update { it.copy(selected = task, fileContent = "", fileName = "") }

    /** 加载生成文件或日志的纯文本内容。 */
    fun loadFile(name: String) = launch {
        val id = mutableState.value.selected?.id ?: return@launch
        val content = ApiClient(mutableState.value.serverURL).file(id, name)
        mutableState.update { it.copy(fileName = name, fileContent = content) }
    }

    /** 取消当前任务。 */
    fun cancel() = taskAction("cancel", "任务已取消")

    /** 重试当前失败或取消任务。 */
    fun retry() = taskAction("retry", "任务已重新排队")

    /** 发布当前验证通过的软件包。 */
    fun publish() = taskAction("publish", "发布任务已启动")

    /** 删除当前终态任务并清除选择。 */
    fun delete() = launch {
        val id = mutableState.value.selected?.id ?: return@launch
        ApiClient(mutableState.value.serverURL).delete(id)
        mutableState.update { it.copy(selected = null, notice = "任务已删除") }
        refresh()
    }

    /** 退出会话并返回登录页。 */
    fun logout() = launch(showBusy = false) {
        runCatching { ApiClient(mutableState.value.serverURL).logout() }
        mutableState.update { it.copy(authenticated = false, tasks = emptyList(), selected = null, health = Health()) }
    }

    /** 保存 MD3/Miuix、主题预设和深色模式。 */
    fun updateAppearance(style: String, preset: String, darkMode: String) {
        settings.saveAppearance(style, preset, darkMode)
        mutableState.update { it.copy(style = style, preset = preset, darkMode = darkMode) }
    }

    /** 清除当前错误或通知。 */
    fun dismissMessages() = mutableState.update { it.copy(error = "", notice = "") }

    /** 执行一个任务操作并统一刷新。 */
    private fun taskAction(action: String, message: String) = launch {
        val id = mutableState.value.selected?.id ?: return@launch
        val client = ApiClient(mutableState.value.serverURL)
        when (action) {
            "cancel" -> client.cancel(id)
            "retry" -> client.retry(id)
            "publish" -> client.publish(id)
        }
        mutableState.update { it.copy(notice = message) }
        refresh()
    }

    /** 在 IO 调度器执行请求，统一处理 401、错误和 busy 状态。 */
    private fun launch(showBusy: Boolean = true, block: suspend () -> Unit) = viewModelScope.launch(Dispatchers.IO) {
        if (showBusy) mutableState.update { it.copy(busy = true, error = "") }
        try {
            block()
        } catch (error: ApiException) {
            mutableState.update { it.copy(error = error.message ?: "请求失败", authenticated = if (error.status == 401) false else it.authenticated) }
        } catch (error: Exception) {
            mutableState.update { it.copy(error = error.message ?: "网络连接失败", checking = false) }
        } finally {
            if (showBusy) mutableState.update { it.copy(busy = false) }
        }
    }
}
