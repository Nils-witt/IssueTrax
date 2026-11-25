import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { useCreateProject } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';
import { MarkdownEditor } from '../components/MarkdownEditor';
import { fieldErrorText } from '../lib/errors';

export function NewProjectPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const create = useCreateProject();

  const [key, setKey] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const fieldError = (field: string) => fieldErrorText(t, create.error, field);

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(
      { key, name, description },
      { onSuccess: (project) => void navigate(`/p/${project.key}`) },
    );
  }

  return (
    <Stack component="form" onSubmit={submit} spacing={3} sx={{ maxWidth: '44rem' }}>
      <Typography variant="h4" component="h1">
        {t('projects.new')}
      </Typography>

      <TextField
        id="key"
        label={t('projects.key')}
        value={key}
        onChange={(e) => setKey(e.target.value.toUpperCase())}
        required
        autoComplete="off"
        error={!!fieldError('key')}
        helperText={fieldError('key') ?? t('projects.keyHint')}
        slotProps={{ htmlInput: { maxLength: 10 } }}
      />

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
        label={`${t('projects.description')} (${t('app.optional')})`}
        value={description}
        onChange={setDescription}
        error={fieldError('description')}
      />

      <ErrorMessage error={create.error} />

      <Stack direction="row" spacing={1}>
        <Button type="submit" variant="contained" loading={create.isPending}>
          {t('app.create')}
        </Button>
        <Button href="/">{t('app.cancel')}</Button>
      </Stack>
    </Stack>
  );
}
