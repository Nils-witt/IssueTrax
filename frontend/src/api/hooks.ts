// TanStack Query hooks for every API resource.
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { query, request } from './client';
import type {
  ApiKey,
  Attachment,
  AuditEntry,
  AuditFilter,
  Comment,
  CreateApiKeyInput,
  CreatedApiKey,
  CreateIssueInput,
  CreateProjectInput,
  CreateStorageInput,
  Issue,
  Me,
  Page,
  Project,
  ServiceAccount,
  Storage,
  Tag,
  UpdateIssueInput,
  UpdateTagInput,
  UpdateProjectInput,
  UpdateStorageInput,
  User,
  VersionInfo,
} from './types';

const pk = (key: string) => encodeURIComponent(key);

export const keys = {
  me: ['me'] as const,
  version: ['version'] as const,
  users: ['users'] as const,
  projects: ['projects'] as const,
  project: (key: string) => ['project', key.toUpperCase()] as const,
  tags: (key: string) => ['tags', key.toUpperCase()] as const,
  issues: (key: string) => ['issues', key.toUpperCase()] as const,
  issue: (key: string, number: number) => ['issue', key.toUpperCase(), number] as const,
  comments: (key: string, number: number) => ['comments', key.toUpperCase(), number] as const,
  attachments: (key: string, number: number) => ['attachments', key.toUpperCase(), number] as const,
  serviceAccounts: ['serviceAccounts'] as const,
  apiKeys: (accountId: string) => ['apiKeys', accountId] as const,
  auditLog: (filter: AuditFilter) => ['auditLog', filter] as const,
  /** Prefix of every storage query. */
  storages: ['storages'] as const,
  /** Children of parent; null lists the roots. */
  storageChildren: (parent: string | null) => ['storages', 'children', parent ?? ''] as const,
};

export function useServiceAccounts() {
  return useQuery({ queryKey: keys.serviceAccounts, queryFn: () => request<ServiceAccount[]>('GET', '/api/service-accounts') });
}

export function useCreateServiceAccount() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => request<ServiceAccount>('POST', '/api/service-accounts', { name }),
    onSuccess: () =>
      Promise.all([qc.invalidateQueries({ queryKey: keys.serviceAccounts }), qc.invalidateQueries({ queryKey: keys.users })]),
  });
}

const accountPath = (id: string) => `/api/service-accounts/${encodeURIComponent(id)}/keys`;

export function useApiKeys(accountId: string) {
  return useQuery({ queryKey: keys.apiKeys(accountId), queryFn: () => request<ApiKey[]>('GET', accountPath(accountId)) });
}

export function useCreateApiKey(accountId: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (input: CreateApiKeyInput) => request<CreatedApiKey>('POST', accountPath(accountId), input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.apiKeys(accountId) }),
  });
}

export function useRevokeApiKey(accountId: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (keyId: string) => request<null>('DELETE', `${accountPath(accountId)}/${encodeURIComponent(keyId)}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.apiKeys(accountId) }),
  });
}

export function useVersion() {
  return useQuery({ queryKey: keys.version, queryFn: () => request<VersionInfo>('GET', '/api/version'), staleTime: Infinity });
}

export function useMe() {
  return useQuery({ queryKey: keys.me, queryFn: () => request<Me>('GET', '/api/me'), staleTime: Infinity });
}

export function useUsers() {
  return useQuery({ queryKey: keys.users, queryFn: () => request<User[]>('GET', '/api/users') });
}

export function useProjects() {
  return useInfiniteQuery({
    queryKey: keys.projects,
    queryFn: ({ pageParam }) => request<Page<Project>>('GET', `/api/projects${query({ cursor: pageParam })}`),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

export function useProject(key: string) {
  return useQuery({ queryKey: keys.project(key), queryFn: () => request<Project>('GET', `/api/projects/${pk(key)}`) });
}

export function useCreateProject() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (input: CreateProjectInput) => request<Project>('POST', '/api/projects', input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.projects }),
  });
}

export function useUpdateProject(key: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (input: UpdateProjectInput) => request<Project>('PATCH', `/api/projects/${pk(key)}`, input),
    onSuccess: (project) => {
      qc.setQueryData(keys.project(key), project);
      void qc.invalidateQueries({ queryKey: keys.projects });
    },
  });
}

export function useTags(key: string) {
  return useQuery({ queryKey: keys.tags(key), queryFn: () => request<Tag[]>('GET', `/api/projects/${pk(key)}/tags`) });
}

export function useUpdateTag(key: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ id, ...input }: UpdateTagInput & { id: string }) =>
      request<Tag>('PATCH', `/api/projects/${pk(key)}/tags/${encodeURIComponent(id)}`, input),
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: keys.tags(key) }),
        qc.invalidateQueries({ queryKey: keys.issues(key) }),
        qc.invalidateQueries({ queryKey: ['issue', key.toUpperCase()] }),
      ]),
  });
}

/**
 * Lists issues. filterQuery is the URL query string of the issue list
 * page (status, tag, tagMode, assignee, assigneeMode, q), which uses the
 * same parameter names as the API.
 */
export function useIssues(key: string, filterQuery: string) {
  return useInfiniteQuery({
    queryKey: [...keys.issues(key), filterQuery],
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams(filterQuery);
      if (pageParam) params.set('cursor', pageParam);
      const qs = params.toString();

      return request<Page<Issue>>('GET', `/api/projects/${pk(key)}/issues${qs ? `?${qs}` : ''}`);
    },
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

export function useIssue(key: string, number: number) {
  return useQuery({
    queryKey: keys.issue(key, number),
    queryFn: () => request<Issue>('GET', `/api/projects/${pk(key)}/issues/${number}`),
  });
}

function useIssueSaved(key: string) {
  const qc = useQueryClient();

  return (issue: Issue) => {
    qc.setQueryData(keys.issue(key, issue.number), issue);

    return Promise.all([
      qc.invalidateQueries({ queryKey: keys.issues(key) }),
      qc.invalidateQueries({ queryKey: keys.tags(key) }), // new tags may have been created
    ]);
  };
}

export function useCreateIssue(key: string) {
  const onSuccess = useIssueSaved(key);

  return useMutation({
    mutationFn: (input: CreateIssueInput) => request<Issue>('POST', `/api/projects/${pk(key)}/issues`, input),
    onSuccess,
  });
}

export function useUpdateIssue(key: string, number: number) {
  const onSuccess = useIssueSaved(key);

  return useMutation({
    mutationFn: (input: UpdateIssueInput) => request<Issue>('PATCH', `/api/projects/${pk(key)}/issues/${number}`, input),
    onSuccess,
  });
}

export interface BulkIssueUpdate {
  number: number;
  input: UpdateIssueInput;
}

/** Outcome of a bulk update: issues that failed, with their errors. */
export interface BulkResult {
  failed: { number: number; error: unknown }[];
}

/**
 * Updates several issues, one PATCH each. Individual failures (such as
 * an archived issue rejecting a tag change) do not stop the others; they
 * are reported in the result.
 */
export function useBulkUpdateIssues(key: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (updates: BulkIssueUpdate[]): Promise<BulkResult> => {
      const failed: BulkResult['failed'] = [];

      await Promise.all(
        updates.map(async (u) => {
          try {
            const issue = await request<Issue>('PATCH', `/api/projects/${pk(key)}/issues/${u.number}`, u.input);
            qc.setQueryData(keys.issue(key, issue.number), issue);
          } catch (error) {
            failed.push({ number: u.number, error });
          }
        }),
      );

      return { failed };
    },
    onSettled: () =>
      Promise.all([qc.invalidateQueries({ queryKey: keys.issues(key) }), qc.invalidateQueries({ queryKey: keys.tags(key) })]),
  });
}

const commentsPath = (key: string, number: number) => `/api/projects/${pk(key)}/issues/${number}/comments`;

export function useComments(key: string, number: number) {
  return useQuery({ queryKey: keys.comments(key, number), queryFn: () => request<Comment[]>('GET', commentsPath(key, number)) });
}

function useCommentsChanged(key: string, number: number) {
  const qc = useQueryClient();

  return () => qc.invalidateQueries({ queryKey: keys.comments(key, number) });
}

export function useCreateComment(key: string, number: number) {
  const onSuccess = useCommentsChanged(key, number);

  return useMutation({
    mutationFn: (body: string) => request<Comment>('POST', commentsPath(key, number), { body }),
    onSuccess,
  });
}

export function useUpdateComment(key: string, number: number) {
  const onSuccess = useCommentsChanged(key, number);

  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      request<Comment>('PATCH', `${commentsPath(key, number)}/${encodeURIComponent(id)}`, { body }),
    onSuccess,
  });
}

export function useDeleteComment(key: string, number: number) {
  const onSuccess = useCommentsChanged(key, number);

  return useMutation({
    mutationFn: (id: string) => request<null>('DELETE', `${commentsPath(key, number)}/${encodeURIComponent(id)}`),
    onSuccess,
  });
}

const attachmentsPath = (key: string, number: number) => `/api/projects/${pk(key)}/issues/${number}/attachments`;

/** Where an attachment's file is served (a download, or inline for images). */
export function attachmentUrl(key: string, number: number, id: string): string {
  return `${attachmentsPath(key, number)}/${encodeURIComponent(id)}`;
}

export function useAttachments(key: string, number: number) {
  return useQuery({ queryKey: keys.attachments(key, number), queryFn: () => request<Attachment[]>('GET', attachmentsPath(key, number)) });
}

function useAttachmentsChanged(key: string, number: number) {
  const qc = useQueryClient();

  return () => qc.invalidateQueries({ queryKey: keys.attachments(key, number) });
}

export function useUploadAttachment(key: string, number: number) {
  const onSuccess = useAttachmentsChanged(key, number);

  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData();
      form.append('file', file, file.name);
      return request<Attachment>('POST', attachmentsPath(key, number), form);
    },
    onSuccess,
  });
}

export function useDeleteAttachment(key: string, number: number) {
  const onSuccess = useAttachmentsChanged(key, number);

  return useMutation({
    mutationFn: (id: string) => request<null>('DELETE', attachmentUrl(key, number, id)),
    onSuccess,
  });
}

/** The audit log, newest first (admins only). Always refetched on open. */
export function useAuditLog(filter: AuditFilter) {
  return useInfiniteQuery({
    queryKey: keys.auditLog(filter),
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams();
      for (const a of filter.actions) params.append('action', a);
      if (filter.actor) params.set('actor', filter.actor);
      if (pageParam) params.set('cursor', pageParam);

      return request<Page<AuditEntry>>('GET', `/api/audit-log?${params.toString()}`);
    },
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
    staleTime: 0,
  });
}

/** One parent's direct children (the roots for null), paginated by name. */
export function useStorageChildren(parent: string | null, enabled = true) {
  return useInfiniteQuery({
    queryKey: keys.storageChildren(parent),
    queryFn: ({ pageParam }) =>
      request<Page<Storage>>('GET', `/api/storages${query({ parent: parent ?? undefined, cursor: pageParam })}`),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
    enabled,
  });
}

export function useCreateStorage() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: (input: CreateStorageInput) => request<Storage>('POST', '/api/storages', input),
    // The parent's child count changes too, so reload every storage list.
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.storages }),
  });
}

export function useUpdateStorage() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateStorageInput }) =>
      request<Storage>('PATCH', `/api/storages/${encodeURIComponent(id)}`, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.storages }),
  });
}
