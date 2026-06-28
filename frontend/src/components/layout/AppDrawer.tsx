"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import { CopilotChat, type AttachmentsConfig } from "@copilotkit/react-core/v2";
import { MessageSquare, Plus, Settings, X } from "lucide-react";
import { useConsolePreferences, type ConsolePreferences } from "@/hooks/useConsolePreferences";

// ── Types ───────────────────────────────────────────────────────

type TabKey = "chat" | "settings";

export type AppDrawerHandle = {
  openToSettings: () => void;
};

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

// ── Component ───────────────────────────────────────────────────

// eslint-disable-next-line @typescript-eslint/no-empty-interface
interface AppDrawerProps {}
export const AppDrawer = forwardRef<AppDrawerHandle, AppDrawerProps>(function AppDrawer(_props, ref) {
  const { preferences, updatePreferences, resetPreferences } = useConsolePreferences();
  const [isOpen, setIsOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<TabKey>("chat");
  const panelRef = useRef<HTMLDivElement>(null);

  useImperativeHandle(ref, () => ({
    openToSettings() {
      setActiveTab("settings");
      setIsOpen(true);
    },
  }));

  // Auto-open if preference says so (mount only)
  useEffect(() => {
    if (preferences.chatDefaultOpen) {
      setIsOpen(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const attachments = useMemo<AttachmentsConfig>(
    () => ({
      enabled: true,
      accept: attachmentAccept,
      maxSize: attachmentMaxSize,
      onUpload: uploadAttachmentForAgent,
      onUploadFailed: (error: AttachmentUploadError) => {
        console.warn(`[KylinGuard] attachment rejected: ${error.reason}: ${error.message}`);
      },
    }),
    [],
  );

  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (event.key === "Escape") {
      setIsOpen(false);
    }
  }, []);

  // Prevent body scroll when open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen]);

  const isLeft = preferences.chatPosition === "left";
  const drawerWidth = preferences.chatWidth;

  const toggle = useCallback(() => setIsOpen((v) => !v), []);
  const close = useCallback(() => setIsOpen(false), []);

  return (
    <>
      {/* Toggle button at bottom corner */}
      <button
        type="button"
        className={`app-drawer-toggle ${isLeft ? "left" : "right"}`}
        onClick={toggle}
        aria-label={isOpen ? "关闭面板" : "打开面板"}
        title={isOpen ? "关闭面板" : "打开面板"}
      >
        <MessageSquare size={22} />
      </button>

      {/* Overlay */}
      {isOpen && <div className="app-drawer-overlay" onClick={close} role="presentation" />}

      {/* Drawer panel */}
      <div
        ref={panelRef}
        className={`app-drawer-panel ${isOpen ? "open" : ""} ${isLeft ? "left" : "right"}`}
        style={{ width: drawerWidth }}
        onKeyDown={handleKeyDown}
        role="dialog"
        aria-modal="true"
        aria-label="应用面板"
      >
        {/* Tab bar */}
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
          <button type="button" className="app-drawer-close" onClick={close} aria-label="关闭">
            <X size={18} />
          </button>
        </div>

        {/* Content area */}
        <div className="app-drawer-content">
          {activeTab === "chat" ? (
            <CopilotChat
              agentId="default"
              labels={{
                modalHeaderTitle: "麒盾",
                chatInputPlaceholder: "直接输入你的问题...",
                chatInputToolbarAddButtonLabel: "添加附件",
                welcomeMessageText:
                  "你好，我是麒盾。你可以直接描述问题，也可以附上日志、配置片段或截图；我会在安全策略约束下处理。",
                chatDisclaimerText: "智能体只执行受控只读工具；重要结论请结合审计证据复核。",
              }}
              input={{
                addMenuButton: DirectAttachmentButton,
              }}
              attachments={attachments}
            />
          ) : (
            <SettingsPanel
              preferences={preferences}
              updatePreferences={updatePreferences}
              resetPreferences={resetPreferences}
            />
          )}
        </div>
      </div>
    </>
  );
});

// ── Settings Panel ──────────────────────────────────────────────

function SettingsPanel({
  preferences,
  updatePreferences,
  resetPreferences,
}: {
  preferences: ConsolePreferences;
  updatePreferences: (patch: Partial<ConsolePreferences>) => void;
  resetPreferences: () => void;
}) {
  return (
    <div className="app-drawer-settings">
      <div className="app-drawer-settings-header">
        <Settings size={18} />
        <h3>界面设置</h3>
      </div>
      <p className="app-drawer-settings-desc">更改会立即生效，并保存在当前浏览器。</p>

      <div className="app-drawer-settings-list">
        <SettingRow label="外观" description="跟随系统或指定浅色、深色主题。">
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

        <SettingRow label="面板位置" description="面板从左侧或右侧滑出。">
          <div className="segmented-control" role="group" aria-label="面板位置">
            {(["left", "right"] as const).map((chatPosition) => (
              <button
                key={chatPosition}
                type="button"
                aria-pressed={preferences.chatPosition === chatPosition}
                className={preferences.chatPosition === chatPosition ? "active" : ""}
                onClick={() => updatePreferences({ chatPosition })}
              >
                {chatPosition === "left" ? "左侧" : "右侧"}
              </button>
            ))}
          </div>
        </SettingRow>

        <SettingRow label="面板宽度" description={`${preferences.chatWidth} 像素`}>
          <div className="range-control">
            <span>360</span>
            <input
              type="range"
              min={360}
              max={640}
              step={20}
              value={preferences.chatWidth}
              onChange={(event) => updatePreferences({ chatWidth: Number(event.target.value) })}
              aria-label="面板宽度"
            />
            <span>640</span>
          </div>
        </SettingRow>

        <SettingRow label="默认展开" description="页面加载后直接打开面板。">
          <label className="switch-control">
            <input
              type="checkbox"
              checked={preferences.chatDefaultOpen}
              onChange={(event) => updatePreferences({ chatDefaultOpen: event.target.checked })}
            />
            <span aria-hidden="true" />
            <strong>{preferences.chatDefaultOpen ? "已开启" : "已关闭"}</strong>
          </label>
        </SettingRow>
      </div>

      <div className="app-drawer-settings-footer">
        <button className="secondary-action" type="button" onClick={resetPreferences}>
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

// ── DirectAttachmentButton (consumed by CopilotChat.input) ──────

function DirectAttachmentButton({
  onAddFile,
  disabled,
  className,
  toolsMenu,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { onAddFile?: () => void; toolsMenu?: unknown }) {
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

// ── Attachment upload helpers ───────────────────────────────────

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
