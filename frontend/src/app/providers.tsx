"use client";

import { CopilotKit } from "@copilotkit/react-core/v2";

export function Providers({ children }: { children: React.ReactNode }) {
  const runtimeUrl = process.env.NEXT_PUBLIC_COPILOTKIT_RUNTIME_URL || "/api/copilotkit";

  return (
    <CopilotKit
      runtimeUrl={runtimeUrl}
      agent="default"
      useSingleEndpoint={false}
      showDevConsole={false}
      enableInspector={false}
      credentials="include"
      onError={({ type, error }) => {
        if (type === "error") {
          console.error(`[CopilotKit] ${error instanceof Error ? error.message : "request failed"}`);
        }
      }}
    >
      {children}
    </CopilotKit>
  );
}
