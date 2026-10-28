import { AuthGate } from "@/components/auth-gate";

/** 渲染带登录认证的 AUR Agent 中文工作台首页。 */
export default function Home() {
  return <AuthGate />;
}
