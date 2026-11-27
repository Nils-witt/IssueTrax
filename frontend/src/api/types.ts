// Wire types of the IssueTrax JSON API (see docs/openapi.yaml).

export const issueStatuses = ['draft', 'open', 'closed', 'archived'] as const;
export type IssueStatus = (typeof issueStatuses)[number];

export type MatchMode = 'all' | 'any';

export interface User {
  id: string;
  /** Empty for service accounts. */
  email: string;
  displayName: string;
  /** True for "bot-<name>" users that call the API with API keys. */
  serviceAccount: boolean;
}

/** The signed-in user (GET /api/me). */
/** Build metadata from /api/version. */
export interface VersionInfo {
  /** `git describe --tags` output, e.g. `v1.2.0`, or `dev` for local builds. */
  version: string;
  /** Short commit hash, or `unknown`. */
  commit: string;
}

export interface Me extends User {
  /** Member of the admin group at the latest sign-in; may read the audit log. */
  admin: boolean;
}

export const auditActions = [
  'auth.login',
  'auth.login_failed',
  'auth.logout',
  'project.create',
  'project.update',
  'issue.create',
  'issue.update',
  'tag.update',
  'comment.create',
  'comment.update',
  'comment.delete',
  'attachment.create',
  'attachment.delete',
  'storage.create',
  'storage.update',
  'generic_storage_item.create',
  'generic_storage_item.update',
  'generic_storage_item.delete',
  'generic_storage_item.restore',
  'network_device.create',
  'network_device.update',
  'network_device.delete',
  'network_device.restore',
  'service_account.create',
  'api_key.create',
  'api_key.revoke',
] as const;
export type AuditAction = (typeof auditActions)[number];

/** One changed field; `old` is absent on creation, long text is only `redacted`. */
export interface AuditChange {
  old?: string;
  new?: string;
  redacted?: boolean;
}

export interface AuditEntry {
  id: string;
  at: string;
  /** Null for failed sign-ins. */
  actor: { id: string; displayName: string; serviceAccount: boolean } | null;
  action: AuditAction;
  targetType: string;
  targetId: string;
  /** Display snapshot such as "TRAX-12". */
  targetLabel: string;
  changes: Record<string, AuditChange>;
  ip: string;
  userAgent: string;
}

export interface AuditFilter {
  actions: AuditAction[];
  actor: string | null;
}

/** A service account: the user "bot-<name>". */
export interface ServiceAccount extends User {
  /** The name without "bot-". */
  name: string;
  createdAt: string;
}

export interface ApiKey {
  id: string;
  name: string;
  /** The token's first characters, safe to display. */
  prefix: string;
  createdBy: string;
  createdAt: string;
  expiresAt: string | null;
  lastUsedAt: string | null;
  revokedAt: string | null;
}

/** A freshly created key; the token is never shown again. */
export interface CreatedApiKey extends ApiKey {
  token: string;
}

export interface CreateApiKeyInput {
  name: string;
  /** RFC 3339; omitted or null for no expiry. */
  expiresAt?: string | null;
}

export interface Project {
  id: string;
  key: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export interface Tag {
  id: string;
  name: string;
  /** Display color, "#rrggbb". */
  color: string;
}

export interface UpdateTagInput {
  name?: string;
  color?: string;
}

export interface Issue {
  id: string;
  key: string;
  number: number;
  projectKey: string;
  title: string;
  description: string;
  status: IssueStatus;
  tags: Tag[];
  assignees: User[];
  createdBy: User;
  createdAt: string;
  updatedAt: string;
}

export interface Comment {
  id: string;
  /** Markdown. */
  body: string;
  author: User;
  createdAt: string;
  /** Equals createdAt until the body is edited. */
  updatedAt: string;
}

/** Largest file the server accepts as an attachment (10 MiB). */
export const maxAttachmentBytes = 10 * 1024 * 1024;

/** A file attached to an issue; its content is served at attachmentUrl(). */
export interface Attachment {
  id: string;
  fileName: string;
  /** As declared by the uploader; not verified. */
  mediaType: string;
  /** In bytes. */
  size: number;
  uploader: User;
  createdAt: string;
}

/** A physical storage location; storages form a tree through parentId. */
export interface Storage {
  id: string;
  /** Null for a root storage. */
  parentId: string | null;
  name: string;
  /** Number of direct children. */
  childCount: number;
  /** Number of items (any kind, not deleted) kept directly in it. */
  itemCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateStorageInput {
  name: string;
  parentId?: string | null;
}

export interface UpdateStorageInput {
  name?: string;
  /** Moves the storage; null makes it a root. */
  parentId?: string | null;
}

/** A plain item kept in a storage. Deleting only sets deletedAt. */
export interface GenericStorageItem {
  id: string;
  storageId: string;
  name: string;
  /** Plain text; may be empty. */
  description: string;
  createdAt: string;
  updatedAt: string;
  /** Null unless the item is deleted. */
  deletedAt: string | null;
}

export interface CreateGenericStorageItemInput {
  storageId: string;
  name: string;
  description?: string;
}

export interface UpdateGenericStorageItemInput {
  name?: string;
  description?: string;
  /** Moves the item to this storage. */
  storageId?: string;
}

export type NetworkRole = 'router' | 'switch' | 'client';
export type NetworkInterfaceType = 'wired' | 'wireless';
export type IPv4Mode = 'dhcp' | 'static' | 'off';
/** SLAAC only exists for IPv6. */
export type IPv6Mode = 'dhcp' | 'slaac' | 'static' | 'off';

/** One IP family of an interface; address, prefix, gateway and DNS are only set when static. */
export interface IPConfig<Mode extends string> {
  mode: Mode;
  address: string | null;
  prefixLength: number | null;
  gateway: string | null;
  dnsServers: string[];
}

export interface NetworkInterface {
  name: string;
  /** Lower case, colon-separated. */
  mac: string;
  type: NetworkInterfaceType;
  /** Always empty for wired interfaces. */
  ssid: string;
  /** The untagged VLAN, if any. */
  nativeVlan: number | null;
  taggedVlans: number[];
  ipv4: IPConfig<IPv4Mode>;
  ipv6: IPConfig<IPv6Mode>;
}

/** A network device kept in a storage. Deleting only sets deletedAt. */
export interface NetworkDevice {
  id: string;
  storageId: string;
  name: string;
  description: string;
  role: NetworkRole;
  hostname: string;
  manufacturer: string;
  model: string;
  serialNumber: string;
  primaryInterface: NetworkInterface;
  secondaryInterfaces: NetworkInterface[];
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}

/** Static settings are only allowed (and address and prefix required) when mode is 'static'. */
export interface IPConfigInput<Mode extends string> {
  mode: Mode;
  address?: string | null;
  prefixLength?: number | null;
  gateway?: string | null;
  dnsServers?: string[];
}

export interface NetworkInterfaceInput {
  name: string;
  mac: string;
  type: NetworkInterfaceType;
  /** Only for wireless interfaces. */
  ssid?: string | null;
  nativeVlan?: number | null;
  taggedVlans?: number[];
  ipv4: IPConfigInput<IPv4Mode>;
  ipv6: IPConfigInput<IPv6Mode>;
}

export interface CreateNetworkDeviceInput {
  storageId: string;
  name: string;
  description?: string;
  role: NetworkRole;
  hostname?: string;
  manufacturer?: string;
  model?: string;
  serialNumber?: string;
  primaryInterface: NetworkInterfaceInput;
  secondaryInterfaces?: NetworkInterfaceInput[];
}

/** Partial update; the interfaces given replace the stored ones, storageId moves the device. */
export type UpdateNetworkDeviceInput = Partial<CreateNetworkDeviceInput>;

/** Narrows an item list. */
export interface StorageItemFilter {
  /** Only when listing a storage: include items anywhere below it. */
  recursive?: boolean;
  /** Only deleted items instead of only live ones. */
  deleted?: boolean;
  /** Only when searching: name or description contains this, ignoring case. */
  q?: string;
}

export interface Page<T> {
  items: T[];
  total: number;
  nextCursor: string | null;
}

export interface CreateProjectInput {
  key: string;
  name: string;
  description?: string;
}

export interface UpdateProjectInput {
  name?: string;
  description?: string;
}

export interface CreateIssueInput {
  title: string;
  description?: string;
  status?: IssueStatus;
  tags?: string[];
  assignees?: string[];
}

export type UpdateIssueInput = Partial<CreateIssueInput>;
