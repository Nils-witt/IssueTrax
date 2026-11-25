import Box from '@mui/material/Box';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import Typography from '@mui/material/Typography';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router';
import { useProject } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';

/** Project title with Issues / Settings tabs, shown on project pages. */
export function ProjectNav({ projectKey }: { projectKey: string }) {
  const { t } = useTranslation();
  const project = useProject(projectKey);
  const onSettings = useLocation().pathname.endsWith('/settings');

  if (project.isError) return <ErrorMessage error={project.error} />;

  const key = project.data?.key ?? projectKey.toUpperCase();

  return (
    <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}>
      <Typography variant="h4" component="h1" sx={{ mb: 1 }}>
        <KeyText>{key}</KeyText> {project.data?.name}
      </Typography>
      {/* "Issues" stays active on the list, issue and new-issue pages. */}
      <Tabs value={onSettings ? 'settings' : 'issues'} component="nav">
        <Tab value="issues" label={t('issues.tab')} href={`/p/${key}`} aria-current={onSettings ? undefined : 'page'} />
        <Tab value="settings" label={t('projects.settings')} href={`/p/${key}/settings`} aria-current={onSettings ? 'page' : undefined} />
      </Tabs>
    </Box>
  );
}
