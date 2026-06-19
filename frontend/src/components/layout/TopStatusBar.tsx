import { Activity, DatabaseZap, KeyRound, RefreshCw, ShieldCheck } from "lucide-react";
import type { ReactNode } from "react";
import { runtimeModeLabel, serviceStatusLabel } from "@/lib/formatters";
import type { RuntimeStatus } from "@/types/runtime";

type Props = {
  status?: RuntimeStatus;
  loading: boolean;
  error?: string | null;
  onRefresh: () => void;
};

export function TopStatusBar({ status, loading, error, onRefresh }: Props) {
  const runtime = status?.runtime;
  const layers = status?.security_layers ? Object.keys(status.security_layers) : [];

  return (
    <header className="top-status-bar">
      <div className="brand-block">
        <div className="brand-mark">麒</div>
        <div>
          <strong>KylinGuard</strong>
          <span>麒盾智能体控制台</span>
        </div>
      </div>

      <div className="status-items">
        <StatusPill icon={<Activity size={15} />} label="运行模式" value={runtimeModeLabel(runtime?.chat_model)} />
        <StatusPill icon={<DatabaseZap size={15} />} label="后端服务" value={serviceStatusLabel(status?.services.go_agent.status)} />
        <StatusPill icon={<ShieldCheck size={15} />} label="安全防护" value={layers.length ? `${layers.length} 层` : "未知"} />
        <StatusPill
          icon={<KeyRound size={15} />}
          label="接口凭据"
          value={status?.secret_safety.api_key_present ? "已配置并隐藏" : "未配置"}
        />
      </div>

      {error ? <span className="status-error">{error}</span> : null}

      <button className="icon-button" type="button" onClick={onRefresh} disabled={loading} aria-label="刷新状态" title="刷新状态">
        <RefreshCw size={16} className={loading ? "spin" : ""} />
      </button>
    </header>
  );
}

function StatusPill({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="status-pill">
      {icon}
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
