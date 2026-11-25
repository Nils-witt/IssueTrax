import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useProjects } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';

export function ProjectsPage() {
  const { t } = useTranslation();
  const projects = useProjects();

  const items = projects.data?.pages.flatMap((p) => p.items) ?? [];
  const total = projects.data?.pages[0]?.total ?? 0;

  return (
    <>
      <div className="page-head">
        <h1>{t('projects.title')}</h1>
        <Link to="/projects/new" className="button primary">
          {t('projects.new')}
        </Link>
      </div>

      <ErrorMessage error={projects.error} />
      {projects.isPending && <p className="muted">{t('app.loading')}</p>}
      {projects.isSuccess && (
        <>
          <p className="muted">{t('projects.count', { count: total })}</p>
          {items.length === 0 ? (
            <p>{t('projects.empty')}</p>
          ) : (
            <ul className="cards">
              {items.map((p) => (
                <li key={p.id}>
                  <Link to={`/p/${p.key}`} className="card">
                    <span className="project-key">{p.key}</span>
                    <strong>{p.name}</strong>
                    {p.description && <span className="muted clamp">{p.description}</span>}
                  </Link>
                </li>
              ))}
            </ul>
          )}
          {projects.hasNextPage && (
            <button type="button" onClick={() => void projects.fetchNextPage()} disabled={projects.isFetchingNextPage}>
              {projects.isFetchingNextPage ? t('app.loading') : t('app.loadMore')}
            </button>
          )}
        </>
      )}
    </>
  );
}
