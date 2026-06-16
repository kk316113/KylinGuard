import logging
import re
import sys
from pathlib import Path
from typing import Any, Dict, Optional, Tuple

from .audit_stub import audit_trace as fallback_audit_trace
from .config import AuditCoreConfig, load_config
from .schemas import (
    AuditTraceRequest,
    AuditTraceResponse,
    AuditViolation,
    EvidenceItem,
    RiskGraph,
    ToolTraceStep,
)


LOGGER = logging.getLogger(__name__)


TOOL_NAME_ALIASES = {
    "os_info": "get_user_info",
    "service_status": "summarize_text",
    "port_checker": "web_search",
    "log_reader": "read_file",
    "read_recent_logs": "read_file",
    "ssh_login_analyzer": "summarize_text",
    "safe_shell": "shell_exec",
    # Stage 10 OS deep sensing tools — mapped to safe read/inspect actions
    "process_inspector": "list_files",
    "network_connection_inspector": "web_search",
    "journalctl_reader": "read_file",
    "resource_usage_checker": "read_user_data",
    "disk_memory_checker": "read_user_data",
}


class TraceShieldAdapter:
    def __init__(self, config: Optional[AuditCoreConfig] = None):
        self.config = config or load_config()
        self._available = False
        self._unavailable_reason = ""
        self._TraceShieldEvaluator = None
        self._IntentFrame = None
        self._ToolEvent = None
        self._evaluator = None
        self._load_core()

    def is_available(self) -> bool:
        return self._available

    @property
    def unavailable_reason(self) -> str:
        return self._unavailable_reason

    def audit_trace(self, request: AuditTraceRequest) -> AuditTraceResponse:
        if not self.is_available():
            return fallback_audit_trace(request, self._unavailable_reason)

        try:
            intent = self._build_intent(request)
            trace, step_lookup = self._build_trace(request)
            sample_id = request.task_id or "kylin-guard-agent-task"
            result = self._evaluator.evaluate_tool_events(sample_id, intent, trace)
            return self._convert_result(result, request, step_lookup)
        except Exception as exc:
            LOGGER.exception("TraceShield audit failed")
            return fallback_audit_trace(request, f"TraceShield runtime failed: {exc}")

    def _load_core(self) -> None:
        core_path = Path(self.config.traceshield_core_path)
        if not core_path.exists():
            self._unavailable_reason = f"TRACESHIELD_CORE_PATH does not exist: {core_path}"
            LOGGER.warning(self._unavailable_reason)
            return

        core_path_str = str(core_path)
        if core_path_str not in sys.path:
            sys.path.insert(0, core_path_str)

        try:
            from traceshield_experiment_core import IntentFrame, ToolEvent, TraceShieldEvaluator

            self._TraceShieldEvaluator = TraceShieldEvaluator
            self._IntentFrame = IntentFrame
            self._ToolEvent = ToolEvent
            self._evaluator = TraceShieldEvaluator()
            self._available = True
            self._unavailable_reason = ""
        except Exception as exc:
            LOGGER.exception("TraceShield core import failed")
            self._available = False
            self._unavailable_reason = f"TraceShield import failed: {exc}"

    def _build_intent(self, request: AuditTraceRequest):
        metadata = request.metadata or {}
        return self._IntentFrame(
            task_goal=request.user_goal or "audit KylinGuard tool trace",
            allowed_actions=list(metadata.get("allowed_actions") or self._default_allowed_actions(request)),
            allowed_resources=list(metadata.get("allowed_resources") or []),
            forbidden_actions=list(metadata.get("forbidden_actions") or self._default_forbidden_actions()),
            forbidden_resources=list(metadata.get("forbidden_resources") or []),
            sensitive_resources=list(metadata.get("sensitive_resources") or self._default_sensitive_resources()),
            constraints={
                "user_query": request.user_goal,
                "source": request.source,
                "metadata": metadata,
            },
        )

    def _build_trace(self, request: AuditTraceRequest):
        trace = []
        step_lookup: Dict[int, ToolTraceStep] = {}
        for index, step in enumerate(request.steps, start=1):
            numeric_step_id = self._numeric_step_id(step.step_id, index)
            tool_name, args = self._normalize_tool_call(step)
            trace.append(
                self._ToolEvent(
                    step_id=numeric_step_id,
                    tool_name=tool_name,
                    args=args,
                    raw_call=step.model_dump_json(),
                    observation=step.output_summary,
                )
            )
            step_lookup[numeric_step_id] = step
        return trace, step_lookup

    def _normalize_tool_call(self, step: ToolTraceStep) -> Tuple[str, Dict[str, Any]]:
        original_tool_name = step.tool_name
        tool_name = self._mapped_tool_name(step)
        args = dict(step.input or {})
        args.setdefault("status", step.status)
        args["_kylin_semantics"] = self._semantic_metadata(step)
        if step.risk_hint:
            args.setdefault("risk_hint", step.risk_hint)
        if step.output_summary:
            args.setdefault("observation", step.output_summary)

        if tool_name == "read_file":
            args.setdefault("path", self._resource_from_step(step) or "unknown-log-resource")
        elif tool_name == "shell_exec":
            args.setdefault("command", self._command_from_step(step))
        elif tool_name == "web_search":
            args.setdefault("query", self._resource_from_step(step) or step.output_summary or original_tool_name)

        args["_kylin_original_tool_name"] = original_tool_name
        return tool_name, args

    def _mapped_tool_name(self, step: ToolTraceStep) -> str:
        if step.tool_name == "safe_shell" and step.allowed_by_policy is True:
            return "summarize_text"
        if step.tool_name == "safe_shell" and step.allowed_by_policy is False:
            return "shell_exec"
        return TOOL_NAME_ALIASES.get(step.tool_name, step.tool_name)

    def _convert_result(self, result: Any, request: AuditTraceRequest, step_lookup: Dict[int, ToolTraceStep]) -> AuditTraceResponse:
        violations = []
        evidence = []
        for item in getattr(result, "violations", []) or []:
            first_step_id = self._first_evidence_step(item, step_lookup)
            violations.append(
                AuditViolation(
                    type=getattr(item, "violation_type", "traceshield_violation"),
                    severity=self._severity(getattr(item, "source", None)),
                    message=getattr(item, "reason", ""),
                    step_id=first_step_id,
                )
            )
            evidence.extend(self._evidence_items(item, step_lookup))

        decision = getattr(result, "decision", "review")
        if decision == "allow" and self._requires_semantic_review(request):
            decision = "review"
        risk_score = self._risk_score(decision, violations)
        return AuditTraceResponse(
            decision=decision if decision in {"allow", "deny", "review"} else "review",
            risk_score=risk_score,
            violations=violations,
            evidence_chain=evidence,
            risk_graph=self._risk_graph(request),
            method="traceshield",
            message="audit completed by TraceShield adapter",
        )

    def _evidence_items(self, violation: Any, step_lookup: Dict[int, ToolTraceStep]):
        items = []
        evidence_steps = getattr(violation, "evidence_steps", []) or []
        for numeric_step_id in evidence_steps:
            step = step_lookup.get(numeric_step_id)
            items.append(
                EvidenceItem(
                    step_id=step.step_id if step else numeric_step_id,
                    tool_name=step.tool_name if step else None,
                    resource=getattr(violation, "target", None) or (self._resource_from_step(step) if step else None),
                    reason=getattr(violation, "reason", ""),
                )
            )
        return items

    def _risk_graph(self, request: AuditTraceRequest) -> RiskGraph:
        nodes = []
        for step in request.steps:
            nodes.append(
                {
                    "step_id": step.step_id,
                    "tool_name": step.tool_name,
                    "operation_type": step.operation_type,
                    "resource_type": step.resource_type,
                    "resource_path": step.resource_path or self._resource_from_step(step),
                    "boundary_level": step.boundary_level,
                    "risk_hint": step.risk_hint,
                    "status": step.status,
                    "allowed_by_policy": step.allowed_by_policy,
                }
            )
        edges = []
        for previous, current in zip(request.steps, request.steps[1:]):
            edges.append(
                {
                    "from": previous.step_id,
                    "to": current.step_id,
                    "type": "sequence",
                }
            )
        return RiskGraph(
            nodes=nodes,
            edges=edges,
        )

    def _first_evidence_step(self, violation: Any, step_lookup: Dict[int, ToolTraceStep]):
        evidence_steps = getattr(violation, "evidence_steps", []) or []
        if not evidence_steps:
            return None
        step = step_lookup.get(evidence_steps[0])
        return step.step_id if step else evidence_steps[0]

    @staticmethod
    def _numeric_step_id(step_id: Any, fallback: int) -> int:
        if isinstance(step_id, int):
            return step_id
        match = re.search(r"\d+", str(step_id))
        if match:
            return int(match.group(0))
        return fallback

    @staticmethod
    def _resource_from_step(step: Optional[ToolTraceStep]) -> Optional[str]:
        if step is None:
            return None
        if step.resource_path:
            return step.resource_path
        data = step.input or {}
        for key in ("path", "file_path", "filename", "resource", "url", "host", "address", "command"):
            value = data.get(key)
            if value:
                if key == "host" and data.get("port"):
                    return f"{value}:{data.get('port')}"
                return str(value)
        return None

    @staticmethod
    def _command_from_step(step: ToolTraceStep) -> str:
        data = step.input or {}
        command = data.get("command")
        if command:
            return str(command)
        return step.output_summary or step.tool_name

    @staticmethod
    def _semantic_metadata(step: ToolTraceStep) -> Dict[str, Any]:
        return {
            "operation_type": step.operation_type,
            "resource_type": step.resource_type,
            "resource_path": step.resource_path,
            "permission_scope": step.permission_scope,
            "boundary_level": step.boundary_level,
            "tool_semantic": step.tool_semantic,
            "requires_privilege": step.requires_privilege,
            "allowed_by_policy": step.allowed_by_policy,
            "policy_reason": step.policy_reason,
        }

    @staticmethod
    def _requires_semantic_review(request: AuditTraceRequest) -> bool:
        for step in request.steps:
            if step.boundary_level in {"sensitive_system_resource", "privileged"}:
                return True
            if step.requires_privilege:
                return True
        return False

    @staticmethod
    def _default_allowed_actions(request: AuditTraceRequest):
        allowed = {"calendar", "list_files", "read_file", "read_user_data", "summarize_text", "web_click", "web_search"}
        goal = (request.user_goal or "").lower()
        if any(token in goal for token in ("write", "create", "save", "写入", "创建", "保存")):
            allowed.add("write_file")
        return sorted(allowed)

    @staticmethod
    def _default_forbidden_actions():
        return [
            "delete_file",
            "delete_email",
            "external_send",
            "modify_credentials",
            "modify_financial_transaction",
            "network_post",
            "shell_exec",
            "web_submit",
            "write_user_data",
        ]

    @staticmethod
    def _default_sensitive_resources():
        return [
            "/etc/**",
            "/root/**",
            "/var/log/**",
            "*.key",
            "*.pem",
            "id_rsa",
            "authorized_keys",
        ]

    @staticmethod
    def _severity(source: Optional[str]) -> str:
        if source in {"risk_graph", "sensitive_external_chain", "prompt_injection", "resource_boundary", "capability_boundary"}:
            return "high"
        if source:
            return "medium"
        return "low"

    @staticmethod
    def _risk_score(decision: str, violations: list) -> float:
        if decision == "deny":
            return 0.85 if violations else 0.7
        if decision == "review":
            return 0.45
        return 0.1
