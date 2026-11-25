import { useTranslation } from 'react-i18next';
import { Link, useLocation } from 'react-router';
import { useProject } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';

/** Project title with Issues / Settings tabs, shown on project pages. */
export function ProjectNav({ projectKey }: { projectKey: string }) {
  const { t } = useTranslation();
  const project = useProject(projectKey);
  const onSettings = useLocation().pathname.endsWith('/settings');

  if (project.isError) return <ErrorMessage error={project.error} />;

  const key = project.data?.key ?? projectKey.toUpperCase();

  return (
    <div className="project-nav">
      <h1>
        <span className="project-key">{key}</span> {project.data?.name}
      </h1>
      <nav>
        {/* "Issues" stays active on the list, issue and new-issue pages. */}
        <Link to={`/p/${key}`} className={onSettings ? undefined : 'active'} aria-current={onSettings ? undefined : 'page'}>
          {t('issues.tab')}
        </Link>
        <Link to={`/p/${key}/settings`} className={onSettings ? 'active' : undefined} aria-current={onSettings ? 'page' : undefined}>
          {t('projects.settings')}
        </Link>
      </nav>
    </div>
  );
}
