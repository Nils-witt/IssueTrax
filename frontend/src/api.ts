export type IssueStatus = 'open' | 'closed';

export interface Issue {
  id: string;
  title: string;
  description: string;
  status: IssueStatus;
  createdAt: string;
  updatedAt: string;
}

export interface CreateIssueInput {
  title: string;
  description?: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(body.error ?? `${res.status} ${res.statusText}`);
  }

  return (await res.json()) as T;
}

export function listIssues(): Promise<Issue[]> {
  return request<Issue[]>('/api/issues');
}

export function createIssue(input: CreateIssueInput): Promise<Issue> {
  return request<Issue>('/api/issues', { method: 'POST', body: JSON.stringify(input) });
}
