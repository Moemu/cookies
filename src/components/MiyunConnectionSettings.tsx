import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Check, CircleAlert, ExternalLink, LockKeyhole, RefreshCw, Save } from "lucide-react";
import { useProject } from "../context/ProjectContext";
import { api, ApiRequestError, type ApiMiyunConnection } from "../data/api";

type ConnectionLoadState = "loading" | "ready" | "error";

const connectionCopy: Record<ApiMiyunConnection["status"], { title: string; detail: string }> = {
  unverified: { title: "已保存，尚未验证", detail: "保存后执行一次只读验证，确认会话仍可用。" },
  ready: { title: "连接正常", detail: "最近一次服务端验证通过，可以开始米云产品分析和采集。" },
  auth_required: { title: "需要更新会话", detail: "米云会话已失效或需要重新验证，请按右侧步骤更新。" },
  disabled: { title: "服务端未启用", detail: "请联系管理员启用米云连接能力后再验证会话。" },
};

function connectionError(error: unknown) {
  if (error instanceof ApiRequestError && error.code === "VERSION_CONFLICT") return "数据已更新，请刷新后重试。";
  if (error instanceof ApiRequestError && error.status === 403) return "你没有管理当前 Project 米云连接的权限。";
  return error instanceof Error ? error.message : "连接操作失败，请稍后重试。";
}

function verificationNotice(connection: ApiMiyunConnection) {
  if (connection.status === "ready") return "已完成一次只读验证，连接正常。";
  if (connection.status === "auth_required") return connectionCopy.auth_required.detail;
  if (connection.last_error_kind === "rate_limited") return "米云暂时限流，稍后再验证连接。";
  if (["invalid_request", "graphql_error", "malformed_response"].includes(connection.last_error_kind ?? "")) return "米云未接受本次验证请求。请确认仍在控制台登录后，重新复制完整 Cookie 值。";
  return "米云暂时未能完成验证。请稍后重试；若持续发生，请联系管理员检查米云服务可用性。";
}

export function MiyunConnectionSettings() {
  const { currentProject } = useProject();
  const sessionInputRef = useRef<HTMLInputElement>(null);
  const requestIdRef = useRef(0);
  const [connection, setConnection] = useState<ApiMiyunConnection | null>(null);
  const [loadState, setLoadState] = useState<ConnectionLoadState>("loading");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    const requestId = ++requestIdRef.current;
    setLoadState("loading");
    setNotice("");
    try {
      const next = await api.getMiyunConnection(currentProject.id);
      if (requestId !== requestIdRef.current) return;
      setConnection(next);
      setLoadState("ready");
    } catch (error) {
      if (requestId !== requestIdRef.current) return;
      if (error instanceof ApiRequestError && error.status === 404) {
        setConnection(null);
        setLoadState("ready");
      } else {
        setLoadState("error");
        setNotice(connectionError(error));
      }
    }
  }, [currentProject.id]);

  useEffect(() => {
    void load();
  }, [load]);

  const saveSession = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const session = sessionInputRef.current?.value.trim() ?? "";
    if (session.length < 8) {
      setNotice("请粘贴完整的 Cookie 值后再保存。");
      return;
    }
    if (session.length > 16356) {
      setNotice("Cookie 值超过 16 KiB，无法安全保存；请确认只复制 Cookie 的值。");
      return;
    }
    setBusy(true);
    setNotice("");
    try {
      const next = await api.updateMiyunConnection(currentProject.id, {
        session,
        expected_version: connection?.version ?? 0,
      });
      if (sessionInputRef.current) sessionInputRef.current.value = "";
      setConnection(next);
      setLoadState("ready");
      setNotice("会话已加密保存到服务端，未在本页保留或回显。请执行验证。" );
    } catch (error) {
      setNotice(connectionError(error));
    } finally {
      setBusy(false);
    }
  };

  const verify = async () => {
    if (!connection) return;
    setBusy(true);
    setNotice("");
    try {
      const next = await api.verifyMiyunConnection(currentProject.id, connection.version);
      setConnection(next);
      setNotice(verificationNotice(next));
    } catch (error) {
      setNotice(connectionError(error));
    } finally {
      setBusy(false);
    }
  };

  const copy = connection ? connectionCopy[connection.status] : { title: "尚未配置", detail: "请按右侧步骤保存授权会话。" };

  return (
    <section className="miyun-connection-settings" aria-labelledby="miyun-connection-settings-title">
      <div className="miyun-settings-main">
        <header>
          <div>
            <span>项目级安全配置</span>
            <h2 id="miyun-connection-settings-title">米云连接</h2>
            <p>当前 Project：{currentProject.name}</p>
          </div>
          <span className={`miyun-connection-status ${connection?.status === "ready" ? "ready" : ""}`}>
            {connection?.status === "ready" ? <Check size={14} /> : <CircleAlert size={14} />}
            {loadState === "loading" ? "正在读取" : copy.title}
          </span>
        </header>
        <div className="miyun-settings-secret-policy">
          <LockKeyhole size={18} />
          <p><b>只保存到服务端</b>{copy.detail} Cookie 不会回显；不要将其发送到聊天、截图或文档。</p>
        </div>
        <form className="miyun-session-form" onSubmit={saveSession}>
          <label htmlFor="miyun-session">米云 Cookie 请求头</label>
          <input id="miyun-session" ref={sessionInputRef} type="password" autoComplete="off" placeholder="粘贴完整 Cookie 值，仅用于本次保存" aria-describedby="miyun-session-help" />
          <small id="miyun-session-help">保存后输入框会立即清空；刷新页面也无法查看已保存内容。</small>
          <div className="miyun-settings-actions">
            <button className="secondary-button" type="button" onClick={() => void load()} disabled={busy || loadState === "loading"}><RefreshCw size={15} />刷新状态</button>
            <button className="secondary-button" type="button" onClick={() => void verify()} disabled={busy || !connection || connection.status === "disabled"}><RefreshCw size={15} />验证连接</button>
            <button className="primary-button" type="submit" disabled={busy}><Save size={15} />{busy ? "处理中" : "保存会话"}</button>
          </div>
        </form>
        {notice ? <p className="miyun-settings-notice" role="status" aria-live="polite">{notice}</p> : null}
      </div>
      <aside className="miyun-cookie-guide">
        <h3>获取 Cookie 的步骤</h3>
        <ol>
          <li><span>01</span><p>在 <a href="https://console.youshu.youcloud.com" target="_blank" rel="noreferrer">米云控制台 <ExternalLink size={12} /></a> 登录有授权的企业账号。</p></li>
          <li><span>02</span><p>打开素材列表页，按 <b>F12</b>，进入浏览器的 <b>Network（网络）</b> 面板后刷新页面。</p></li>
          <li><span>03</span><p>选择任意一个请求，在“标头 → 请求标头”列表中找到 <b>Cookie</b>，右键点击复制值；如果没有 Cookie 这一项就换一个请求。</p></li>
          <li><span>04</span><p>只粘贴到左侧表单并保存，然后点击“验证连接”。不要把值发给他人或放入文档。</p></li>
        </ol>
      </aside>
    </section>
  );
}
