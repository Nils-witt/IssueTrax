import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useTags, useUsers } from '../api/hooks';
import { issueStatuses, type CreateIssueInput, type IssueStatus } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { Field } from '../components/Field';
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

  function submit(e: FormEvent) {
    e.preventDefault();
    onSubmit(withStatus ? values : { ...values, status: undefined });
  }

  return (
    <form onSubmit={submit} className="form">
      <Field id="title" label={t('issues.title')} error={fieldErrorText(t, error, 'title')}>
        <input id="title" value={values.title} onChange={(e) => set('title', e.target.value)} maxLength={200} required autoFocus />
      </Field>

      <Field id="description" label={`${t('issues.description')} (${t('app.optional')})`} error={fieldErrorText(t, error, 'description')}>
        <MarkdownEditor id="description" value={values.description} onChange={(v) => set('description', v)} />
      </Field>

      {withStatus && (
        <Field id="status" label={t('issues.status')} error={fieldErrorText(t, error, 'status')}>
          <select id="status" value={values.status} onChange={(e) => set('status', e.target.value as IssueStatus)}>
            {issueStatuses.map((s) => (
              <option key={s} value={s}>
                {t(`status.${s}`)}
              </option>
            ))}
          </select>
        </Field>
      )}

      <Field id="tags" label={t('issues.tags')} error={fieldErrorText(t, error, 'tags')}>
        <TagInput id="tags" value={values.tags} onChange={(v) => set('tags', v)} suggestions={tags.data?.map((tag) => tag.name) ?? []} />
      </Field>

      {users.data && (
        <div className={fieldErrorText(t, error, 'assignees') ? 'field field-invalid' : 'field'}>
          <UserPicker label={t('issues.assignees')} users={users.data} value={values.assignees} onChange={(v) => set('assignees', v)} />
        </div>
      )}

      <ErrorMessage error={error} />

      <div className="actions">
        <button type="submit" className="primary" disabled={pending}>
          {pending ? t('app.saving') : submitLabel}
        </button>
        <button type="button" className="link" onClick={onCancel}>
          {t('app.cancel')}
        </button>
      </div>
    </form>
  );
}
