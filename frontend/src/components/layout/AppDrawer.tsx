"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import { CopilotChat, type AttachmentsConfig } from "@copilotkit/react-core/v2";
import {
  CheckCircle2,
  ChevronDown,
  GripHorizontal,
  Loader2,
  MessageSquare,
  Plus,
  Settings,
  Wrench,
  X,
} from "lucide-react";
import { useConsolePreferences, type ConsolePreferences } from "@/hooks/useConsolePreferences";
import type { AgentRun, AgentStep, ToolTrace } from "@/types/agent";

type TabKey = "chat" | "settings";

export type AppDrawerHandle = {
  openToSettings: () => void;
};

type AppDrawerProps = {
  currentRun?: AgentRun | null;
};

const MIN_W = 560;
const MIN_H = 430;
const MAX_W_RATIO = 0.92;
const MAX_H_RATIO = 0.88;
const DEFAULT_W = 860;
const DEFAULT_H = 680;
const EDGE_GAP = 12;

const attachmentAccept =
  "text/plain,text/markdown,application/json,application/pdf,image/png,image/jpeg,.log,.conf,.ini,.yaml,.yml,.md,.txt,.json";
const attachmentMaxSize = 5 * 1024 * 1024;

type AttachmentUploadResult = {
  type: "data";
  value: string;
  mimeType: string;
  metadata?: Record<string, unknown>;
};

type AttachmentUploadError = {
  reason: string;
  message: string;
};

type DrawerSize = { width: number; height: number };
type DrawerPosition = { left: number; top: number };

export const AppDrawer = forwardRef<AppDrawerHandle, AppDrawerProps>(function AppDrawer({ currentRun }, ref) {
  const { preferences, updatePreferences, resetPreferences, hydrated } = useConsolePreferences();
  const [isOpen, setIsOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<TabKey>("chat");
  const [size, setSize] = useState<DrawerSize>({ width: DEFAULT_W, height: DEFAULT_H });
  const [position, setPosition] = useState<DrawerPosition | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isThinking, setIsThinking] = useState(false);
  const [hasChatActivity, setHasChatActivity] = useState(false);
  const modalRef = useRef<HTMLDivElement>(null);
  const resizeRef = useRef<{ startX: number; startY: number; startW: number; startH: number } | null>(null);
  const dragRef = useRef<{ startX: number; startY: number; startLeft: number; startTop: number } | null>(null);
  const lastRunID = currentRun?.run_id || currentRun?.task_id || "";

  useImperativeHandle(ref, () => ({
    openToSettings() {
      setActiveTab("settings");
      setIsOpen(true);
    },
  }));

  useEffect(() => {
    if (hydrated && preferences.chatDefaultOpen) {
      setIsOpen(true);
    }
  }, [hydrated, preferences.chatDefaultOpen]);

  useEffect(() => {
    if (isOpen && !position && typeof window !== "undefined") {
      setPosition(defaultPosition(preferences.chatPosition, size));
    }
  }, [isOpen, position, preferences.chatPosition, size]);

  useEffect(() => {
    if (!hydrated) return;
    const nextSize = clampSize({ width: preferences.chatWidth, height: preferences.chatHeight });
    setSize(nextSize);
    setPosition((current) => {
      if (!isOpen && current) return current;
      return defaultPosition(preferences.chatPosition, nextSize);
    });
  }, [hydrated, isOpen, preferences.chatHeight, preferences.chatPosition, preferences.chatWidth]);

  useEffect(() => {
    if (lastRunID) {
      setIsThinking(false);
      setHasChatActivity(true);
    }
  }, [lastRunID]);

  useEffect(() => {
    if (!isThinking) return;
    const timeout = window.setTimeout(() => setIsThinking(false), 90000);
    return () => window.clearTimeout(timeout);
  }, [isThinking]);

  useEffect(() => {
    if (!isOpen || typeof window === "undefined") return;
    const onResize = () => {
      setSize((current) => clampSize(current));
      setPosition((current) => (current ? clampPosition(current, size) : centerPosition(size)));
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [isOpen, size]);

  const attachments = useMemo<AttachmentsConfig>(
    () => ({
      enabled: preferences.attachmentsEnabled,
      accept: attachmentAccept,
      maxSize: attachmentMaxSize,
      onUpload: uploadAttachmentForAgent,
      onUploadFailed: (error: AttachmentUploadError) => {
        console.warn(`[KylinGuard] attachment rejected: ${error.reason}: ${error.message}`);
      },
    }),
    [preferences.attachmentsEnabled],
  );

  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
      document.body.style.userSelect = "";
    };
  }, [isOpen]);

  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (event.key === "Escape") {
      setIsOpen(false);
    }
  }, []);

  const toggle = useCallback(() => {
    setIsOpen((value) => !value);
    setActiveTab("chat");
  }, []);

  const close = useCallback(() => setIsOpen(false), []);

  const markChatActive = useCallback(() => {
    setHasChatActivity(true);
  }, []);

  const markThinking = useCallback(() => {
    setHasChatActivity(true);
    setIsThinking(true);
  }, []);

  const handleChatKeyDown = useCallback(
    (event: React.KeyboardEvent) => {
      markChatActive();
      if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
        markThinking();
      }
    },
    [markChatActive, markThinking],
  );

  const handleChatClick = useCallback(
    (event: React.MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      if (target.closest('[data-testid="copilot-send-button"]')) {
        markThinking();
        return;
      }
      if (target.closest('[data-testid="copilot-chat-textarea"], .copilotKitInput')) {
        markChatActive();
      }
    },
    [markChatActive, markThinking],
  );

  const handleDragStart = useCallback(
    (event: React.MouseEvent) => {
      if (!position) return;
      event.preventDefault();
      setIsDragging(true);
      document.body.style.userSelect = "none";
      dragRef.current = {
        startX: event.clientX,
        startY: event.clientY,
        startLeft: position.left,
        startTop: position.top,
      };

      const onMove = (moveEvent: MouseEvent) => {
        if (!dragRef.current) return;
        const next = {
          left: dragRef.current.startLeft + moveEvent.clientX - dragRef.current.startX,
          top: dragRef.current.startTop + moveEvent.clientY - dragRef.current.startY,
        };
        setPosition(clampPosition(next, size));
      };

      const onUp = () => {
        dragRef.current = null;
        setIsDragging(false);
        document.body.style.userSelect = "";
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
      };

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onUp);
    },
    [position, size],
  );

  const handleResizeStart = useCallback(
    (event: React.MouseEvent) => {
      event.preventDefault();
      document.body.style.userSelect = "none";
      resizeRef.current = { startX: event.clientX, startY: event.clientY, startW: size.width, startH: size.height };
      let latestSize = size;

      const onMove = (moveEvent: MouseEvent) => {
        if (!resizeRef.current) return;
        const dx = moveEvent.clientX - resizeRef.current.startX;
        const dy = moveEvent.clientY - resizeRef.current.startY;
        const nextSize = clampSize({
          width: resizeRef.current.startW + dx,
          height: resizeRef.current.startH + dy,
        });
        latestSize = nextSize;
        setSize(nextSize);
        setPosition((current) => (current ? clampPosition(current, nextSize) : centerPosition(nextSize)));
      };

      const onUp = () => {
        resizeRef.current = null;
        document.body.style.userSelect = "";
        updatePreferences({ chatWidth: Math.round(latestSize.width), chatHeight: Math.round(latestSize.height) });
        document.removeEventListener("mousemove", onMove);
        document.removeEventListener("mouseup", onUp);
      };

      document.addEventListener("mousemove", onMove);
      document.addEventListener("mouseup", onUp);
    },
    [size, updatePreferences],
  );

  const drawerPosition = position || { left: 0, top: 0 };

  return (
    <>
      <button
        type="button"
        className="app-drawer-toggle"
        onClick={toggle}
        aria-label={isOpen ? "关闭聊天" : "打开聊天"}
        title={isOpen ? "关闭聊天" : "打开聊天"}
      >
        <MessageSquare size={22} />
      </button>

      {isOpen && (
        <>
          <div className="app-drawer-overlay" onClick={close} role="presentation" />

          <div
            ref={modalRef}
            className={[
              "app-drawer-modal",
              "liquid-chat-drawer",
              `glass-${preferences.glassIntensity}`,
              preferences.compactMode ? "is-compact" : "",
              preferences.reduceMotion ? "reduce-motion" : "",
              isDragging ? "is-dragging" : "",
            ]
              .filter(Boolean)
              .join(" ")}
            style={{
              width: size.width,
              height: size.height,
              left: drawerPosition.left,
              top: drawerPosition.top,
              transform: "none",
            }}
            onKeyDown={handleKeyDown}
            role="dialog"
            aria-modal="true"
            aria-label="聊天与设置"
          >
            <div className="app-drawer-tabs">
              <button
                type="button"
                className={activeTab === "chat" ? "active" : ""}
                onClick={() => setActiveTab("chat")}
              >
                <MessageSquare size={16} />
                <span>聊天</span>
              </button>
              <button
                type="button"
                className={activeTab === "settings" ? "active" : ""}
                onClick={() => setActiveTab("settings")}
              >
                <Settings size={16} />
                <span>设置</span>
              </button>
              <div className="app-drawer-drag-handle" onMouseDown={handleDragStart} title="拖动调整位置">
                <GripHorizontal size={16} />
                <span>{isThinking ? "正在思考中" : "拖动调整位置"}</span>
              </div>
              <button type="button" className="app-drawer-close" onClick={close} aria-label="关闭">
                <X size={18} />
              </button>
            </div>

            <div className="app-drawer-content">
              {activeTab === "chat" ? (
                <div className="kg-chat-workspace">
                  <div
                    className="kg-copilot-glass-frame"
                    onInputCapture={markChatActive}
                    onKeyDownCapture={handleChatKeyDown}
                    onClickCapture={handleChatClick}
                  >
                    {!hasChatActivity && !isThinking && !currentRun ? (
                      <div className="kg-chat-empty-state" aria-hidden="true">
                        <h2>有什么可以帮忙的？</h2>
                        <p>输入消息即可。</p>
                      </div>
                    ) : null}
                    <CopilotChat
                      agentId="default"
                      labels={{
                        modalHeaderTitle: "麒盾",
                        chatInputPlaceholder: "输入消息...",
                        chatInputToolbarAddButtonLabel: "添加附件",
                        welcomeMessageText: "",
                        chatDisclaimerText: "",
                      }}
                      welcomeScreen={false}
                      input={{
                        addMenuButton: DirectAttachmentButton,
                      }}
                      attachments={attachments}
                      onSubmitMessage={markThinking}
                    >
                      {({ scrollView, input }) => (
                        <div className="kg-copilot-chat-layout">
                          {scrollView}
                          <ThinkingTracePanel run={currentRun} thinking={isThinking} />
                          {input}
                        </div>
                      )}
                    </CopilotChat>
                  </div>
                </div>
              ) : (
                <SettingsPanel
                  preferences={preferences}
                  updatePreferences={updatePreferences}
                  resetPreferences={resetPreferences}
                  resetPanel={() => {
                    const nextSize = clampSize({ width: DEFAULT_W, height: DEFAULT_H });
                    setSize(nextSize);
                    setPosition(defaultPosition("center", nextSize));
                  }}
                />
              )}
            </div>

            <div className="app-drawer-resize-handle" onMouseDown={handleResizeStart} title="拖动调整大小" />
          </div>
        </>
      )}
    </>
  );
});

function ThinkingTracePanel({
  run,
  thinking,
}: {
  run?: AgentRun | null;
  thinking: boolean;
}) {
  const steps = run?.agent_steps || [];
  const traces = run?.tool_trace || [];
  const hasProcess = thinking || steps.length > 0 || traces.length > 0;
  const meta = processMeta(steps, traces);

  if (!hasProcess) {
    return null;
  }

  return (
    <details className="kg-thinking-panel kg-message-trace-panel" open={thinking}>
      <summary>
        <span className={`kg-thinking-dot${thinking ? " active" : ""}`}>
          {thinking ? <Loader2 size={15} /> : <CheckCircle2 size={15} />}
        </span>
        <span>{thinking ? "正在思考中" : "思考与工具调用过程"}</span>
        <small>
          {steps.length || traces.length
            ? `${steps.length} 个步骤 / ${traces.length} 条证据`
            : "理解请求并规划下一步"}
        </small>
        <ChevronDown size={16} className="kg-thinking-chevron" />
      </summary>

      <div className="kg-thinking-body">
        {meta ? <div className="kg-process-meta">{meta}</div> : null}

        {thinking ? (
          <div className="kg-thinking-live">
            <span data-state="done">理解请求</span>
            <span data-state="active">规划下一步</span>
            <span data-state="active">等待工具与策略结果</span>
            <span data-state="pending">生成回答</span>
            <span>理解你的输入</span>
            <span>判断是否需要调用工具</span>
            <span>准备安全策略检查</span>
          </div>
        ) : null}

        {steps.length > 0 ? (
          <div className="kg-thinking-section">
            <strong>执行步骤</strong>
            <div className="kg-process-list">
              {steps.map((step, index) => (
                <ProcessStep key={`${step.step_index ?? index}-${step.tool_name ?? "step"}`} step={step} index={index} />
              ))}
            </div>
          </div>
        ) : null}

        {traces.length > 0 ? (
          <div className="kg-thinking-section">
            <strong>工具证据</strong>
            <div className="kg-process-list compact">
              {traces.slice(0, 5).map((trace, index) => (
                <TraceLine key={`${trace.step_id ?? index}-${trace.tool_name ?? "trace"}`} trace={trace} index={index} />
              ))}
              {traces.length > 5 ? <span className="kg-process-more">还有 {traces.length - 5} 条证据已同步到看板</span> : null}
            </div>
          </div>
        ) : null}
      </div>
    </details>
  );
}

function ProcessStep({ step, index }: { step: AgentStep; index: number }) {
  return (
    <div className="kg-process-step">
      <span className="kg-process-index">{step.step_index ?? index + 1}</span>
      <div>
        <strong>{step.tool_name || step.action_type || "步骤"}</strong>
        <p>{step.user_visible_summary || step.reason || observationText(step) || "已完成一步受控处理"}</p>
        <small>
          {[step.policy_decision, step.operation_type, step.resource_type, step.boundary_level].filter(Boolean).join(" / ")}
        </small>
      </div>
    </div>
  );
}

function TraceLine({ trace, index }: { trace: ToolTrace; index: number }) {
  return (
    <div className="kg-process-step">
      <span className="kg-process-index">{index + 1}</span>
      <div>
        <strong>
          <Wrench size={13} />
          {trace.tool_name || "工具调用"}
        </strong>
        <p>{trace.output_summary || trace.risk_hint || trace.status || "工具返回了审计证据"}</p>
      </div>
    </div>
  );
}

function processMeta(steps: AgentStep[], traces: ToolTrace[]) {
  const step = steps[0];
  const trace = traces[0];
  return [
    step?.policy_decision || trace?.allowed_by_policy,
    step?.operation_type || trace?.operation_type,
    step?.tool_name || trace?.tool_name,
    step?.boundary_level || trace?.boundary_level,
  ]
    .filter(Boolean)
    .join(" / ");
}

function SettingsPanel({
  preferences,
  updatePreferences,
  resetPreferences,
  resetPanel,
}: {
  preferences: ConsolePreferences;
  updatePreferences: (patch: Partial<ConsolePreferences>) => void;
  resetPreferences: () => void;
  resetPanel: () => void;
}) {
  return (
    <div className="app-drawer-settings">
      <div className="app-drawer-settings-header">
        <Settings size={18} />
        <h3>界面设置</h3>
      </div>
      <p className="app-drawer-settings-desc">设置会立即生效，并保存在当前浏览器。</p>

      <section className="app-settings-group">
        <h4>显示</h4>
        <div className="app-drawer-settings-list">
          <SettingRow label="外观" description="跟随系统，或固定为浅色/深色。">
            <div className="segmented-control" role="group" aria-label="外观主题">
              {(["system", "light", "dark"] as const).map((theme) => (
                <button
                  key={theme}
                  type="button"
                  aria-pressed={preferences.theme === theme}
                  className={preferences.theme === theme ? "active" : ""}
                  onClick={() => updatePreferences({ theme })}
                >
                  {theme === "system" ? "跟随系统" : theme === "light" ? "浅色" : "深色"}
                </button>
              ))}
            </div>
          </SettingRow>

          <SettingRow label="玻璃强度" description="调整抽屉和输入框的透明、模糊与层次感。">
            <div className="segmented-control" role="group" aria-label="玻璃强度">
              {(["soft", "balanced", "strong"] as const).map((glassIntensity) => (
                <button
                  key={glassIntensity}
                  type="button"
                  aria-pressed={preferences.glassIntensity === glassIntensity}
                  className={preferences.glassIntensity === glassIntensity ? "active" : ""}
                  onClick={() => updatePreferences({ glassIntensity })}
                >
                  {glassIntensity === "soft" ? "轻柔" : glassIntensity === "strong" ? "清晰" : "均衡"}
                </button>
              ))}
            </div>
          </SettingRow>

          <SettingRow label="紧凑显示" description="收紧抽屉内间距，适合小屏或录屏时展示更多内容。">
            <SwitchControl
              checked={preferences.compactMode}
              onChange={(compactMode) => updatePreferences({ compactMode })}
            />
          </SettingRow>

          <SettingRow label="减少动效" description="关闭非必要动画，保留状态变化但减少视觉干扰。">
            <SwitchControl
              checked={preferences.reduceMotion}
              onChange={(reduceMotion) => updatePreferences({ reduceMotion })}
            />
          </SettingRow>
        </div>
      </section>

      <section className="app-settings-group">
        <h4>抽屉</h4>
        <div className="app-drawer-settings-list">
          <SettingRow label="打开位置" description="决定下次打开或切换设置时抽屉停靠的位置。">
            <div className="segmented-control" role="group" aria-label="抽屉位置">
              {(["left", "center", "right"] as const).map((chatPosition) => (
                <button
                  key={chatPosition}
                  type="button"
                  aria-pressed={preferences.chatPosition === chatPosition}
                  className={preferences.chatPosition === chatPosition ? "active" : ""}
                  onClick={() => updatePreferences({ chatPosition })}
                >
                  {chatPosition === "left" ? "左侧" : chatPosition === "right" ? "右侧" : "居中"}
                </button>
              ))}
            </div>
          </SettingRow>

          <SettingRow label="面板宽度" description="调整聊天抽屉宽度，也会作为下次打开的默认宽度。">
            <RangeControl
              label="窄"
              min={560}
              max={1100}
              step={20}
              value={preferences.chatWidth}
              unit="px"
              onChange={(chatWidth) => updatePreferences({ chatWidth })}
            />
          </SettingRow>

          <SettingRow label="面板高度" description="调整聊天抽屉高度，也会作为下次打开的默认高度。">
            <RangeControl
              label="矮"
              min={430}
              max={820}
              step={10}
              value={preferences.chatHeight}
              unit="px"
              onChange={(chatHeight) => updatePreferences({ chatHeight })}
            />
          </SettingRow>

          <SettingRow label="默认展开" description="页面加载后自动打开聊天抽屉。">
            <SwitchControl
              checked={preferences.chatDefaultOpen}
              onChange={(chatDefaultOpen) => updatePreferences({ chatDefaultOpen })}
            />
          </SettingRow>
        </div>
      </section>

      <section className="app-settings-group">
        <h4>交互</h4>
        <div className="app-drawer-settings-list">
          <SettingRow label="过程面板" description="任务完成后默认展开思考、步骤和工具调用摘要。">
            <SwitchControl
              checked={preferences.processPanelDefaultOpen}
              onChange={(processPanelDefaultOpen) => updatePreferences({ processPanelDefaultOpen })}
            />
          </SettingRow>

          <SettingRow label="附件上传" description="控制聊天输入栏中的附件按钮，可附加日志、配置或文本。">
            <SwitchControl
              checked={preferences.attachmentsEnabled}
              onChange={(attachmentsEnabled) => updatePreferences({ attachmentsEnabled })}
            />
          </SettingRow>
        </div>
      </section>

      <div className="app-drawer-settings-footer">
        <button
          className="secondary-action"
          type="button"
          onClick={() => {
            resetPreferences();
            resetPanel();
          }}
        >
          恢复默认设置
        </button>
      </div>
    </div>
  );
}

function SettingRow({ label, description, children }: { label: string; description: string; children: React.ReactNode }) {
  return (
    <div className="setting-row">
      <div>
        <strong>{label}</strong>
        <span>{description}</span>
      </div>
      {children}
    </div>
  );
}

function RangeControl({
  min,
  max,
  step,
  value,
  unit,
  label,
  onChange,
}: {
  min: number;
  max: number;
  step: number;
  value: number;
  unit: string;
  label: string;
  onChange: (value: number) => void;
}) {
  return (
    <label className="range-control">
      <span>{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
      <strong>
        {value}
        {unit}
      </strong>
    </label>
  );
}

function SwitchControl({ checked, onChange }: { checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="switch-control">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      <span aria-hidden="true" />
      <strong>{checked ? "已开启" : "已关闭"}</strong>
    </label>
  );
}

function DirectAttachmentButton({
  onAddFile,
  disabled,
  className,
  toolsMenu,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { onAddFile?: () => void; toolsMenu?: unknown }) {
  void toolsMenu;
  return (
    <button
      {...props}
      type="button"
      className={["kg-chat-attachment-button", className].filter(Boolean).join(" ")}
      disabled={disabled || !onAddFile}
      aria-label="添加附件"
      title="添加附件"
      onClick={(event) => {
        props.onClick?.(event);
        if (!event.defaultPrevented) {
          onAddFile?.();
        }
      }}
    >
      <Plus size={20} aria-hidden="true" />
    </button>
  );
}

async function uploadAttachmentForAgent(file: File): Promise<AttachmentUploadResult> {
  const mimeType = file.type || mimeTypeFromName(file.name);
  const preview = await textPreview(file, mimeType);
  return {
    type: "data",
    value: await fileToBase64(file),
    mimeType,
    metadata: {
      filename: file.name,
      size: file.size,
      text_preview: preview,
      preview_truncated: preview.length >= 12000,
    },
  };
}

async function textPreview(file: File, mimeType: string) {
  if (!isTextLikeAttachment(file.name, mimeType)) {
    return "";
  }
  const text = await file.text();
  return text.slice(0, 12000);
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const value = typeof reader.result === "string" ? reader.result : "";
      const base64 = value.split(",")[1] || "";
      if (!base64) {
        reject(new Error("Failed to read attachment"));
        return;
      }
      resolve(base64);
    };
    reader.onerror = () => reject(reader.error || new Error("Failed to read attachment"));
    reader.readAsDataURL(file);
  });
}

function mimeTypeFromName(name: string) {
  const lower = name.toLowerCase();
  if (lower.endsWith(".json")) return "application/json";
  if (lower.endsWith(".md")) return "text/markdown";
  if (lower.endsWith(".log") || lower.endsWith(".txt") || lower.endsWith(".conf") || lower.endsWith(".ini")) {
    return "text/plain";
  }
  if (lower.endsWith(".yaml") || lower.endsWith(".yml")) return "text/yaml";
  if (lower.endsWith(".pdf")) return "application/pdf";
  return "application/octet-stream";
}

function isTextLikeAttachment(name: string, mimeType: string) {
  const lower = name.toLowerCase();
  return (
    mimeType.startsWith("text/") ||
    mimeType === "application/json" ||
    lower.endsWith(".log") ||
    lower.endsWith(".conf") ||
    lower.endsWith(".ini") ||
    lower.endsWith(".yaml") ||
    lower.endsWith(".yml")
  );
}

function observationText(step: AgentStep) {
  const observation = step.observation;
  if (!observation) return "";
  if (typeof observation === "string") return observation;
  if (typeof observation.summary === "string") return observation.summary;
  if (typeof observation.result === "string") return observation.result;
  if (typeof observation.status === "string") return observation.status;
  return "";
}

function centerPosition(size: DrawerSize): DrawerPosition {
  if (typeof window === "undefined") {
    return { left: EDGE_GAP, top: EDGE_GAP };
  }
  return clampPosition(
    {
      left: Math.round((window.innerWidth - size.width) / 2),
      top: Math.round((window.innerHeight - size.height) / 2),
    },
    size,
  );
}

function defaultPosition(position: ConsolePreferences["chatPosition"], size: DrawerSize): DrawerPosition {
  if (typeof window === "undefined") {
    return { left: EDGE_GAP, top: EDGE_GAP };
  }
  const top = Math.round((window.innerHeight - size.height) / 2);
  if (position === "left") {
    return clampPosition({ left: EDGE_GAP, top }, size);
  }
  if (position === "right") {
    return clampPosition({ left: window.innerWidth - size.width - EDGE_GAP, top }, size);
  }
  return centerPosition(size);
}

function clampSize(size: DrawerSize): DrawerSize {
  if (typeof window === "undefined") return size;
  return {
    width: Math.min(window.innerWidth * MAX_W_RATIO, Math.max(MIN_W, size.width)),
    height: Math.min(window.innerHeight * MAX_H_RATIO, Math.max(MIN_H, size.height)),
  };
}

function clampPosition(position: DrawerPosition, size: DrawerSize): DrawerPosition {
  if (typeof window === "undefined") return position;
  const maxLeft = Math.max(EDGE_GAP, window.innerWidth - size.width - EDGE_GAP);
  const maxTop = Math.max(EDGE_GAP, window.innerHeight - size.height - EDGE_GAP);
  return {
    left: Math.min(maxLeft, Math.max(EDGE_GAP, position.left)),
    top: Math.min(maxTop, Math.max(EDGE_GAP, position.top)),
  };
}
