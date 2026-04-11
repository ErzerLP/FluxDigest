import type {
  AdminConfigSnapshot,
  AdminStatus,
  ConnectivityTestResult,
  JobRunListResponse,
  LLMTestDraft,
  ProfileVersion,
  UpdateLLMConfigInput,
} from '../../types/admin';

const apiBaseURL = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/$/, '');
const adminBaseURL = `${apiBaseURL}/admin`;

export class AdminApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'AdminApiError';
    this.status = status;
  }
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

async function requestAdmin<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set('Accept', 'application/json');

  if (init?.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(`${adminBaseURL}${path}`, {
    ...init,
    headers,
  });

  if (!response.ok) {
    let message = `Admin request failed with status ${response.status}`;

    try {
      const payload = await parseResponse<unknown>(response);
      if (typeof payload === 'string' && payload.trim()) {
        message = payload.trim();
      } else if (
        payload &&
        typeof payload === 'object' &&
        'message' in payload &&
        typeof payload.message === 'string'
      ) {
        message = payload.message;
      }
    } catch {
      // Ignore body parsing errors and use the default message.
    }

    throw new AdminApiError(message, response.status);
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
