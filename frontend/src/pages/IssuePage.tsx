import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import EditIcon from '@mui/icons-material/Edit';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useIssue, useUpdateIssue } from '../api/hooks';
import { issueStatuses, type CreateIssueInput, type Issue, type IssueStatus, type UpdateIssueInput } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { Markdown } from '../components/Markdown';
import { tagChipSx } from '../lib/tagColor';
import { IssueForm } from './IssueForm';

export function IssuePage() {
  const key = useParam('key');
  const number = Number(useParam('number'));
  const issue = useIssue(key, number);

  return (
    <>
      <ProjectNav projectKey={key} />
      <ErrorMessage error={issue.error} />
      {issue.isPending && <Loading />}
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
        <Typography variant="h5" component="h2" sx={{ mb: 3 }}>
          <KeyText>{issue.key}</KeyText>
        </Typography>
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
    <Stack component="article" spacing={2}>
      {archived && (
        <Alert
          severity="warning"
          role="status"
          action={
            <Button color="inherit" size="small" onClick={() => update.mutate({ status: 'open' })} loading={update.isPending}>
              {t('issues.unarchive')}
            </Button>
          }
        >
          {t('issues.archivedBanner')}
        </Alert>
      )}

      <Stack direction="row" spacing={2} sx={{ justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Typography variant="h5" component="h2">
          <KeyText>{issue.key}</KeyText> {issue.title}
        </Typography>
        <Button variant="outlined" startIcon={<EditIcon />} onClick={() => setEditing(true)} disabled={archived}>
          {t('app.edit')}
        </Button>
      </Stack>

      <ErrorMessage error={update.error} />

      <Box sx={{ display: 'grid', gap: 4, gridTemplateColumns: { xs: '1fr', md: '1fr 16rem' } }}>
        <Box component="section">
          {issue.description ? <Markdown>{issue.description}</Markdown> : <Typography color="text.secondary">{t('issues.noDescription')}</Typography>}
        </Box>

        <Stack component="aside" spacing={2.5}>
          <TextField
            select
            size="small"
            label={t('issues.status')}
            value={issue.status}
            disabled={update.isPending}
            onChange={(e) => update.mutate({ status: e.target.value as IssueStatus })}
          >
            {issueStatuses.map((s) => (
              <MenuItem key={s} value={s}>
                {t(`status.${s}`)}
              </MenuItem>
            ))}
          </TextField>

          <MetaSection title={t('issues.tags')}>
            {issue.tags.length === 0 ? (
              <Typography color="text.secondary">—</Typography>
            ) : (
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5 }}>
                {issue.tags.map((tag) => (
                  <Chip
                    key={tag.id}
                    size="small"
                    label={tag.name}
                    sx={tagChipSx(tag.color)}
                    component={Link}
                    to={`/p/${issue.projectKey}?tag=${tag.id}`}
                    clickable
                  />
                ))}
              </Stack>
            )}
          </MetaSection>

          <MetaSection title={t('issues.assignees')}>
            <Typography color={issue.assignees.length ? undefined : 'text.secondary'}>
              {issue.assignees.map((a) => a.displayName).join(', ') || t('issues.nobody')}
            </Typography>
          </MetaSection>

          <Typography variant="body2" color="text.secondary">
            {t('issues.createdBy', { name: issue.createdBy.displayName || '?' })}
            <br />
            {t('issues.created')} <DateTime value={issue.createdAt} />
            <br />
            {t('issues.updated')} <DateTime value={issue.updatedAt} />
          </Typography>
        </Stack>
      </Box>
    </Stack>
  );
}

function MetaSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Box>
      <Typography variant="overline" component="h3" color="text.secondary" sx={{ lineHeight: 2 }}>
        {title}
      </Typography>
      {children}
    </Box>
  );
}

function sameSet(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((x) => b.includes(x));
}
