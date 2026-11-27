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
  'storage_item.create',
  'storage_item.update',
  'storage_item.delete',
  'storage_item.restore',
  'item_type.create',
  'item_type.update',
  'item_type.delete',
  'item_type.restore',
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
  /** Linked storage items, deleted ones included. */
  storageItems: StorageItemRef[];
  createdBy: User;
  createdAt: string;
  updatedAt: string;
}

/** A storage item as linked from issues. */
export interface StorageItemRef {
  id: string;
  assetTag: string;
  storageId: string;
  name: string;
  deleted: boolean;
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
  /** Number of items (not deleted) kept directly in it. */
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

/** Module kinds in display order; all but network interfaces occur at most once per item. */
export const moduleKinds = ['hardware', 'network_host', 'purchase', 'sim_card', 'wwan', 'operating_system', 'network_interface'] as const;
export type ModuleKind = (typeof moduleKinds)[number];

/** Only network interfaces may occur several times (and be primary). */
export const isMultiModule = (kind: ModuleKind) => kind === 'network_interface';

/** A module new items of a type start with, count times (always 1 for single modules). */
export interface ModuleDefault {
  module: ModuleKind;
  count: number;
}

export interface ItemTypeRef {
  id: string;
  name: string;
  deleted: boolean;
}

/** An item type; types form a tree through parentId. Deleting only sets deletedAt. */
export interface ItemType {
  id: string;
  /** Null for a root type. */
  parentId: string | null;
  name: string;
  /** Starts proposed asset tags, separator included; empty inherits it. */
  tagPrefix: string;
  /** The type's own defaults, without the inherited ones. */
  defaults: ModuleDefault[];
  /** Direct children, deleted ones included. */
  childCount: number;
  /** Live items of exactly this type. */
  itemCount: number;
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}

/** An item type with what it inherits. */
export interface ItemTypeView extends ItemType {
  /** Ancestors from the root down to the parent. */
  path: ItemTypeRef[];
  effectiveDefaults: ModuleDefault[];
  effectiveTagPrefix: string;
}

export interface CreateItemTypeInput {
  name: string;
  parentId?: string | null;
  tagPrefix?: string;
  defaults?: ModuleDefault[];
}

export interface UpdateItemTypeInput {
  name?: string;
  /** Moves the type; null makes it a root. */
  parentId?: string | null;
  tagPrefix?: string;
  /** Replaces the type's own defaults. */
  defaults?: ModuleDefault[];
}

/** A manufacturer or hardware model as named by an item. */
export interface CatalogRef {
  id: string;
  name: string;
  deleted: boolean;
}

/** Manufacturer and model come from the central list; the refs are only in responses. */
export interface Hardware {
  manufacturerId: string | null;
  modelId: string | null;
  serialNumber: string;
  manufacturer: CatalogRef | null;
  model: CatalogRef | null;
}

export type HardwareInput = Pick<Hardware, 'manufacturerId' | 'modelId' | 'serialNumber'>;

/** An entry of the central manufacturer list. Deleting only sets deletedAt. */
export interface Manufacturer {
  id: string;
  name: string;
  /** Live models. */
  modelCount: number;
  /** Live items whose hardware names it. */
  itemCount: number;
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}

/** A model of one manufacturer. Deleting only sets deletedAt. */
export interface HardwareModel {
  id: string;
  manufacturerId: string;
  name: string;
  /** Live items whose hardware names it. */
  itemCount: number;
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}

export interface NetworkHost {
  role: NetworkRole;
  hostname: string;
}

export interface Purchase {
  supplier: string;
  invoiceNumber: string;
  /** Decimal with two places ("12.50"); null when unknown. */
  price: string | null;
  /** ISO 4217; empty without a price. */
  currency: string;
  warrantyEnd: string | null;
}

/** A SIM card; every field may be empty. Phone number, ICCID and IMSI come back without separators. */
export interface SimCard {
  provider: string;
  phoneNumber: string;
  iccid: string;
  imsi: string;
}

/** Mobile network generations a WWAN modem connects with; empty means unknown. */
export const wwanTechnologies = ['2g', '3g', '4g', '5g'] as const;
export type WWANTechnology = (typeof wwanTechnologies)[number];

/** A mobile (cellular) connection; every field may be empty. The IMEI comes back without separators. */
export interface WWAN {
  imei: string;
  apn: string;
  technology: WWANTechnology | '';
  /** Another storage item, carrying a SIM card module, used as the SIM card. */
  simCardItemId: string | null;
  /** The item simCardItemId names (responses only). */
  simCardItem?: StorageItemRef | null;
}

/** The operating system installed on a device; both fields may be empty. */
export interface OperatingSystem {
  name: string;
  version: string;
}

/** One module instance; exactly the payload matching kind is set. */
export interface Module {
  id: string;
  kind: ModuleKind;
  primary: boolean;
  hardware?: Hardware;
  networkHost?: NetworkHost;
  purchase?: Purchase;
  simCard?: SimCard;
  wwan?: WWAN;
  operatingSystem?: OperatingSystem;
  networkInterface?: NetworkInterface;
}

/** A module as sent: id keeps an existing instance, leave it out for a new one. */
export interface ModuleInput {
  id?: string;
  kind: ModuleKind;
  primary?: boolean;
  hardware?: HardwareInput;
  networkHost?: NetworkHost;
  purchase?: Omit<Purchase, 'price'> & { price: string | null };
  simCard?: SimCard;
  wwan?: Omit<WWAN, 'simCardItem'>;
  operatingSystem?: OperatingSystem;
  networkInterface?: NetworkInterfaceInput;
}

/** A thing kept in a storage. Deleting only sets deletedAt. */
export interface StorageItem {
  id: string;
  /** Unique ignoring case, never changes. */
  assetTag: string;
  name: string;
  description: string;
  storageId: string;
  type: ItemTypeRef;
  usable: boolean;
  orderDate: string | null;
  commissionedAt: string | null;
  decommissionedAt: string | null;
  decommissioningReason: string;
  personResponsible: User | null;
  /** In display order: by kind, primary first, then by name. */
  modules: Module[];
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}

export interface CreateStorageItemInput {
  assetTag: string;
  name: string;
  description?: string;
  storageId: string;
  typeId: string;
  usable: boolean;
  orderDate?: string | null;
  commissionedAt?: string | null;
  decommissionedAt?: string | null;
  decommissioningReason?: string;
  personResponsibleId?: string | null;
  modules?: ModuleInput[];
}

/** Partial update; null clears a date or the responsible person, modules replaces all instances. */
export type UpdateStorageItemInput = Partial<Omit<CreateStorageItemInput, 'assetTag'>>;

/** Narrows an item list. */
export interface StorageItemFilter {
  /** Only when listing a storage: include items anywhere below it. */
  recursive?: boolean;
  /** Only deleted items instead of only live ones. */
  deleted?: boolean;
  /** Name, description, asset tag, hardware fields, host name or MAC contains this, ignoring case. */
  q?: string;
  typeId?: string;
  includeSubtypes?: boolean;
  usable?: boolean;
  responsibleId?: string;
  /** With (true) or without (false) a decommissioning date. */
  decommissioned?: boolean;
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
  /** IDs of storage items to link; they must not be deleted. */
  storageItems?: string[];
}

export type UpdateIssueInput = Partial<CreateIssueInput>;
