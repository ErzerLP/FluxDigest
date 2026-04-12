import type {
  AdminConfigSnapshot,
  AdminStatus,
  ConnectivityTestResult,
  JobRunDetail,
  JobRunListResponse,
  LLMTestDraft,
  ProfileVersion,
  RunJobResponse,
  UpdateLLMConfigInput,
} from '../../types/admin';

const apiBaseURL = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/$/, '');
const adminBaseURL = `${apiBaseURL}/admin`;
const defaultAdminRequestTimeoutMS = 15000;
const adminRequestTimeoutMessage = '请求超时，请检查代理、网络或服务地址后重试。';

type adminRuntimeGlobals = typeof globalThis & {
  __ADMIN_REQUEST_TIMEOUT_MS__?: number;
};

export class AdminApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'AdminApiError';
    this.status = status;
  }
}

function extractAdminErrorMessage(payload: unknown, status: number) {
  if (typeof payload === 'string' && payload.trim()) {
    return payload.trim();
  }

  if (payload && typeof payload === 'object') {
    if ('error' in payload && typeof payload.error === 'string' && payload.error.trim()) {
      return payload.error;
    }

    if ('message' in payload && typeof payload.message === 'string' && payload.message.trim()) {
      return payload.message;
    }
  }

  return `Admin request failed with status ${status}`;
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T;
  }

  const contentType = response.headers.get('content-type') ?? '';
  if (contentType.includes('application/json')) {
    return (await response.json()) as T;
  }

  return (await response.text()) as T;
}

function getAdminRequestTimeoutMS() {
  const runtimeTimeout = (globalThis as adminRuntimeGlobals).__ADMIN_REQUEST_TIMEOUT_MS__;
  if (typeof runtimeTimeout === 'number' && runtimeTimeout > 0) {
    return runtimeTimeout;
  }

  return defaultAdminRequestTimeoutMS;
}

async function requestAdmin<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set('Accept', 'application/json');

  if (init?.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const controller = new AbortController();
  const timeoutMS = getAdminRequestTimeoutMS();
  let timeoutID = 0;
  const timeoutPromise = new Promise<Response>((_, reject) => {
    timeoutID = window.setTimeout(() => {
      controller.abort();
      reject(new AdminApiError(adminRequestTimeoutMessage, 408));
    }, timeoutMS);
  });

  let response: Response;
  try {
    response = (await Promise.race([
      fetch(`${adminBaseURL}${path}`, {
        ...init,
        headers,
        signal: controller.signal,
      }),
      timeoutPromise,
    ])) as Response;
  } catch (error) {
    if (error instanceof AdminApiError) {
      throw error;
    }

    if (error instanceof Error && error.name === 'AbortError') {
      throw new AdminApiError(adminRequestTimeoutMessage, 408);
    }

    throw error;
  } finally {
    window.clearTimeout(timeoutID);
  }

  if (!response.ok) {
    try {
      const payload = await parseResponse<unknown>(response);
      throw new AdminApiError(extractAdminErrorMessage(payload, response.status), response.status);
    } catch (error) {
      if (error instanceof AdminApiError) {
        throw error;
      }

      throw new AdminApiError(
        `Admin request failed with status ${response.status}`,
        response.status,
      );
    }
  }

  return parseResponse<T>(response);
}

export function getAdminStatus() {
  return requestAdmin<AdminStatus>('/status');
}

export function getAdminConfigs() {
  return requestAdmin<AdminConfigSnapshot>('/configs');
}

export function updateLLMConfig(input: UpdateLLMConfigInput) {
  return requestAdmin<ProfileVersion>('/configs/llm', {
    method: 'PUT',
    body: JSON.stringify(input),
  });
}

export function testLLMConfig(input: LLMTestDraft) {
  return requestAdmin<ConnectivityTestResult>('/test/llm', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export function getJobRuns(limit = 20) {
  const query = new URLSearchParams({ limit: String(limit) });
  return requestAdmin<JobRunListResponse>(`/jobs?${query.toString()}`);
}

export function getJobRunDetail(jobId: string) {
  return requestAdmin<JobRunDetail>(`/jobs/${encodeURIComponent(jobId)}`);
}

export function runDailyDigest() {
  return requestAdmin<RunJobResponse>('/jobs/daily-digest/run', {
    method: 'POST',
  });
}
