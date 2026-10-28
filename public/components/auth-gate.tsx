"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiRequest } from "@/lib/api";
import type { AuthStatus } from "@/lib/types";
import { Dashboard } from "./dashboard";
import { PackageIcon, SparkIcon } from "./icons";

/** 在加载控制台之前检查登录状态，并在各视图间切换。 */
export function AuthGate() {
  const [view, setView] = useState<"loading" | "login" | "authenticated">("loading");

  useEffect(() => {
    void apiRequest<AuthStatus>("/api/auth/status").then((status) => {
      setView(!status.enabled || status.authenticated ? "authenticated" : "login");
    }).catch(() => setView("login"));
  }, []);

  if (view === "loading") return <div className="auth-loading"><span className="spinner dark" /><p>正在安全检查工作区…</p></div>;
  if (view === "login") return <LoginScreen onSuccess={() => setView("authenticated")} />;
  return <Dashboard onLogout={() => setView("login")} />;
}

/** 渲染页面密码登录表单并处理错误和提交状态。 */
function LoginScreen({ onSuccess }: { onSuccess: () => void }) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  /** 提交页面密码，登录成功后由父组件挂载工作台。 */
  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await apiRequest("/api/auth/login", { method: "POST", body: JSON.stringify({ password }) });
      onSuccess();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "登录失败，请重试");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-brand-panel">
        <div className="brand"><span className="brand-mark"><PackageIcon /></span><span>PackPilot<span className="brand-dot">.</span></span></div>
        <div className="login-pitch"><span className="hero-kicker"><SparkIcon />AUTONOMOUS PACKAGING</span><h1>让上游源码，<br />安全地抵达 AUR。</h1><p>自主发现、并行构建、真实验证。你的私有 Arch Linux 打包工作区。</p></div>
        <div className="login-features"><span>隔离的 Arch 容器</span><span>加密签名会话</span><span>完整执行日志</span></div>
      </section>
      <section className="login-form-panel">
        <form className="login-form" onSubmit={login}>
          <span className="login-lock"><PackageIcon /></span>
          <span className="eyebrow">PRIVATE WORKSPACE</span>
          <h2>欢迎回来</h2>
          <p>输入工作区密码以继续。</p>
          <input type="hidden" name="username" autoComplete="username" value="workspace" readOnly />
          <label htmlFor="workspace-password">页面密码</label>
          <input id="workspace-password" name="password" type="password" autoFocus required minLength={12} autoComplete="current-password" placeholder="至少 12 个字符" value={password} onChange={(event) => setPassword(event.target.value)} />
          {error && <div className="login-error" role="alert">{error}</div>}
          <button className="primary-button login-button" disabled={submitting} type="submit">{submitting ? <span className="spinner" /> : null}{submitting ? "正在验证…" : "进入工作台"}</button>
          <small>会话使用 HttpOnly Cookie 保存，密码不会写入浏览器存储。</small>
        </form>
      </section>
    </main>
  );
}
