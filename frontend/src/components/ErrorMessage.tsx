import { useTranslation } from 'react-i18next';
import { errorText } from '../lib/errors';

export function ErrorMessage({ error }: { error: unknown }) {
  const { t } = useTranslation();

  if (!error) return null;

  return (
    <p role="alert" className="error">
      {errorText(t, error)}
    </p>
  );
}
