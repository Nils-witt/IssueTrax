import Alert from '@mui/material/Alert';
import { useTranslation } from 'react-i18next';
import { errorText } from '../lib/errors';

export function ErrorMessage({ error }: { error: unknown }) {
  const { t } = useTranslation();

  if (!error) return null;

  return <Alert severity="error">{errorText(t, error)}</Alert>;
}
