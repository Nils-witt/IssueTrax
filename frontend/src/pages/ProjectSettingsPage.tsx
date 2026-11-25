import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useProject, useTags, useUpdateProject, useUpdateTag } from '../api/hooks';
import type { Project, Tag } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { ErrorMessage } from '../components/ErrorMessage';
import { MarkdownEditor } from '../components/MarkdownEditor';
import { fieldErrorText } from '../lib/errors';
import { tagChipSx } from '../lib/tagColor';

export function ProjectSettingsPage() {
  const { t } = useTranslation();
  const key = useParam('key');
  const project = useProject(key);
  const tags = useTags(key);

  return (
    <>
      <ProjectNav projectKey={key} />
      <Typography variant="h5" component="h2" sx={{ mb: 3 }}>
        {t('projects.settingsTitle')}
      </Typography>
      {project.data && <ProjectForm key={project.data.updatedAt} project={project.data} />}

      <Typography variant="h5" component="h2" sx={{ mt: 5, mb: 2 }}>
        {t('issues.tags')}
      </Typography>
      <ErrorMessage error={tags.error} />
      {tags.data && tags.data.length === 0 && <Typography color="text.secondary">{t('tags.none')}</Typography>}
      <Stack component="ul" spacing={1.5} sx={{ listStyle: 'none', p: 0, m: 0 }}>
        {tags.data?.map((tag) => (
          <TagRow key={`${tag.id}:${tag.name}:${tag.color}`} projectKey={key} tag={tag} />
        ))}
      </Stack>
    </>
  );
}

function ProjectForm({ project }: { project: Project }) {
  const { t } = useTranslation();
  const update = useUpdateProject(project.key);
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description);

  const fieldError = (field: string) => fieldErrorText(t, update.error, field);

  function submit(e: FormEvent) {
    e.preventDefault();
    update.mutate({ name, description });
  }

  return (
    <Stack component="form" onSubmit={submit} spacing={3} sx={{ maxWidth: '44rem' }}>
      <TextField id="key" label={t('projects.key')} value={project.key} disabled helperText={t('fieldErrors.immutable')} />
      <TextField
        id="name"
        label={t('projects.name')}
        value={name}
        onChange={(e) => setName(e.target.value)}
        required
        error={!!fieldError('name')}
        helperText={fieldError('name')}
        slotProps={{ htmlInput: { maxLength: 100 } }}
      />
      <MarkdownEditor
        id="description"
        label={t('projects.description')}
        value={description}
        onChange={setDescription}
        error={fieldError('description')}
      />
      <ErrorMessage error={update.error} />
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        <Button type="submit" variant="contained" loading={update.isPending}>
          {t('app.save')}
        </Button>
        {update.isSuccess && (
          <Alert severity="success" role="status" sx={{ py: 0 }}>
            {t('projects.saved')}
          </Alert>
        )}
      </Stack>
    </Stack>
  );
}

function TagRow({ projectKey, tag }: { projectKey: string; tag: Tag }) {
  const { t } = useTranslation();
  const update = useUpdateTag(projectKey);
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(tag.color);

  const nameError = fieldErrorText(t, update.error, 'name');
  const colorError = fieldErrorText(t, update.error, 'color');
  const changed = name !== tag.name || color !== tag.color;

  function submit(e: FormEvent) {
    e.preventDefault();
    update.mutate({
      id: tag.id,
      ...(name !== tag.name && { name }),
      ...(color !== tag.color && { color }),
    });
  }

  return (
    <li>
      <Stack component="form" onSubmit={submit} direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
        <TextField
          size="small"
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={!!nameError}
          helperText={nameError}
          slotProps={{ htmlInput: { maxLength: 30, 'aria-label': t('tags.name') } }}
        />
        <TextField
          size="small"
          type="color"
          value={color}
          onChange={(e) => setColor(e.target.value)}
          error={!!colorError}
          helperText={colorError}
          sx={{ width: '4.5rem' }}
          slotProps={{ htmlInput: { 'aria-label': t('tags.color'), style: { padding: 4, height: 32, cursor: 'pointer' } } }}
        />
        <Chip label={name || tag.name} sx={[{ mt: 0.5 }, tagChipSx(color)].flat()} />
        <Button type="submit" variant="outlined" loading={update.isPending} disabled={!changed}>
          {t('app.save')}
        </Button>
        <ErrorMessage error={update.error && !nameError && !colorError ? update.error : null} />
      </Stack>
    </li>
  );
}
