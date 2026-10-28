package com.packpilot.android.data

import com.sun.net.httpserver.HttpServer
import java.net.CookieHandler
import java.net.CookieManager
import java.net.InetSocketAddress
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test

/** 使用本地 HTTP 服务验证真实请求、Cookie、模型和错误处理。 */
class ApiClientTest {
    private lateinit var server: HttpServer
    private lateinit var client: ApiClient

    /** 启动覆盖主要 API 的本地测试服务。 */
    @Before fun setUp() {
        CookieHandler.setDefault(CookieManager())
        server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        server.createContext("/api/auth/login") { exchange ->
            exchange.responseHeaders.add("Set-Cookie", "packpilot_session=test-cookie; HttpOnly")
            exchange.respond(200, "{\"authenticated\":true}")
        }
        server.createContext("/api/auth/status") { exchange ->
            val cookie = exchange.requestHeaders.getFirst("Cookie").orEmpty()
            exchange.respond(200, "{\"enabled\":true,\"authenticated\":${cookie.contains("test-cookie")}}")
        }
        server.createContext("/api/tasks") { exchange ->
            exchange.respond(200, "{\"tasks\":[{\"id\":\"0123456789abcdef\",\"source_url\":\"https://github.com/jpmens/jo\",\"status\":\"succeeded\",\"package_name\":\"jo\",\"version\":\"1.9\"}]}")
        }
        server.createContext("/api/tasks/0123456789abcdef/files/PKGBUILD") { exchange -> exchange.respond(200, "pkgname=jo\n") }
        server.createContext("/api/tasks/error") { exchange -> exchange.respond(401, "{\"error\":\"请先登录\"}") }
        server.start()
        client = ApiClient("http://127.0.0.1:${server.address.port}")
    }

    /** 关闭本地测试服务并清除 Cookie。 */
    @After fun tearDown() {
        server.stop(0)
        CookieHandler.setDefault(null)
    }

    /** 验证登录 Cookie 会随后续请求发送，任务和纯文本可解析。 */
    @Test fun loginTasksAndFile() = runBlocking {
        client.login("a-test-password")
        assertTrue(client.authStatus().authenticated)
        val tasks = client.tasks()
        assertEquals("jo", tasks.single().package_name)
        assertEquals("1.9", tasks.single().version)
        assertEquals("pkgname=jo\n", client.file("0123456789abcdef", "PKGBUILD"))
    }

    /** 验证 HTTP 401 被转换为带状态码的 ApiException。 */
    @Test fun unauthorized() = runBlocking {
        try {
            client.task("error")
            fail("应当抛出 ApiException")
        } catch (error: ApiException) {
            assertEquals(401, error.status)
            assertEquals("请先登录", error.message)
        }
    }

    /** 验证服务器地址规范化和非法输入。 */
    @Test fun normalizeURL() {
        assertEquals("http://10.0.2.2:8080", normalizeServerURL(" http://10.0.2.2:8080/ "))
        try {
            normalizeServerURL("file:///tmp/server")
            fail("非法协议未被拒绝")
        } catch (_: IllegalArgumentException) {
        }
    }

    /** 可选直连真实 Go 后端，只有显式提供测试地址和密码时执行。 */
    @Test fun liveBackendWhenConfigured() = runBlocking {
        val url = System.getenv("PACKPILOT_ANDROID_TEST_URL") ?: return@runBlocking
        val password = System.getenv("PACKPILOT_ANDROID_TEST_PASSWORD") ?: return@runBlocking
        val liveClient = ApiClient(url)
        liveClient.login(password)
        assertTrue(liveClient.authStatus().authenticated)
        assertTrue(liveClient.health().worker.concurrency >= 1)
        liveClient.tasks()
        liveClient.logout()
    }
}

/** 以指定状态码和 UTF-8 文本完成本地 HTTP 响应。 */
private fun com.sun.net.httpserver.HttpExchange.respond(status: Int, body: String) {
    val bytes = body.toByteArray()
    responseHeaders.add("Content-Type", "application/json; charset=utf-8")
    sendResponseHeaders(status, bytes.size.toLong())
    responseBody.use { it.write(bytes) }
}
