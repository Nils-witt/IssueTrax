import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useIssue, useUpdateIssue } from '../api/hooks';
import { issueStatuses, type CreateIssueInput, type Issue, type IssueStatus, type UpdateIssueInput } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { Markdown } from '../components/Markdown';
import { StatusBadge } from '../components/StatusBadge';
import { IssueForm } from './IssueForm';

export function IssuePage() {
  const { t } = useTranslation();
  const key = useParam('key');
  const number = Number(useParam('number'));
  const issue = useIssue(key, number);

  return (
    <>
      <ProjectNav projectKey={key} />
      <ErrorMessage error={issue.error} />
      {issue.isPending && <p className="muted">{t('app.loading')}</p>}
      {issue.data && <IssueDetail key={issue.data.updatedAt} issue={issue.data} />}
    </>
  );
}

function IssueDetail({ issue }: { issue: Issue }) {
  const { t } = useTranslation();
  const update = useUpdateIssue(issue.projectKey, issue.number);
  const [editing, setEditing] = useState(false);
  const archived = issue.status === 'archived';

  // Only send what changed, so the archived rule and partial updates hold.
  function save(values: CreateIssueInput) {
    const changes: UpdateIssueInput = {};

    if (values.title !== issue.title) changes.title = values.title;
    if (values.description !== issue.description) changes.description = values.description;
    if (!sameSet(values.tags ?? [], issue.tags.map((tag) => tag.name))) changes.tags = values.tags;
    if (!sameSet(values.assignees ?? [], issue.assignees.map((a) => a.id))) changes.assignees = values.assignees;

    if (Object.keys(changes).length === 0) {
      setEditing(false);
      return;
    }

    update.mutate(changes, { onSuccess: () => setEditing(false) });
  }

  if (editing) {
    return (
      <>
        <h2>{issue.key}</h2>
        <IssueForm
          projectKey={issue.projectKey}
          initial={{
            title: issue.title,
            description: issue.description,
            status: issue.status,
            tags: issue.tags.map((tag) => tag.name),
            assignees: issue.assignees.map((a) => a.id),
          }}
          withStatus={false}
          submitLabel={t('app.save')}
          pending={update.isPending}
          error={update.error}
          onSubmit={save}
          onCancel={() => setEditing(false)}
        />
      </>
    );
  }

  return (
    <article className="issue">
      {archived && (
        <div className="banner" role="status">
          {t('issues.archivedBanner')}{' '}
          <button type="button" onClick={() => update.mutate({ status: 'open' })} disabled={update.isPending}>
            {t('issues.unarchive')}
          </button>
        </div>
      )}

      <div className="page-head">
        <h2>
          <span className="issue-key">{issue.key}</span> {issue.title}
        </h2>
        <button type="button" onClick={() => setEditing(true)} disabled={archived}>
          {t('app.edit')}
        </button>
      </div>

      <ErrorMessage error={update.error} />

      <div className="issue-layout">
        <section className="issue-body">
          {issue.description ? <Markdown>{issue.description}</Markdown> : <p className="muted">{t('issues.noDescription')}</p>}
        </section>

        <aside className="issue-meta">
          <label>
            {t('issues.status')}
            <select
              value={issue.status}
              disabled={update.isPending}
              onChange={(e) => update.mutate({ status: e.target.value as IssueStatus })}
            >
              {issueStatuses.map((s) => (
                <option key={s} value={s}>
                  {t(`status.${s}`)}
                </option>
              ))}
            </select>
          </label>
          <StatusBadge status={issue.status} />

          <h3>{t('issues.tags')}</h3>
          <p className="chips">
            {issue.tags.length === 0 && <span className="muted">—</span>}
            {issue.tags.map((tag) => (
              <Link key={tag.id} className="chip" to={`/p/${issue.projectKey}?tag=${tag.id}`}>
                {tag.name}
              </Link>
            ))}
          </p>

          <h3>{t('issues.assignees')}</h3>
          <p>{issue.assignees.map((a) => a.displayName).join(', ') || <span className="muted">{t('issues.nobody')}</span>}</p>

          <p className="muted small">
            {t('issues.createdBy', { name: issue.createdBy.displayName || '?' })}
            <br />
            {t('issues.created')} <DateTime value={issue.createdAt} />
            <br />
            {t('issues.updated')} <DateTime value={issue.updatedAt} />
          </p>
        </aside>
      </div>
    </article>
  );
}

function sameSet(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((x) => b.includes(x));
}
