import axios, { AxiosError } from 'axios'
import type {
  AcceptanceSummaryResponse,
  AgentCapabilitiesResponse,
  AgentRunResponse,
  HealthResponse,
  RuntimeStatusResponse
} from '../types/agent'

const client = axios.create({
  timeout: 20000,
  headers: {
    'Content-Type': 'application/json; charset=utf-8'
  }
})

export async function getHealth(): Promise<HealthResponse> {
  try {
    const response = await client.get<HealthResponse>('/health')
    return response.data
  } catch (error) {
    throw normalizeError(error, 'Go Agent health check failed')
  }
}

export async function runAgent(task: string): Promise<AgentRunResponse> {
  return postAgentRun('/api/agent/run', task)
}

export async function runAgentEino(task: string): Promise<AgentRunResponse> {
  return postAgentRun('/api/agent/run-eino', task)
}

export async function getRuntimeStatus(): Promise<RuntimeStatusResponse> {
  try {
    const response = await client.get<RuntimeStatusResponse>('/api/agent/runtime-status')
    return response.data
  } catch (error) {
    throw normalizeError(error, 'Runtime status request failed')
  }
}

export async function getAgentCapabilities(): Promise<AgentCapabilitiesResponse> {
  try {
    const response = await client.get<AgentCapabilitiesResponse>('/api/agent/capabilities')
    return response.data
  } catch (error) {
    throw normalizeError(error, 'Agent capabilities request failed')
  }
}

export async function getAcceptanceSummary(): Promise<AcceptanceSummaryResponse> {
  try {
    const response = await client.get<AcceptanceSummaryResponse>('/api/agent/acceptance-summary')
    return response.data
  } catch (error) {
    throw normalizeError(error, 'Acceptance summary request failed')
  }
}

async function postAgentRun(path: string, task: string): Promise<AgentRunResponse> {
  try {
    const response = await client.post<AgentRunResponse>(path, { task })
    return response.data
  } catch (error) {
    throw normalizeError(error, 'Agent run request failed')
  }
}

function normalizeError(error: unknown, fallback: string): Error {
  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<{ error?: string; message?: string }>
    const serverMessage = axiosError.response?.data?.error || axiosError.response?.data?.message
    const status = axiosError.response?.status
    const detail = serverMessage || axiosError.message
    return new Error(status ? `${fallback}: HTTP ${status} ${detail}` : `${fallback}: ${detail}`)
  }
  if (error instanceof Error) {
    return new Error(`${fallback}: ${error.message}`)
  }
  return new Error(fallback)
}
