import Button from '@mui/material/Button';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useTags, useUsers } from '../api/hooks';
import { issueStatuses, type CreateIssueInput, type IssueStatus } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { MarkdownEditor } from '../components/MarkdownEditor';
import { TagInput } from '../components/TagInput';
import { UserPicker } from '../components/UserPicker';
import { fieldErrorText } from '../lib/errors';

export interface IssueFormValues {
  title: string;
  description: string;
  status: IssueStatus;
  tags: string[];
  assignees: string[];
}

interface IssueFormProps {
  projectKey: string;
  initial: IssueFormValues;
  /** Hide the status select (the issue page changes status separately). */
  withStatus: boolean;
  submitLabel: string;
  pending: boolean;
  error: unknown;
  onSubmit: (values: CreateIssueInput) => void;
  onCancel: () => void;
}

/** Shared form for creating and editing issues. */
export function IssueForm({ projectKey, initial, withStatus, submitLabel, pending, error, onSubmit, onCancel }: IssueFormProps) {
  const { t } = useTranslation();
  const tags = useTags(projectKey);
  const users = useUsers();
  const [values, setValues] = useState(initial);

  const set = <K extends keyof IssueFormValues>(k: K, v: IssueFormValues[K]) => setValues((prev) => ({ ...prev, [k]: v }));
  const fieldError = (field: string) => fieldErrorText(t, error, field);

  function submit(e: FormEvent) {
    e.preventDefault();
    onSubmit(withStatus ? values : { ...values, status: undefined });
  }

  return (
    <Stack component="form" onSubmit={submit} spacing={3} sx={{ maxWidth: '44rem' }}>
      <TextField
        id="title"
        label={t('issues.title')}
        value={values.title}
        onChange={(e) => set('title', e.target.value)}
        required
        autoFocus
        error={!!fieldError('title')}
        helperText={fieldError('title')}
        slotProps={{ htmlInput: { maxLength: 200 } }}
      />

      <MarkdownEditor
        id="description"
        label={`${t('issues.description')} (${t('app.optional')})`}
        value={values.description}
        onChange={(v) => set('description', v)}
        error={fieldError('description')}
      />

      {withStatus && (
        <TextField
          select
          id="status"
          label={t('issues.status')}
          value={values.status}
          onChange={(e) => set('status', e.target.value as IssueStatus)}
          error={!!fieldError('status')}
          helperText={fieldError('status')}
          sx={{ maxWidth: '16rem' }}
        >
          {issueStatuses.map((s) => (
            <MenuItem key={s} value={s}>
              {t(`status.${s}`)}
            </MenuItem>
          ))}
        </TextField>
      )}

      <TagInput
        id="tags"
        label={t('issues.tags')}
        value={values.tags}
        onChange={(v) => set('tags', v)}
        tags={tags.data ?? []}
        error={fieldError('tags')}
      />

      {users.data && (
        <UserPicker
          id="assignees"
          label={t('issues.assignees')}
          users={users.data}
          value={values.assignees}
          onChange={(v) => set('assignees', v)}
          error={fieldError('assignees')}
        />
      )}

      <ErrorMessage error={error} />

      <Stack direction="row" spacing={1}>
        <Button type="submit" variant="contained" loading={pending}>
          {submitLabel}
        </Button>
        <Button onClick={onCancel}>{t('app.cancel')}</Button>
      </Stack>
    </Stack>
  );
}
