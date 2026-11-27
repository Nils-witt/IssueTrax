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
  CreateGenericStorageItemInput,
  CreateNetworkDeviceInput,
  CreateStorageInput,
  GenericStorageItem,
  NetworkDevice,
  Issue,
  Me,
  Page,
  Project,
  ServiceAccount,
  Storage,
  StorageItemFilter,
  StorageItemRef,
  Tag,
  UpdateIssueInput,
  UpdateTagInput,
  UpdateProjectInput,
  UpdateGenericStorageItemInput,
  UpdateNetworkDeviceInput,
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
  storage: (id: string) => ['storages', 'one', id] as const,
  /** Prefix of every storage item query. */
  storageItems: ['storageItems'] as const,
  storageItemsIn: (storage: string, filter: StorageItemFilter) => ['storageItems', 'in', storage, filter] as const,
  storageItemSearch: (filter: StorageItemFilter) => ['storageItems', 'search', filter] as const,
  // Under storageItems, so item invalidation covers network devices too.
  networkDevicesIn: (storage: string, filter: StorageItemFilter) => ['storageItems', 'networkDevices', 'in', storage, filter] as const,
  networkDeviceSearch: (filter: StorageItemFilter) => ['storageItems', 'networkDevices', 'search', filter] as const,
  /** Items of any kind matching q, e.g. to link them to an issue. */
  storageItemLookup: (q: string) => ['storageItems', 'lookup', q] as const,
  /** Prefix of every linked-issues query; under storageItems, so item changes refresh it too. */
  storageItemIssuesAll: ['storageItems', 'issues'] as const,
  storageItemIssues: (id: string) => ['storageItems', 'issues', id] as const,
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
      qc.invalidateQueries({ queryKey: keys.storageItemIssuesAll }), // links may have changed
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

export function useStorage(id: string) {
  return useQuery({ queryKey: keys.storage(id), queryFn: () => request<Storage>('GET', `/api/storages/${encodeURIComponent(id)}`) });
}

const flag = (on: boolean | undefined) => (on ? 'true' : undefined);

/** The generic items in one storage (or its subtree), paginated by name. */
export function useStorageItems(storage: string, filter: StorageItemFilter) {
  return useInfiniteQuery({
    queryKey: keys.storageItemsIn(storage, filter),
    queryFn: ({ pageParam }) =>
      request<Page<GenericStorageItem>>(
        'GET',
        `/api/storages/${encodeURIComponent(storage)}/generic-storage-items${query({
          recursive: flag(filter.recursive),
          deleted: flag(filter.deleted),
          cursor: pageParam,
        })}`,
      ),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

/** Generic items across all storages whose name or description contains filter.q. */
export function useStorageItemSearch(filter: StorageItemFilter, enabled = true) {
  return useInfiniteQuery({
    queryKey: keys.storageItemSearch(filter),
    queryFn: ({ pageParam }) =>
      request<Page<GenericStorageItem>>(
        'GET',
        `/api/generic-storage-items${query({ q: filter.q, deleted: flag(filter.deleted), cursor: pageParam })}`,
      ),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
    enabled,
  });
}

/** Live (not deleted) items of any kind whose name or description contains q; the first page only. */
export function useStorageItemLookup(q: string) {
  return useQuery({
    queryKey: keys.storageItemLookup(q),
    queryFn: () => request<Page<StorageItemRef>>('GET', `/api/storage-items${query({ q, limit: '20' })}`),
    placeholderData: (prev) => prev,
  });
}

/** The issues of every project linking a storage item, newest first. */
export function useStorageItemIssues(id: string) {
  return useInfiniteQuery({
    queryKey: keys.storageItemIssues(id),
    queryFn: ({ pageParam }) =>
      request<Page<Issue>>('GET', `/api/storage-items/${encodeURIComponent(id)}/issues${query({ cursor: pageParam })}`),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

/** Item changes also change the storages' item counts. */
function useInvalidateStorageItems() {
  const qc = useQueryClient();

  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: keys.storageItems }),
      qc.invalidateQueries({ queryKey: keys.storages }),
      // Issues show the names and deletion state of their linked items.
      qc.invalidateQueries({ queryKey: ['issue'] }),
      qc.invalidateQueries({ queryKey: ['issues'] }),
    ]);
}

export function useCreateStorageItem() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({
    mutationFn: (input: CreateGenericStorageItemInput) => request<GenericStorageItem>('POST', '/api/generic-storage-items', input),
    onSuccess,
  });
}

const itemPath = (id: string) => `/api/generic-storage-items/${encodeURIComponent(id)}`;

export function useUpdateStorageItem() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateGenericStorageItemInput }) =>
      request<GenericStorageItem>('PATCH', itemPath(id), input),
    onSuccess,
  });
}

export function useDeleteStorageItem() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({ mutationFn: (id: string) => request<null>('DELETE', itemPath(id)), onSuccess });
}

export function useRestoreStorageItem() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({ mutationFn: (id: string) => request<GenericStorageItem>('POST', `${itemPath(id)}/restore`), onSuccess });
}

/** The network devices in one storage (or its subtree), paginated by name. */
export function useNetworkDevices(storage: string, filter: StorageItemFilter) {
  return useInfiniteQuery({
    queryKey: keys.networkDevicesIn(storage, filter),
    queryFn: ({ pageParam }) =>
      request<Page<NetworkDevice>>(
        'GET',
        `/api/storages/${encodeURIComponent(storage)}/network-devices${query({
          recursive: flag(filter.recursive),
          deleted: flag(filter.deleted),
          cursor: pageParam,
        })}`,
      ),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  });
}

/** Network devices across all storages whose name, description, hostname, serial number or MAC contains filter.q. */
export function useNetworkDeviceSearch(filter: StorageItemFilter, enabled = true) {
  return useInfiniteQuery({
    queryKey: keys.networkDeviceSearch(filter),
    queryFn: ({ pageParam }) =>
      request<Page<NetworkDevice>>(
        'GET',
        `/api/network-devices${query({ q: filter.q, deleted: flag(filter.deleted), cursor: pageParam })}`,
      ),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextCursor ?? undefined,
    enabled,
  });
}

export function useCreateNetworkDevice() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({
    mutationFn: (input: CreateNetworkDeviceInput) => request<NetworkDevice>('POST', '/api/network-devices', input),
    onSuccess,
  });
}

const devicePath = (id: string) => `/api/network-devices/${encodeURIComponent(id)}`;

export function useUpdateNetworkDevice() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateNetworkDeviceInput }) =>
      request<NetworkDevice>('PATCH', devicePath(id), input),
    onSuccess,
  });
}

export function useDeleteNetworkDevice() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({ mutationFn: (id: string) => request<null>('DELETE', devicePath(id)), onSuccess });
}

export function useRestoreNetworkDevice() {
  const onSuccess = useInvalidateStorageItems();

  return useMutation({ mutationFn: (id: string) => request<NetworkDevice>('POST', `${devicePath(id)}/restore`), onSuccess });
}
