export type ServiceStatus = {
  status: string;
  port?: number;
  url?: string;
};

export type RuntimeStatus = {
  ok: boolean;
  runtime: {
    agent_mode: string;
    current_mode: string;
    llm_enabled: boolean;
    remote_llm_used: boolean;
    chat_model: string;
    provider: string;
    endpoint_kind: string;
    model: string;
  };
  services: {
    go_agent: ServiceStatus;
    audit_core: ServiceStatus;
    frontend: ServiceStatus;
  };
  security_layers: Record<string, string>;
  secret_safety: {
    api_key_present: boolean;
    api_key_display: string;
  };
  updated_at: string;
};

export type CapabilityTool = {
  tool_name: string;
  display_name: string;
  description: string;
  operation_type: string;
  resource_type: string;
  boundary_level: string;
  requires_privilege: boolean;
  read_only: boolean;
  policy_controlled: boolean;
  traceshield_mapped: boolean;
  enabled: boolean;
};

export type CapabilitiesResponse = {
  available_tools: CapabilityTool[];
  tool_policy: {
    enabled: boolean;
    default_mode: string;
    dangerous_actions_blocked: boolean;
    unknown_tools_default_denied: boolean;
    raw_shell_execution: string;
  };
  agent_loop: {
    next_action_schema: string[];
    max_steps: number;
  };
};

export type AcceptanceSummary = {
  stages: Array<{
    name: string;
    title: string;
    status: string;
    evidence?: Record<string, unknown>;
  }>;
  commands: string[];
  notes: string[];
};
