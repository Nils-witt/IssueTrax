import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router';
import { useCreateProject } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';
import { Field } from '../components/Field';
import { MarkdownEditor } from '../components/MarkdownEditor';
import { fieldErrorText } from '../lib/errors';

export function NewProjectPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const create = useCreateProject();

  const [key, setKey] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(
      { key, name, description },
      { onSuccess: (project) => void navigate(`/p/${project.key}`) },
    );
  }

  return (
    <form onSubmit={submit} className="form">
      <h1>{t('projects.new')}</h1>

      <Field id="key" label={t('projects.key')} hint={t('projects.keyHint')} error={fieldErrorText(t, create.error, 'key')}>
        <input
          id="key"
          value={key}
          onChange={(e) => setKey(e.target.value.toUpperCase())}
          maxLength={10}
          autoComplete="off"
          required
        />
      </Field>

      <Field id="name" label={t('projects.name')} error={fieldErrorText(t, create.error, 'name')}>
        <input id="name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
      </Field>

      <Field id="description" label={`${t('projects.description')} (${t('app.optional')})`} error={fieldErrorText(t, create.error, 'description')}>
        <MarkdownEditor id="description" value={description} onChange={setDescription} />
      </Field>

      <ErrorMessage error={create.error} />

      <div className="actions">
        <button type="submit" className="primary" disabled={create.isPending}>
          {create.isPending ? t('app.saving') : t('app.create')}
        </button>
        <Link to="/">{t('app.cancel')}</Link>
      </div>
    </form>
  );
}
