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
