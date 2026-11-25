import { useTranslation } from 'react-i18next';
import { Link, useRouteError } from 'react-router';
import { ErrorMessage } from '../components/ErrorMessage';

/** Shown when a route throws while rendering. */
export function ErrorPage() {
  const error = useRouteError();

  return (
    <main>
      <ErrorMessage error={error} />
      <HomeLink />
    </main>
  );
}

export function NotFoundPage() {
  const { t } = useTranslation();

  return (
    <>
      <p>{t('app.notFound')}</p>
      <HomeLink />
    </>
  );
}

function HomeLink() {
  const { t } = useTranslation();

  return <Link to="/">{t('app.goHome')}</Link>;
}
