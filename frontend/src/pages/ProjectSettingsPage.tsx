import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useProject, useRenameTag, useTags, useUpdateProject } from '../api/hooks';
import type { Project, Tag } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { ErrorMessage } from '../components/ErrorMessage';
import { Field } from '../components/Field';
import { MarkdownEditor } from '../components/MarkdownEditor';
import { fieldErrorText } from '../lib/errors';

export function ProjectSettingsPage() {
  const { t } = useTranslation();
  const key = useParam('key');
  const project = useProject(key);
  const tags = useTags(key);

  return (
    <>
      <ProjectNav projectKey={key} />
      <h2>{t('projects.settingsTitle')}</h2>
      {project.data && <ProjectForm key={project.data.updatedAt} project={project.data} />}

      <h2>{t('issues.tags')}</h2>
      <ErrorMessage error={tags.error} />
      {tags.data && tags.data.length === 0 && <p className="muted">{t('tags.none')}</p>}
      <ul className="tag-admin">
        {tags.data?.map((tag) => (
          <TagRow key={`${tag.id}:${tag.name}`} projectKey={key} tag={tag} />
        ))}
      </ul>
    </>
  );
}

function ProjectForm({ project }: { project: Project }) {
  const { t } = useTranslation();
  const update = useUpdateProject(project.key);
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description);

  function submit(e: FormEvent) {
    e.preventDefault();
    update.mutate({ name, description });
  }

  return (
    <form onSubmit={submit} className="form">
      <Field id="key" label={t('projects.key')} hint={t('fieldErrors.immutable')}>
        <input id="key" value={project.key} disabled />
      </Field>
      <Field id="name" label={t('projects.name')} error={fieldErrorText(t, update.error, 'name')}>
        <input id="name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
      </Field>
      <Field id="description" label={t('projects.description')} error={fieldErrorText(t, update.error, 'description')}>
        <MarkdownEditor id="description" value={description} onChange={setDescription} />
      </Field>
      <ErrorMessage error={update.error} />
      <div className="actions">
        <button type="submit" className="primary" disabled={update.isPending}>
          {update.isPending ? t('app.saving') : t('app.save')}
        </button>
        {update.isSuccess && <span role="status">{t('projects.saved')}</span>}
      </div>
    </form>
  );
}

function TagRow({ projectKey, tag }: { projectKey: string; tag: Tag }) {
  const { t } = useTranslation();
  const rename = useRenameTag(projectKey);
  const [name, setName] = useState(tag.name);

  function submit(e: FormEvent) {
    e.preventDefault();
    rename.mutate({ id: tag.id, name });
  }

  return (
    <li>
      <form onSubmit={submit} className="inline-form">
        <input aria-label={t('tags.rename')} value={name} onChange={(e) => setName(e.target.value)} maxLength={30} />
        <button type="submit" disabled={rename.isPending || name === tag.name}>
          {t('tags.rename')}
        </button>
        <span className="field-error">{fieldErrorText(t, rename.error, 'name')}</span>
        <ErrorMessage error={rename.error && !fieldErrorText(t, rename.error, 'name') ? rename.error : null} />
      </form>
    </li>
  );
}
