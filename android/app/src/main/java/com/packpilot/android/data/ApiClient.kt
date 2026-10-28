package com.packpilot.android.data

import java.net.HttpURLConnection
import java.net.URI
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

/** 表示带 HTTP 状态码的 PackPilot API 异常。 */
class ApiException(message: String, val status: Int) : Exception(message)

/** 规范化服务器地址，拒绝非 HTTP(S)、查询参数和片段。 */
fun normalizeServerURL(raw: String): String {
    val value = raw.trim().trimEnd('/')
    val uri = URI(value)
    require((uri.scheme == "http" || uri.scheme == "https") && !uri.host.isNullOrBlank()) { "请输入完整的 HTTP(S) 服务器地址" }
    require(uri.query == null && uri.fragment == null && uri.userInfo == null) { "服务器地址不能包含凭据、查询参数或片段" }
    return value
}

/** 使用系统 CookieHandler 和 HttpURLConnection 访问 PackPilot API。 */
class ApiClient(serverURL: String) {
    private val baseURL = normalizeServerURL(serverURL)
    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    /** 查询认证开关和当前 Cookie 状态。 */
    suspend fun authStatus(): AuthStatus = get("/api/auth/status")

    /** 提交页面密码，Cookie 由系统 CookieManager 保存。 */
    suspend fun login(password: String) {
        request<Unit>("/api/auth/login", "POST", json.encodeToString(LoginRequest(password)))
    }

    /** 退出当前会话。 */
    suspend fun logout() = request<Unit>("/api/auth/logout", "POST")

    /** 获取 Docker 和并行 worker 状态。 */
    suspend fun health(): Health = get("/api/health")

    /** 获取全部任务。 */
    suspend fun tasks(): List<AgentTask> = get<TaskList>("/api/tasks").tasks

    /** 创建打包任务。 */
    suspend fun createTask(sourceURL: String, packageName: String, publish: Boolean): AgentTask =
        request("/api/tasks", "POST", json.encodeToString(CreateTaskRequest(sourceURL, packageName, publish)))

    /** 获取单个任务快照。 */
    suspend fun task(id: String): AgentTask = get("/api/tasks/$id")

    /** 取消排队或运行任务。 */
    suspend fun cancel(id: String) = action(id, "cancel")

    /** 重试失败或取消任务。 */
    suspend fun retry(id: String) = action(id, "retry")

    /** 发布验证通过的软件包。 */
    suspend fun publish(id: String) = action(id, "publish")

    /** 删除终态任务。 */
    suspend fun delete(id: String) = request<Unit>("/api/tasks/$id", "DELETE")

    /** 下载 PKGBUILD、SRCINFO、build.log 或 agent.log 的原始文本。 */
    suspend fun file(id: String, name: String): String = requestText("/api/tasks/$id/files/$name", "GET", null)

    /** 执行一个任务动作接口。 */
    private suspend fun action(id: String, action: String): ActionStatus = request("/api/tasks/$id/$action", "POST")

    /** 执行 GET 并解码 JSON 响应。 */
    private suspend inline fun <reified T> get(path: String): T = request(path, "GET")

    /** 执行 HTTP 请求并统一处理空响应、错误体和状态码。 */
    private suspend inline fun <reified T> request(path: String, method: String, body: String? = null): T {
        val text = requestText(path, method, body)
        return if (T::class == Unit::class || text.isBlank()) Unit as T else json.decodeFromString(text)
    }

    /** 在调用协程中执行阻塞网络请求，ViewModel 负责切换到 IO 调度器。 */
    private fun requestText(path: String, method: String, body: String?): String {
        val connection = (URI(baseURL + path).toURL().openConnection() as HttpURLConnection)
        try {
            connection.requestMethod = method
            connection.connectTimeout = 15_000
            connection.readTimeout = 60_000
            connection.setRequestProperty("Accept", "application/json, text/plain")
            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json; charset=utf-8")
                connection.outputStream.bufferedWriter().use { it.write(body) }
            }
            val status = connection.responseCode
            val stream = if (status in 200..299) connection.inputStream else connection.errorStream
            val text = stream?.bufferedReader()?.use { it.readText() }.orEmpty()
            if (status !in 200..299) {
                val message = runCatching { json.decodeFromString<Map<String, String>>(text)["error"] }.getOrNull() ?: "请求失败（HTTP $status）"
                throw ApiException(message, status)
            }
            return text
        } finally {
            connection.disconnect()
        }
    }
}
