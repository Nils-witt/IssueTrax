// Wire types of the IssueTrax JSON API (see docs/openapi.yaml).

export const issueStatuses = ['draft', 'open', 'closed', 'archived'] as const;
export type IssueStatus = (typeof issueStatuses)[number];

export type MatchMode = 'all' | 'any';

export interface User {
  id: string;
  email: string;
  displayName: string;
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
