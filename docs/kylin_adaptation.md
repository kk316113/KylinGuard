# Kylin Adaptation

## 目标环境

- 银河麒麟高级服务器版 V11
- Linux systemd 环境
- x86_64、aarch64、LoongArch 等架构

## Stage 0 适配原则

- Go 代码使用标准库优先。
- Python stub 只使用 FastAPI、uvicorn、pydantic。
- 部署脚本使用 POSIX sh，避免 Windows 路径。
- Agent 默认调用远程模型 API，不在本机或虚拟机内运行大模型。
- 运维工具默认走只读或低风险路径。

## 后续验证项

- 在目标 Kylin V11 系统上运行 `deploy/kylin/check_env.sh`。
- 验证 Go 编译器版本和 `GOARCH=loong64` 支持情况。
- 验证 systemd、日志路径、权限模型和服务管理策略。
- 验证 Python venv 和 pip 源可用性。
