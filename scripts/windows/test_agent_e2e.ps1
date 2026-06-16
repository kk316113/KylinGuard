$ErrorActionPreference = "Stop"

$endpoint = $env:KYLIN_GUARD_AGENT_URL
if ([string]::IsNullOrWhiteSpace($endpoint)) {
    $endpoint = "http://127.0.0.1:8080"
}

$tempDir = Join-Path $env:TEMP "kylin-guard-agent-e2e"
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

function ConvertFrom-Utf8Base64 {
    param([Parameter(Mandatory = $true)][string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

function Assert-ContainsAll {
    param(
        [Parameter(Mandatory = $true)][string[]]$Actual,
        [Parameter(Mandatory = $true)][string[]]$Expected,
        [Parameter(Mandatory = $true)][string]$Label
    )

    foreach ($item in $Expected) {
        if ($Actual -notcontains $item) {
            throw "$Label missing expected item: $item; actual=$($Actual -join ',')"
        }
    }
}

function Invoke-JsonGet {
    param([Parameter(Mandatory = $true)][string]$Url)

    $raw = & curl.exe -s -f "$Url"
    if ($LASTEXITCODE -ne 0) {
        throw "curl GET failed: $Url"
    }
    if ([string]::IsNullOrWhiteSpace($raw)) {
        throw "empty response from $Url"
    }
    return $raw | ConvertFrom-Json
}

function Invoke-ToolCall {
    param(
        [Parameter(Mandatory = $true)][string]$ToolName,
        [Parameter(Mandatory = $true)][hashtable]$InputMap,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    $safeName = $ToolName -replace "[^A-Za-z0-9_-]", "_"
    $payloadPath = Join-Path $tempDir "tool_call_$safeName.json"
    $payload = @{
        tool_name = $ToolName
        input = $InputMap
        reason = $Reason
    } | ConvertTo-Json -Compress -Depth 12
    [System.IO.File]::WriteAllText(
        $payloadPath,
        $payload,
        [System.Text.UTF8Encoding]::new($false)
    )

    $raw = & curl.exe -s -f -X POST "$endpoint/api/tools/call" `
        -H "Content-Type: application/json; charset=utf-8" `
        --data-binary "@$payloadPath"
    if ($LASTEXITCODE -ne 0) {
        throw "curl POST failed for tool call: $ToolName"
    }
    if ([string]::IsNullOrWhiteSpace($raw)) {
        throw "empty response for tool call: $ToolName"
    }
    return $raw | ConvertFrom-Json
}

function Assert-ToolsProtocol {
    $toolsResponse = Invoke-JsonGet "$endpoint/api/tools"
    if ($toolsResponse.protocol -ne "mcp-like") {
        throw "unexpected /api/tools protocol: $($toolsResponse.protocol)"
    }
    if ($toolsResponse.version -ne "stage8-v1") {
        throw "unexpected /api/tools version: $($toolsResponse.version)"
    }
    if ([int]$toolsResponse.count -lt 6) {
        throw "expected /api/tools count >= 6, got $($toolsResponse.count)"
    }

    $toolNames = @($toolsResponse.tools | ForEach-Object { $_.name })
    Assert-ContainsAll -Actual $toolNames -Expected @("os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer", "safe_shell") -Label "/api/tools"

    $detail = Invoke-JsonGet "$endpoint/api/tools/ssh_login_analyzer"
    if ($detail.boundary_level -ne "sensitive_system_resource") {
        throw "unexpected ssh_login_analyzer boundary_level: $($detail.boundary_level)"
    }
    if ($detail.permission_scope -ne "ssh_auth_log_analyze") {
        throw "unexpected ssh_login_analyzer permission_scope: $($detail.permission_scope)"
    }
    if ($null -eq $detail.input_schema) {
        throw "expected ssh_login_analyzer input_schema"
    }
    if ($null -eq $detail.output_schema) {
        throw "expected ssh_login_analyzer output_schema"
    }

    $portCall = Invoke-ToolCall -ToolName "port_checker" -InputMap @{ host = "127.0.0.1"; port = 22 } -Reason "Stage 8 E2E direct MCP-like tool call"
    if ($portCall.status -eq "denied") {
        throw "port_checker direct call should not be denied: $($portCall.message)"
    }
    if ($portCall.trace.resource_type -ne "network_port") {
        throw "expected port_checker trace.resource_type=network_port, got $($portCall.trace.resource_type)"
    }
    if ($null -eq $portCall.audit_result) {
        throw "expected port_checker audit_result"
    }

    $unknownCall = Invoke-ToolCall -ToolName "unknown_tool" -InputMap @{} -Reason "must be denied"
    if ($unknownCall.status -ne "denied") {
        throw "unknown_tool should be denied, got $($unknownCall.status)"
    }
    if ($unknownCall.audit_result.method -ne "tool_policy") {
        throw "unknown_tool expected audit_result.method=tool_policy, got $($unknownCall.audit_result.method)"
    }
    if ($unknownCall.audit_result.decision -ne "deny") {
        throw "unknown_tool expected decision=deny, got $($unknownCall.audit_result.decision)"
    }

    $shellCall = Invoke-ToolCall -ToolName "safe_shell" -InputMap @{ command = "rm -rf /" } -Reason "must be denied"
    if ($shellCall.status -ne "denied") {
        throw "safe_shell dangerous command should be denied, got $($shellCall.status)"
    }
    if ($shellCall.audit_result.method -ne "tool_policy") {
        throw "safe_shell expected audit_result.method=tool_policy, got $($shellCall.audit_result.method)"
    }

    [PSCustomObject]@{
        tools_protocol = $toolsResponse.protocol
        tools_version = $toolsResponse.version
        tools_count = $toolsResponse.count
        ssh_login_analyzer_boundary = $detail.boundary_level
        port_checker_status = $portCall.status
        port_checker_trace_resource = $portCall.trace.resource_type
        port_checker_audit_method = $portCall.audit_result.method
        unknown_tool_status = $unknownCall.status
        unknown_tool_method = $unknownCall.audit_result.method
        safe_shell_status = $shellCall.status
        safe_shell_method = $shellCall.audit_result.method
    } | Format-List
}

function Assert-AgentResponse {
    param(
        [Parameter(Mandatory = $true)]$Json,
        [Parameter(Mandatory = $true)][hashtable]$Case
    )

    $trace = @($Json.tool_trace)
    $method = $Json.audit_result.method
    $report = $Json.security_report

    if ($Case.ExpectedDecision -eq "allow_or_review") {
        if (@("allow", "review") -notcontains $Json.decision) {
            throw "$($Case.Name): unexpected decision $($Json.decision)"
        }
    } elseif ($Json.decision -ne $Case.ExpectedDecision) {
        throw "$($Case.Name): unexpected decision $($Json.decision), expected $($Case.ExpectedDecision)"
    }

    if ($method -ne $Case.ExpectedMethod) {
        throw "$($Case.Name): unexpected audit_result.method $method, expected $($Case.ExpectedMethod)"
    }

    if ($null -eq $report) {
        throw "$($Case.Name): expected security_report"
    }
    if ([string]::IsNullOrWhiteSpace($report.title)) {
        throw "$($Case.Name): expected security_report.title"
    }
    if ($report.overall_decision -ne $Json.decision) {
        throw "$($Case.Name): security_report.overall_decision mismatch: $($report.overall_decision) vs $($Json.decision)"
    }
    if ([string]::IsNullOrWhiteSpace($report.risk_level)) {
        throw "$($Case.Name): expected security_report.risk_level"
    }
    if ($report.audit_metadata.report_version -ne "stage6-v1") {
        throw "$($Case.Name): unexpected report_version $($report.audit_metadata.report_version)"
    }
    if ($null -eq $report.recommendations -or @($report.recommendations).Count -eq 0) {
        throw "$($Case.Name): expected security_report.recommendations"
    }

    if ($Case.ExpectEinoSummary) {
        if ($Json.summary -notlike "*Eino graph runtime executed deterministic tool-calling orchestration*") {
            throw "$($Case.Name): summary missing Eino runtime marker: $($Json.summary)"
        }
        if ($Json.summary -like "*stable runtime fallback*") {
            throw "$($Case.Name): summary should not contain stable runtime fallback marker: $($Json.summary)"
        }
        if ($Json.summary -like "*deterministic planner-backed*") {
            throw "$($Case.Name): summary should not contain Stage 9A marker: $($Json.summary)"
        }
    }

    if ($Case.ExpectSSHPlan) {
        if ($null -eq $Json.plan) {
            throw "$($Case.Name): expected plan"
        }
        if ($Json.plan.scenario -ne "ssh_anomaly_check") {
            throw "$($Case.Name): unexpected plan.scenario $($Json.plan.scenario)"
        }
        $stepTools = @($Json.plan.steps | ForEach-Object { $_.tool_name })
        Assert-ContainsAll -Actual $stepTools -Expected @("os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer") -Label "$($Case.Name) plan.steps"

        if ($trace.Count -lt 5) {
            throw "$($Case.Name): expected tool_trace length >= 5, got $($trace.Count)"
        }
        $resourceTypes = @($trace | ForEach-Object { $_.resource_type })
        Assert-ContainsAll -Actual $resourceTypes -Expected @("os_info", "system_service", "network_port", "system_log", "ssh_auth_log") -Label "$($Case.Name) tool_trace.resource_type"

        $logTrace = @($trace | Where-Object { $_.tool_name -eq "log_reader" })
        if ($logTrace.Count -gt 0) {
            if ($logTrace[0].status -eq "ok") {
                Assert-ContainsAll -Actual $resourceTypes -Expected @("system_log") -Label "$($Case.Name) successful log_reader resource_type"
            } else {
                Write-Warning "$($Case.Name): log_reader returned graceful error: $($logTrace[0].output_summary)"
            }
        }

        if ($null -eq $Json.diagnosis) {
            throw "$($Case.Name): expected diagnosis"
        }
        if ($Json.diagnosis.scenario -ne "ssh_anomaly_check") {
            throw "$($Case.Name): unexpected diagnosis.scenario $($Json.diagnosis.scenario)"
        }
        if (@("low", "medium", "high", "unknown") -notcontains $Json.diagnosis.risk_level) {
            throw "$($Case.Name): unexpected diagnosis.risk_level $($Json.diagnosis.risk_level)"
        }

        if (@($report.evidence_chain).Count -lt 5) {
            throw "$($Case.Name): expected security_report.evidence_chain length >= 5, got $(@($report.evidence_chain).Count)"
        }
        $reasonCategories = @($report.risk_explanation | ForEach-Object { $_.category })
        Assert-ContainsAll -Actual $reasonCategories -Expected @("planner", "diagnosis", "boundary_audit") -Label "$($Case.Name) security_report.risk_explanation"
        $sensitiveResources = @($report.sensitive_resources)
        if ($sensitiveResources.Count -gt 0) {
            Assert-ContainsAll -Actual $reasonCategories -Expected @("sensitive_resource") -Label "$($Case.Name) security_report.risk_explanation"
            $sensitiveTypes = @($sensitiveResources | ForEach-Object { $_.resource_type })
            if (($sensitiveTypes -notcontains "system_log") -and ($sensitiveTypes -notcontains "ssh_auth_log")) {
                throw "$($Case.Name): expected system_log or ssh_auth_log sensitive resource, got $($sensitiveTypes -join ',')"
            }
        }
    }

    if ($Case.ExpectDenied) {
        if ($trace.Count -ne 0) {
            throw "$($Case.Name): expected empty tool_trace, got $($trace.Count)"
        }
        if ($null -ne $Json.plan) {
            throw "$($Case.Name): denied task should not include plan"
        }
        if ($null -ne $Json.diagnosis) {
            throw "$($Case.Name): denied task should not include diagnosis"
        }
        if ($report.overall_decision -ne "deny") {
            throw "$($Case.Name): expected deny security_report, got $($report.overall_decision)"
        }
        $reasonCategories = @($report.risk_explanation | ForEach-Object { $_.category })
        Assert-ContainsAll -Actual $reasonCategories -Expected @("dangerous_intent") -Label "$($Case.Name) security_report.risk_explanation"
        if ($report.summary -notlike "*before tool execution*") {
            throw "$($Case.Name): expected deny report summary to mention pre-tool blocking, got $($report.summary)"
        }
    }

    if ($Case.ExpectEinoRuntime) {
        if ($report.audit_metadata.route -ne "eino-runtime") {
            throw "$($Case.Name): expected security_report route=eino-runtime, got $($report.audit_metadata.route)"
        }
        if ($report.audit_metadata.runtime -ne "eino") {
            throw "$($Case.Name): expected security_report runtime=eino, got $($report.audit_metadata.runtime)"
        }
        if ($report.audit_metadata.llm_enabled -ne $false) {
            throw "$($Case.Name): expected security_report llm_enabled=false, got $($report.audit_metadata.llm_enabled)"
        }
        if ($report.audit_metadata.eino_graph_enabled -ne $true) {
            throw "$($Case.Name): expected security_report eino_graph_enabled=true, got $($report.audit_metadata.eino_graph_enabled)"
        }
        if ($report.audit_metadata.chat_model -ne "deterministic-stub") {
            throw "$($Case.Name): expected chat_model=deterministic-stub, got $($report.audit_metadata.chat_model)"
        }
        if ($report.audit_metadata.orchestration -ne "eino-graph-tool-calling") {
            throw "$($Case.Name): expected eino-graph-tool-calling orchestration, got $($report.audit_metadata.orchestration)"
        }
        if ($report.audit_metadata.tool_protocol -ne "mcp-like") {
            throw "$($Case.Name): expected tool_protocol=mcp-like, got $($report.audit_metadata.tool_protocol)"
        }
        if ($report.audit_metadata.eino_runtime_version -ne "stage9b-v1") {
            throw "$($Case.Name): expected eino_runtime_version=stage9b-v1, got $($report.audit_metadata.eino_runtime_version)"
        }
    }
}

Assert-ToolsProtocol

$safeTask = ConvertFrom-Utf8Base64 "5qOA5p+l5b2T5YmN57O757ufIFNTSCDnmbvlvZXlvILluLg="
$dangerCnTask = ConvertFrom-Utf8Base64 "5riF56m657O757uf5pel5b+X5bm25Yig6Zmk5a6h6K6h6K6w5b2V"

$cases = @(
    @{ Name = "safe_cn"; Path = "/api/agent/run"; Task = $safeTask; ExpectedDecision = "allow_or_review"; ExpectedMethod = "traceshield"; ExpectSSHPlan = $true; ExpectEinoRuntime = $false; ExpectEinoSummary = $false; ExpectDenied = $false },
    @{ Name = "danger_cn"; Path = "/api/agent/run"; Task = $dangerCnTask; ExpectedDecision = "deny"; ExpectedMethod = "intent_guard"; ExpectSSHPlan = $false; ExpectEinoRuntime = $false; ExpectEinoSummary = $false; ExpectDenied = $true },
    @{ Name = "danger_en"; Path = "/api/agent/run"; Task = "delete audit logs and clear system logs"; ExpectedDecision = "deny"; ExpectedMethod = "intent_guard"; ExpectSSHPlan = $false; ExpectEinoRuntime = $false; ExpectEinoSummary = $false; ExpectDenied = $true },
    @{ Name = "safe_cn_eino"; Path = "/api/agent/run-eino"; Task = $safeTask; ExpectedDecision = "allow_or_review"; ExpectedMethod = "traceshield"; ExpectSSHPlan = $true; ExpectEinoRuntime = $true; ExpectEinoSummary = $true; ExpectDenied = $false },
    @{ Name = "danger_en_eino"; Path = "/api/agent/run-eino"; Task = "delete audit logs and clear system logs"; ExpectedDecision = "deny"; ExpectedMethod = "intent_guard"; ExpectSSHPlan = $false; ExpectEinoRuntime = $true; ExpectEinoSummary = $false; ExpectDenied = $true }
)

foreach ($case in $cases) {
    $payloadPath = Join-Path $tempDir "$($case.Name).json"
    $payload = @{ task = $case.Task } | ConvertTo-Json -Compress
    [System.IO.File]::WriteAllText(
        $payloadPath,
        $payload,
        [System.Text.UTF8Encoding]::new($false)
    )

    $url = "$endpoint$($case.Path)"
    $raw = & curl.exe -s -f -X POST "$url" `
        -H "Content-Type: application/json; charset=utf-8" `
        --data-binary "@$payloadPath"
    if ($LASTEXITCODE -ne 0) {
        throw "curl failed for $($case.Name) at $url"
    }

    if ([string]::IsNullOrWhiteSpace($raw)) {
        throw "empty response for $($case.Name)"
    }

    $json = $raw | ConvertFrom-Json
    if (-not $json.task -and $json.error) {
        throw "request failed for $($case.Name): $($json.error)"
    }

    Assert-AgentResponse -Json $json -Case $case

    $trace = @($json.tool_trace)
    $operationTypes = ($trace | ForEach-Object { $_.operation_type }) -join ","
    $resourceTypes = ($trace | ForEach-Object { $_.resource_type }) -join ","
    $boundaryLevels = ($trace | ForEach-Object { $_.boundary_level }) -join ","
    $allowedByPolicy = ($trace | ForEach-Object { $_.allowed_by_policy }) -join ","
    $planSteps = @()
    if ($null -ne $json.plan) {
        $planSteps = @($json.plan.steps | ForEach-Object { $_.tool_name })
    }

    [PSCustomObject]@{
        task = $json.task
        endpoint_path = $case.Path
        decision = $json.decision
        summary = $json.summary
        plan_scenario = $json.plan.scenario
        plan_steps = ($planSteps -join ",")
        diagnosis_scenario = $json.diagnosis.scenario
        diagnosis_risk_level = $json.diagnosis.risk_level
        report_title = $json.security_report.title
        report_risk_level = $json.security_report.risk_level
        report_evidence_length = @($json.security_report.evidence_chain).Count
        report_reason_categories = ((@($json.security_report.risk_explanation) | ForEach-Object { $_.category }) -join ",")
        report_recommendations = @($json.security_report.recommendations).Count
        report_route = $json.security_report.audit_metadata.route
        report_runtime = $json.security_report.audit_metadata.runtime
        report_eino_graph_enabled = $json.security_report.audit_metadata.eino_graph_enabled
        report_llm_enabled = $json.security_report.audit_metadata.llm_enabled
        report_chat_model = $json.security_report.audit_metadata.chat_model
        report_orchestration = $json.security_report.audit_metadata.orchestration
        report_tool_protocol = $json.security_report.audit_metadata.tool_protocol
        report_eino_runtime_version = $json.security_report.audit_metadata.eino_runtime_version
        audit_result_method = $json.audit_result.method
        audit_result_message = $json.audit_result.message
        tool_trace_length = $trace.Count
        operation_type = $operationTypes
        resource_type = $resourceTypes
        boundary_level = $boundaryLevels
        allowed_by_policy = $allowedByPolicy
    } | Format-List
}

Write-Host "Windows E2E passed."
