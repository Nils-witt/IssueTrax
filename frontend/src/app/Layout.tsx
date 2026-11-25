import { useTranslation } from 'react-i18next';
import { Link, Outlet } from 'react-router';
import { signOut } from '../api/client';
import { useMe } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';
import { languages, type Language } from '../i18n';

/**
 * App frame. Every page needs a session: without one the API answers 401
 * and the client redirects to /auth/login, so pages only render signed in.
 */
export function Layout() {
  const { t, i18n } = useTranslation();
  const me = useMe();

  return (
    <>
      <header className="topbar">
        <Link to="/" className="brand">
          {t('app.name')}
        </Link>
        <div className="topbar-right">
          <label className="lang">
            <span className="sr-only">{t('app.language')}</span>
            <select
              value={i18n.resolvedLanguage}
              onChange={(e) => void i18n.changeLanguage(e.target.value as Language)}
            >
              {languages.map((l) => (
                <option key={l} value={l}>
                  {l.toUpperCase()}
                </option>
              ))}
            </select>
          </label>
          {me.data && (
            <>
              <span className="user" title={me.data.email}>
                {me.data.displayName}
              </span>
              <button type="button" className="link" onClick={() => void signOut()}>
                {t('app.signOut')}
              </button>
            </>
          )}
        </div>
      </header>
      <main>
        {me.isPending && <p className="muted">{t('app.loading')}</p>}
        {me.isError && <ErrorMessage error={me.error} />}
        {me.isSuccess && <Outlet />}
      </main>
    </>
  );
}
