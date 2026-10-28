"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { APIError, apiRequest, apiURL } from "@/lib/api";
import type { AgentTask, Health, TaskStatus } from "@/lib/types";
import { CheckIcon, ClockIcon, CloseIcon, DownloadIcon, ExternalIcon, FileIcon, LinkIcon, LogoutIcon, PackageIcon, PauseIcon, PlayIcon, PlusIcon, RefreshIcon, SearchIcon, SparkIcon, TerminalIcon, TrashIcon, UploadIcon } from "./icons";

const statusMeta: Record<TaskStatus, { label: string; tone: string }> = {
  queued: { label: "排队中", tone: "neutral" }, discovering: { label: "发现源码", tone: "active" },
  generating: { label: "生成文件", tone: "active" }, building: { label: "容器构建", tone: "active" },
  succeeded: { label: "验证通过", tone: "success" }, failed: { label: "执行失败", tone: "danger" },
  canceled: { label: "已取消", tone: "neutral" }, publishing: { label: "正在发布", tone: "active" },
  published: { label: "已发布", tone: "success" },
};
const activeStatuses: TaskStatus[] = ["queued", "discovering", "generating", "building", "publishing"];
type DetailTab = "logs" | "pkgbuild" | "srcinfo" | "build";

/** 将 ISO 时间转换为本地短日期时间。 */
function formatDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(new Date(value));
}

/** 将日志时间转换为带秒的本地时间。 */
function formatLogTime(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).format(new Date(value));
}

/** 从上游 URL 生成简洁的项目标签。 */
function sourceLabel(value: string): string {
  try {
    const parsed = new URL(value);
    return `${parsed.hostname}${parsed.pathname}`.replace(/\/$/, "");
  } catch { return value; }
}

/** 渲染统一的任务状态标签。 */
function StatusBadge({ status }: { status: TaskStatus }) {
  const meta = statusMeta[status];
  return <span className={`status-badge ${meta.tone}`}><span className="status-dot" />{meta.label}</span>;
}

/** 渲染 PackPilot 品牌。 */
function Brand() {
  return <div className="brand"><span className="brand-mark"><PackageIcon /></span><span>PackPilot<span className="brand-dot">.</span></span></div>;
}

/** 提供并行任务、实时日志、文件预览和生命周期操作的完整控制台。 */
export function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [selectedID, setSelectedID] = useState<string>();
  const [selectedTask, setSelectedTask] = useState<AgentTask>();
  const [health, setHealth] = useState<Health>();
  const [sourceURL, setSourceURL] = useState("");
  const [packageName, setPackageName] = useState("");
  const [publish, setPublish] = useState(false);
  const [filter, setFilter] = useState<"all" | "active" | "finished">("all");
  const [taskSearch, setTaskSearch] = useState("");
  const [tab, setTab] = useState<DetailTab>("logs");
  const [logLevel, setLogLevel] = useState("all");
  const [logSearch, setLogSearch] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [action, setAction] = useState("");
  const [loading, setLoading] = useState(true);
  const [usePolling, setUsePolling] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const logEnd = useRef<HTMLDivElement>(null);

  /** 将 API 错误映射到登录过期或页面错误提示。 */
  const handleFailure = useCallback((reason: unknown, fallback: string) => {
    if (reason instanceof APIError && reason.status === 401) {
      onLogout();
      return;
    }
    setError(reason instanceof Error ? reason.message : fallback);
  }, [onLogout]);

  const loadTasks = useCallback(async () => {
    try {
      const data = await apiRequest<{ tasks: AgentTask[] }>("/api/tasks");
      setTasks(data.tasks ?? []);
      setSelectedTask((current) => current ? data.tasks.find((task) => task.id === current.id) ?? current : current);
      setError("");
    } catch (reason) { handleFailure(reason, "无法连接后端"); }
    finally { setLoading(false); }
  }, [handleFailure]);

  const loadHealth = useCallback(async () => {
    try { setHealth(await apiRequest<Health>("/api/health")); }
    catch (reason) { handleFailure(reason, "健康检查失败"); }
  }, [handleFailure]);

  useEffect(() => {
    const initial = window.setTimeout(() => { void loadTasks(); void loadHealth(); }, 0);
    const timer = window.setInterval(() => { void loadTasks(); void loadHealth(); }, 4000);
    return () => { window.clearTimeout(initial); window.clearInterval(timer); };
  }, [loadTasks, loadHealth]);

  useEffect(() => {
    if (!selectedID) return;
    const events = new EventSource(apiURL(`/api/tasks/${selectedID}/events`), { withCredentials: true });
    events.addEventListener("task", (event) => {
      const task = JSON.parse((event as MessageEvent).data) as AgentTask;
      setSelectedTask(task);
      setTasks((current) => current.map((item) => item.id === task.id ? task : item));
      if (!activeStatuses.includes(task.status)) events.close();
    });
    events.onerror = () => { events.close(); setUsePolling(true); };
    return () => events.close();
  }, [selectedID]);

  useEffect(() => {
    if (!selectedID || !usePolling) return;
    const refresh = async () => {
      try { setSelectedTask(await apiRequest<AgentTask>(`/api/tasks/${selectedID}`)); }
      catch (reason) { handleFailure(reason, "任务刷新失败"); }
    };
    const initial = window.setTimeout(() => void refresh(), 0);
    const timer = window.setInterval(() => void refresh(), 2000);
    return () => { window.clearTimeout(initial); window.clearInterval(timer); };
  }, [selectedID, usePolling, handleFailure]);

  useEffect(() => {
    if (autoScroll) logEnd.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [selectedTask?.logs.length, autoScroll, tab]);

  const visibleTasks = useMemo(() => tasks.filter((task) => {
    if (filter === "active" && !activeStatuses.includes(task.status)) return false;
    if (filter === "finished" && activeStatuses.includes(task.status)) return false;
    const query = taskSearch.trim().toLowerCase();
    return !query || `${task.package_name} ${task.source_url} ${task.id}`.toLowerCase().includes(query);
  }), [tasks, filter, taskSearch]);

  const visibleLogs = useMemo(() => (selectedTask?.logs ?? []).filter((log) => {
    if (logLevel !== "all" && log.level !== logLevel) return false;
    return !logSearch.trim() || log.message.toLowerCase().includes(logSearch.trim().toLowerCase());
  }), [selectedTask?.logs, logLevel, logSearch]);

  /** 创建任务并立即打开其实时日志视图。 */
  async function createTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true); setError("");
    try {
      const task = await apiRequest<AgentTask>("/api/tasks", { method: "POST", body: JSON.stringify({ source_url: sourceURL, package_name: packageName, publish }) });
      setTasks((current) => [task, ...current]); openTask(task);
      setSourceURL(""); setPackageName(""); setPublish(false);
      setNotice("任务已加入并行队列"); void loadHealth();
    } catch (reason) { handleFailure(reason, "创建任务失败"); }
    finally { setSubmitting(false); }
  }

  /** 选择任务并默认展示执行日志。 */
  function openTask(task: AgentTask) {
    setSelectedID(task.id); setSelectedTask(task); setUsePolling(false); setTab("logs"); setNotice("");
  }

  /** 取消当前运行或排队任务。 */
  async function cancelTask() {
    if (!selectedTask) return;
    setAction("cancel");
    try {
      await apiRequest(`/api/tasks/${selectedTask.id}/cancel`, { method: "POST" });
      setSelectedTask(await apiRequest<AgentTask>(`/api/tasks/${selectedTask.id}`));
      setNotice("任务已取消"); void loadTasks();
    } catch (reason) { handleFailure(reason, "取消失败"); }
    finally { setAction(""); }
  }

  /** 将失败或取消的任务重新加入队列。 */
  async function retryTask() {
    if (!selectedTask) return;
    setAction("retry");
    try {
      await apiRequest(`/api/tasks/${selectedTask.id}/retry`, { method: "POST" });
      setSelectedTask(await apiRequest<AgentTask>(`/api/tasks/${selectedTask.id}`));
      setUsePolling(false); setNotice("已重新加入执行队列"); void loadHealth();
    } catch (reason) { handleFailure(reason, "重试失败"); }
    finally { setAction(""); }
  }

  /** 经确认删除终态任务及工作目录。 */
  async function deleteTask() {
    if (!selectedTask || !window.confirm("确定删除该任务、日志和构建工作目录吗？此操作不可恢复。")) return;
    setAction("delete");
    try {
      await apiRequest(`/api/tasks/${selectedTask.id}`, { method: "DELETE" });
      setSelectedID(undefined); setSelectedTask(undefined); setNotice("任务已删除"); void loadTasks();
    } catch (reason) { handleFailure(reason, "删除失败"); }
    finally { setAction(""); }
  }

  /** 经确认把验证通过的软件包发布到自建 aurweb。 */
  async function publishTask() {
    if (!selectedTask || !window.confirm(`确定将 ${selectedTask.package_name} 推送到自建 AUR 仓库吗？`)) return;
    setAction("publish");
    try {
      await apiRequest(`/api/tasks/${selectedTask.id}/publish`, { method: "POST" });
      setSelectedTask(await apiRequest<AgentTask>(`/api/tasks/${selectedTask.id}`));
      setNotice("发布任务已启动");
    } catch (reason) { handleFailure(reason, "发布失败"); }
    finally { setAction(""); }
  }

  /** 清除服务端会话 Cookie 并返回登录页。 */
  async function logout() {
    try { await apiRequest("/api/auth/logout", { method: "POST" }); }
    finally { onLogout(); }
  }

  const activeCount = tasks.filter((task) => activeStatuses.includes(task.status)).length;
  const successCount = tasks.filter((task) => task.status === "succeeded" || task.status === "published").length;
  const preview = tab === "pkgbuild" ? selectedTask?.pkgbuild : tab === "srcinfo" ? selectedTask?.srcinfo : selectedTask?.build_log;
  const downloadName = tab === "pkgbuild" ? "PKGBUILD" : tab === "srcinfo" ? "SRCINFO" : tab === "build" ? "build.log" : "agent.log";

  return (
    <div className="app-shell">
      <aside className="navigation">
        <Brand />
        <nav aria-label="主导航"><a className="nav-item active" href="#workspace"><SparkIcon />Agent 工作台</a><a className="nav-item" href="#tasks"><ClockIcon />构建任务<span className="nav-count">{tasks.length}</span></a></nav>
        <div className="nav-bottom">
          <div className="environment-card"><div className="environment-title"><span className={`health-dot ${health?.docker === "ok" ? "online" : ""}`} />执行环境</div><div className="environment-row"><span>Go API</span><strong>{health ? "已连接" : "检查中"}</strong></div><div className="environment-row"><span>Arch Docker</span><strong>{health?.docker === "ok" ? "就绪" : "不可用"}</strong></div><div className="environment-row"><span>并行 worker</span><strong>{health?.worker.running ?? 0} / {health?.worker.concurrency ?? "—"}</strong></div></div>
          <button className="logout-button" onClick={logout}><LogoutIcon />退出工作区</button>
          <span className="version-label">PACKPILOT / V1.1</span>
        </div>
      </aside>

      <main className="main-content" id="workspace">
        <header className="topbar"><div><span className="eyebrow">AUTONOMOUS PACKAGING</span><h1>Agent 工作台</h1></div><div className="worker-pill"><span className={`health-dot ${health ? "online" : ""}`} /><strong>{health?.worker.running ?? 0}</strong> 运行<span>/</span><strong>{health?.worker.queued ?? 0}</strong> 排队<span className="worker-cap">容量 {health?.worker.concurrency ?? "—"}</span></div></header>

        <section className="hero-section">
          <div className="hero-copy"><span className="hero-kicker"><SparkIcon />FROM SOURCE TO AUR</span><h2>给一个链接，<br />交付一个<span>可用的包。</span></h2><p>多个任务并行发现源码、生成 PKGBUILD，在隔离 Arch 容器中构建，并保留每一步证据。</p><div className="trust-row"><span><CheckIcon />真实 makepkg</span><span><CheckIcon />签名登录会话</span><span><CheckIcon />完整日志</span></div></div>
          <form className="create-card" onSubmit={createTask}><label htmlFor="source-url">GitHub 仓库或软件官网</label><div className="url-input"><LinkIcon /><input id="source-url" name="source_url" type="url" required placeholder="https://github.com/owner/project" value={sourceURL} onChange={(event) => setSourceURL(event.target.value)} /></div><div className="form-row"><div className="package-field"><label htmlFor="package-name">包名 <span>可选</span></label><input id="package-name" name="package_name" placeholder="由 Agent 自动判断" value={packageName} onChange={(event) => setPackageName(event.target.value)} /></div><button className="primary-button" disabled={submitting} type="submit">{submitting ? <span className="spinner" /> : <PlusIcon />}{submitting ? "正在创建" : "开始构建"}</button></div><label className="publish-check"><input type="checkbox" name="publish" checked={publish} onChange={(event) => setPublish(event.target.checked)} /><span>验证通过后自动发布到自建 AUR</span><small>需服务端开启</small></label></form>
        </section>

        <section className="metrics" aria-label="任务统计"><div><span className="metric-icon violet"><TerminalIcon /></span><p>全部任务<strong>{tasks.length}</strong></p></div><div><span className="metric-icon amber"><ClockIcon /></span><p>正在执行<strong>{activeCount}</strong></p></div><div><span className="metric-icon green"><CheckIcon /></span><p>验证通过<strong>{successCount}</strong></p></div><div className="capacity-meter"><span>并行容量</span><div>{Array.from({ length: health?.worker.concurrency ?? 2 }).map((_, index) => <i key={index} className={index < (health?.worker.running ?? 0) ? "used" : ""} />)}</div><small>{health?.worker.running ?? 0} / {health?.worker.concurrency ?? 2} 个容器</small></div></section>

        {error && <div className="error-banner" role="alert">{error}<button onClick={() => setError("")} aria-label="关闭错误"><CloseIcon /></button></div>}
        {notice && <div className="notice-banner" role="status"><CheckIcon />{notice}<button onClick={() => setNotice("")} aria-label="关闭通知"><CloseIcon /></button></div>}

        <section className="workspace-grid" id="tasks">
          <div className="task-panel">
            <div className="panel-heading"><div><h3>构建任务</h3><span>{visibleTasks.length} 个结果</span></div><div className="filters"><button className={filter === "all" ? "selected" : ""} onClick={() => setFilter("all")}>全部</button><button className={filter === "active" ? "selected" : ""} onClick={() => setFilter("active")}>执行中</button><button className={filter === "finished" ? "selected" : ""} onClick={() => setFilter("finished")}>已结束</button></div></div>
            <div className="task-search"><SearchIcon /><input aria-label="搜索任务" name="task_search" placeholder="搜索包名、仓库或任务 ID" value={taskSearch} onChange={(event) => setTaskSearch(event.target.value)} />{taskSearch && <button onClick={() => setTaskSearch("")} aria-label="清空搜索"><CloseIcon /></button>}</div>
            <div className="task-list">{loading && <div className="empty-state"><span className="spinner dark" /><p>正在读取任务…</p></div>}{!loading && visibleTasks.length === 0 && <div className="empty-state"><PackageIcon /><h4>{tasks.length ? "没有匹配的任务" : "还没有构建任务"}</h4><p>{tasks.length ? "尝试调整状态或搜索条件。" : "在上方粘贴仓库或官网地址，让 Agent 开始工作。"}</p></div>}{visibleTasks.map((task) => <button key={task.id} className={`task-card ${selectedID === task.id ? "active" : ""}`} onClick={() => openTask(task)}><span className="task-package-icon"><PackageIcon /></span><span className="task-card-body"><span className="task-title"><strong>{task.package_name || "正在识别软件包"}</strong>{task.version && <code>v{task.version}</code>}</span><span className="task-source">{sourceLabel(task.source_url)}</span><span className="task-meta"><StatusBadge status={task.status} /><time>{formatDate(task.created_at)}</time>{task.attempts > 0 && <span>{task.attempts} 次构建</span>}</span></span></button>)}</div>
          </div>

          <div className="detail-panel">
            {!selectedTask && <div className="detail-empty"><span><TerminalIcon /></span><h3>任务详情与实时日志</h3><p>选择左侧任务，查看 Agent 进度、容器输出和生成的软件包文件。</p></div>}
            {selectedTask && <>
              <div className="detail-heading"><div><span className="detail-label">TASK / {selectedTask.id.toUpperCase()}</span><h3>{selectedTask.package_name || "源码分析中"}{selectedTask.version && <span>v{selectedTask.version}</span>}</h3></div><div className="detail-actions">{activeStatuses.includes(selectedTask.status) && <button className="text-button danger-text" disabled={action === "cancel"} onClick={cancelTask}>取消任务</button>}{(selectedTask.status === "failed" || selectedTask.status === "canceled") && <button className="secondary-button" disabled={action === "retry"} onClick={retryTask}><RefreshIcon />{action === "retry" ? "重试中" : "重试"}</button>}{selectedTask.status === "succeeded" && <button className="small-primary" onClick={publishTask} disabled={action === "publish"}><UploadIcon />{action === "publish" ? "发布中" : "发布到 AUR"}</button>}{!activeStatuses.includes(selectedTask.status) && <button className="icon-button danger-text" title="删除任务" aria-label="删除任务" onClick={deleteTask}><TrashIcon /></button>}</div></div>
              <div className="detail-status-line"><StatusBadge status={selectedTask.status} /><a href={selectedTask.source_url} target="_blank" rel="noreferrer">上游地址<ExternalIcon /></a>{selectedTask.publication && <a href={selectedTask.publication.package_url} target="_blank" rel="noreferrer">查看 AUR 包<ExternalIcon /></a>}<span className="build-attempts">{selectedTask.attempts} 次容器构建</span></div>
              {selectedTask.error && <div className="task-error">{selectedTask.error}</div>}
              <div className="detail-tabs"><button className={tab === "logs" ? "selected" : ""} onClick={() => setTab("logs")}><TerminalIcon />实时日志</button><button className={tab === "pkgbuild" ? "selected" : ""} onClick={() => setTab("pkgbuild")}><FileIcon />PKGBUILD</button><button className={tab === "srcinfo" ? "selected" : ""} onClick={() => setTab("srcinfo")}>.SRCINFO</button><button className={tab === "build" ? "selected" : ""} onClick={() => setTab("build")}>构建输出</button><a href={apiURL(`/api/tasks/${selectedTask.id}/files/${downloadName}`)} title="下载当前内容"><DownloadIcon />下载</a></div>
              {tab === "logs" ? <div className="run-log"><div className="log-heading"><span><TerminalIcon />Agent 执行日志</span><div className="log-controls"><select aria-label="日志级别" name="log_level" value={logLevel} onChange={(event) => setLogLevel(event.target.value)}><option value="all">全部级别</option><option value="info">信息</option><option value="agent">Agent</option><option value="warn">警告</option><option value="error">错误</option></select><input aria-label="搜索日志" name="log_search" placeholder="筛选日志" value={logSearch} onChange={(event) => setLogSearch(event.target.value)} /><button title={autoScroll ? "暂停自动滚动" : "开启自动滚动"} onClick={() => setAutoScroll(!autoScroll)}>{autoScroll ? <PauseIcon /> : <PlayIcon />}</button></div></div><div className="log-lines">{visibleLogs.length === 0 && <p className="log-muted">没有符合当前条件的日志。</p>}{visibleLogs.map((log, index) => <div className={`log-line ${log.level}`} key={`${log.time}-${index}`}><time>{formatLogTime(log.time)}</time><span>{log.message}</span></div>)}<div ref={logEnd} /></div><div className="log-footer"><span>{visibleLogs.length} / {selectedTask.logs.length} 条日志</span><span>{autoScroll ? "自动滚动已开启" : "自动滚动已暂停"}</span></div></div> : <div className="file-preview"><pre>{preview || (tab === "build" ? "容器构建后，完整输出会显示在这里。" : "Agent 提交候选后，文件内容会显示在这里。")}</pre></div>}
            </>}
          </div>
        </section>
        <footer><span>PackPilot · 自主 Arch Linux 打包 Agent</span><span>并行执行 · 签名会话 · 完整审计</span></footer>
      </main>
    </div>
  );
}
