from dataclasses import dataclass
from pathlib import Path
import os


def _default_traceshield_path() -> str:
    # app/config.py -> audit-core-py/app -> audit-core-py -> KylinGuard-Agent -> 2026
    source_root = Path(__file__).resolve().parents[3]
    return str(source_root / "TraceShield-Core")


@dataclass(frozen=True)
class AuditCoreConfig:
    traceshield_core_path: str
    service_host: str
    service_port: int


def load_config() -> AuditCoreConfig:
    return AuditCoreConfig(
        traceshield_core_path=os.getenv("TRACESHIELD_CORE_PATH", _default_traceshield_path()),
        service_host=os.getenv("AUDIT_CORE_HOST", "127.0.0.1"),
        service_port=int(os.getenv("AUDIT_CORE_PORT", "8001")),
    )
