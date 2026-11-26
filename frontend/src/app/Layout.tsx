import AppBar from '@mui/material/AppBar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Container from '@mui/material/Container';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import { useTranslation } from 'react-i18next';
import { Link, Outlet } from 'react-router';
import { signOut } from '../api/client';
import { useMe, useVersion } from '../api/hooks';
import { useLiveUpdates } from '../api/live';
import { ErrorMessage } from '../components/ErrorMessage';
import { Loading } from '../components/Loading';
import { languages, type Language } from '../i18n';

/**
 * App frame. Every page needs a session: without one the API answers 401
 * and the client redirects to /auth/login, so pages only render signed in.
 */
export function Layout() {
  const { t, i18n } = useTranslation();
  const me = useMe();
  useLiveUpdates(me.isSuccess);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <AppBar position="static" color="default" elevation={0} sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Toolbar variant="dense" sx={{ gap: 2 }}>
          <Typography component={Link} to="/" variant="h6" sx={{ fontWeight: 700, color: 'inherit', textDecoration: 'none', flexGrow: 1 }}>
            {t('app.name')}
          </Typography>
          <Button component={Link} to="/service-accounts" size="small" color="inherit">
            {t('serviceAccounts.nav')}
          </Button>
          {me.data?.admin && (
            <Button component={Link} to="/audit-log" size="small" color="inherit">
              {t('auditLog.nav')}
            </Button>
          )}
          <Button component={Link} to="/api-docs" size="small" color="inherit">
            {t('app.apiDocs')}
          </Button>
          <Select
            size="small"
            variant="standard"
            disableUnderline
            value={i18n.resolvedLanguage ?? languages[0]}
            onChange={(e) => void i18n.changeLanguage(e.target.value as Language)}
            inputProps={{ 'aria-label': t('app.language') }}
          >
            {languages.map((l) => (
              <MenuItem key={l} value={l}>
                {l.toUpperCase()}
              </MenuItem>
            ))}
          </Select>
          {me.data && (
            <>
              <Typography variant="body2" title={me.data.email}>
                {me.data.displayName}
              </Typography>
              <Button size="small" color="inherit" onClick={() => void signOut()}>
                {t('app.signOut')}
              </Button>
            </>
          )}
        </Toolbar>
      </AppBar>
      <Container component="main" maxWidth="lg" sx={{ pt: 3, pb: 8, flexGrow: 1 }}>
        {me.isPending && <Loading />}
        {me.isError && <ErrorMessage error={me.error} />}
        {me.isSuccess && <Outlet />}
      </Container>
      <Footer />
    </Box>
  );
}

/**
 * Shows the build the server runs: the git tag (or `git describe` output
 * such as v1.2.0-3-gabc1234), falling back to the commit for untagged builds.
 */
function Footer() {
  const { data } = useVersion();
  if (!data) return null;

  const label = data.version !== 'dev' ? data.version : data.commit !== 'unknown' ? data.commit : 'dev';

  return (
    <Box component="footer" sx={{ borderTop: 1, borderColor: 'divider', py: 1.5, textAlign: 'center' }}>
      <Typography variant="caption" color="text.secondary" title={`commit ${data.commit}`} sx={{ fontFamily: 'monospace' }}>
        {label}
      </Typography>
    </Box>
  );
}
