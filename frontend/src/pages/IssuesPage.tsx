import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { useIssues, useTags, useUsers } from '../api/hooks';
import { issueStatuses, type Issue, type MatchMode } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { StatusBadge } from '../components/StatusBadge';

// The page's URL query uses the API's parameter names, so a filtered view
// is a bookmarkable URL and maps 1:1 onto GET /api/projects/{key}/issues.
const filterParams = ['status', 'tag', 'tagMode', 'assignee', 'assigneeMode', 'q'] as const;

export function IssuesPage() {
  const { t } = useTranslation();
  const key = useParam('key');
  const [params] = useSearchParams();

  const filterQuery = new URLSearchParams(
    [...params].filter(([name]) => (filterParams as readonly string[]).includes(name)),
  ).toString();

  const issues = useIssues(key, filterQuery);
  const items = issues.data?.pages.flatMap((p) => p.items) ?? [];
  const total = issues.data?.pages[0]?.total ?? 0;

  return (
    <>
      <ProjectNav projectKey={key} />

      <div className="page-head">
        <FilterBar projectKey={key} />
        <Link to={`/p/${key}/issues/new`} className="button primary">
          {t('issues.new')}
        </Link>
      </div>

      <ErrorMessage error={issues.error} />
      {issues.isPending && <p className="muted">{t('app.loading')}</p>}
      {issues.isSuccess && (
        <>
          <p className="muted">{t('issues.count', { count: total })}</p>
          {items.length === 0 ? <p>{t('issues.empty')}</p> : <IssueTable issues={items} />}
          {issues.hasNextPage && (
            <button type="button" onClick={() => void issues.fetchNextPage()} disabled={issues.isFetchingNextPage}>
              {issues.isFetchingNextPage ? t('app.loading') : t('app.loadMore')}
            </button>
          )}
        </>
      )}
    </>
  );
}

function IssueTable({ issues }: { issues: Issue[] }) {
  const { t } = useTranslation();

  return (
    <ul className="issue-list">
      {issues.map((i) => (
        <li key={i.id}>
          <Link to={`/p/${i.projectKey}/issues/${i.number}`} className="issue-row">
            <span className="issue-key">{i.key}</span>
            <span className="issue-title">{i.title}</span>
            <StatusBadge status={i.status} />
            <span className="chips">
              {i.tags.map((tag) => (
                <span key={tag.id} className="chip">
                  {tag.name}
                </span>
              ))}
            </span>
            <span className="muted">{i.assignees.map((a) => a.displayName).join(', ') || t('issues.nobody')}</span>
            <span className="muted">
              <DateTime value={i.updatedAt} />
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function FilterBar({ projectKey }: { projectKey: string }) {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const tags = useTags(projectKey);
  const users = useUsers();

  const [q, setQ] = useState(params.get('q') ?? '');

  function update(mutate: (p: URLSearchParams) => void) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        mutate(next);
        return next;
      },
      { replace: true },
    );
  }

  function toggle(name: string, value: string) {
    update((p) => {
      const values = p.getAll(name);
      p.delete(name);

      for (const v of values.includes(value) ? values.filter((x) => x !== value) : [...values, value]) {
        p.append(name, v);
      }
    });
  }

  // Debounce the search box before touching the URL (and the API).
  useEffect(() => {
    const timer = setTimeout(() => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (q.trim()) next.set('q', q.trim());
          else next.delete('q');
          return next;
        },
        { replace: true },
      );
    }, 300);

    return () => clearTimeout(timer);
  }, [q, setParams]);

  const hasFilters = filterParams.some((name) => params.has(name));

  return (
    <div className="filters">
      <input
        type="search"
        aria-label={t('filters.search')}
        placeholder={t('filters.search')}
        value={q}
        maxLength={200}
        onChange={(e) => setQ(e.target.value)}
      />

      <fieldset className="checks" title={t('filters.statusHint')}>
        <legend>{t('filters.statuses')}</legend>
        {issueStatuses.map((s) => (
          <label key={s}>
            <input type="checkbox" checked={params.getAll('status').includes(s)} onChange={() => toggle('status', s)} />
            {t(`status.${s}`)}
          </label>
        ))}
      </fieldset>

      {tags.data && tags.data.length > 0 && (
        <fieldset className="checks">
          <legend>{t('filters.tags')}</legend>
          {tags.data.map((tag) => (
            <label key={tag.id}>
              <input type="checkbox" checked={params.getAll('tag').includes(tag.id)} onChange={() => toggle('tag', tag.id)} />
              {tag.name}
            </label>
          ))}
          <MatchSelect name="tagMode" value={params.get('tagMode')} onChange={(m) => update((p) => p.set('tagMode', m))} />
        </fieldset>
      )}

      {users.data && users.data.length > 0 && (
        <fieldset className="checks">
          <legend>{t('filters.assignees')}</legend>
          {users.data.map((u) => (
            <label key={u.id}>
              <input type="checkbox" checked={params.getAll('assignee').includes(u.id)} onChange={() => toggle('assignee', u.id)} />
              {u.displayName}
            </label>
          ))}
          <MatchSelect name="assigneeMode" value={params.get('assigneeMode')} onChange={(m) => update((p) => p.set('assigneeMode', m))} />
        </fieldset>
      )}

      {hasFilters && (
        <button
          type="button"
          className="link"
          onClick={() => {
            setQ('');
            update((p) => filterParams.forEach((name) => p.delete(name)));
          }}
        >
          {t('filters.clear')}
        </button>
      )}
    </div>
  );
}

function MatchSelect({ name, value, onChange }: { name: string; value: string | null; onChange: (m: MatchMode) => void }) {
  const { t } = useTranslation();

  return (
    <label className="match">
      {t('filters.match')}{' '}
      <select name={name} value={value === 'any' ? 'any' : 'all'} onChange={(e) => onChange(e.target.value as MatchMode)}>
        <option value="all">{t('filters.matchAll')}</option>
        <option value="any">{t('filters.matchAny')}</option>
      </select>
    </label>
  );
}
