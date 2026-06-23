import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";
export const maxDuration = 180;

const backendBase = (
  process.env.KYLIN_GUARD_AGENT_API_URL ||
  process.env.KYLINGUARD_AGENT_API_URL ||
  "http://127.0.0.1:8080"
).replace(/\/$/, "");

const backendTimeoutMs = 180_000;

type RouteContext = {
  params: Promise<{ path?: string[] }>;
};

async function proxyAgentRequest(request: NextRequest, context: RouteContext) {
  const { path = [] } = await context.params;
  const sourceURL = new URL(request.url);
  const targetPath = path.map(encodeURIComponent).join("/");
  const targetURL = `${backendBase}/api/agent/${targetPath}${sourceURL.search}`;

  const headers = new Headers();
  const contentType = request.headers.get("content-type");
  const accept = request.headers.get("accept");
  if (contentType) {
    headers.set("content-type", contentType);
  }
  if (accept) {
    headers.set("accept", accept);
  }

  let body: ArrayBuffer | undefined;
  if (request.method !== "GET" && request.method !== "HEAD") {
    body = await request.arrayBuffer();
  }

  try {
    const response = await fetch(targetURL, {
      method: request.method,
      headers,
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(backendTimeoutMs),
    });
    const responseBody = await response.arrayBuffer();
    const responseHeaders = new Headers();
    const responseContentType = response.headers.get("content-type");
    if (responseContentType) {
      responseHeaders.set("content-type", responseContentType);
    }
    responseHeaders.set("cache-control", "no-store");
    return new NextResponse(responseBody, {
      status: response.status,
      headers: responseHeaders,
    });
  } catch (error) {
    const timedOut = error instanceof Error && error.name === "TimeoutError";
    return NextResponse.json(
      {
        error: {
          code: timedOut ? "AGENT_PROXY_TIMEOUT" : "AGENT_PROXY_FAILED",
          message: timedOut ? "Agent request timed out" : "Failed to reach KylinGuard Agent backend",
          details: {},
        },
      },
      { status: timedOut ? 504 : 502 },
    );
  }
}

export async function GET(request: NextRequest, context: RouteContext) {
  return proxyAgentRequest(request, context);
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxyAgentRequest(request, context);
}

export async function PUT(request: NextRequest, context: RouteContext) {
  return proxyAgentRequest(request, context);
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxyAgentRequest(request, context);
}
