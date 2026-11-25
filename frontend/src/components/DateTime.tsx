import { useTranslation } from 'react-i18next';

/** A timestamp formatted for the active UI language. */
export function DateTime({ value }: { value: string }) {
  const { i18n } = useTranslation();
  const date = new Date(value);

  return (
    <time dateTime={value} title={date.toISOString()}>
      {new Intl.DateTimeFormat(i18n.resolvedLanguage, { dateStyle: 'medium', timeStyle: 'short' }).format(date)}
    </time>
  );
}
