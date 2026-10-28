package com.packpilot.android.data

import kotlinx.serialization.Serializable

@Serializable
data class AuthStatus(val enabled: Boolean = true, val authenticated: Boolean = false)

@Serializable
data class LoginRequest(val password: String)

@Serializable
data class CreateTaskRequest(val source_url: String, val package_name: String = "", val publish: Boolean = false)

@Serializable
data class LogEntry(val time: String = "", val level: String = "info", val message: String = "")

@Serializable
data class Publication(
    val repository: String = "",
    val branch: String = "",
    val commit: String = "",
    val package_url: String = "",
    val pushed_at: String = "",
)

@Serializable
data class AgentTask(
    val id: String = "",
    val source_url: String = "",
    val package_name: String = "",
    val version: String = "",
    val description: String = "",
    val upstream_url: String = "",
    val source_archive_url: String = "",
    val status: String = "queued",
    val publish_requested: Boolean = false,
    val attempts: Int = 0,
    val pkgbuild: String = "",
    val srcinfo: String = "",
    val build_log: String = "",
    val error: String = "",
    val logs: List<LogEntry> = emptyList(),
    val publication: Publication? = null,
    val created_at: String = "",
    val updated_at: String = "",
    val finished_at: String? = null,
)

@Serializable
data class TaskList(val tasks: List<AgentTask> = emptyList())

@Serializable
data class WorkerStats(val concurrency: Int = 1, val running: Int = 0, val queued: Int = 0)

@Serializable
data class Health(val status: String = "", val docker: String = "", val worker: WorkerStats = WorkerStats(), val time: String = "")

@Serializable
data class ActionStatus(val status: String = "")
