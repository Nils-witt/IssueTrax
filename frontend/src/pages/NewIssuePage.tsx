import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useCreateIssue } from '../api/hooks';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { IssueForm } from './IssueForm';

export function NewIssuePage() {
  const { t } = useTranslation();
  const key = useParam('key');
  const navigate = useNavigate();
  const create = useCreateIssue(key);

  return (
    <>
      <ProjectNav projectKey={key} />
      <h2>{t('issues.new')}</h2>
      <IssueForm
        projectKey={key}
        initial={{ title: '', description: '', status: 'open', tags: [], assignees: [] }}
        withStatus
        submitLabel={t('app.create')}
        pending={create.isPending}
        error={create.error}
        onSubmit={(values) =>
          create.mutate(values, { onSuccess: (issue) => void navigate(`/p/${issue.projectKey}/issues/${issue.number}`) })
        }
        onCancel={() => void navigate(`/p/${key}`)}
      />
    </>
  );
}
