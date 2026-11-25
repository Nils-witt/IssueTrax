import Container from '@mui/material/Container';
import Link from '@mui/material/Link';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useTranslation } from 'react-i18next';
import { useRouteError } from 'react-router';
import { ErrorMessage } from '../components/ErrorMessage';

/** Shown when a route throws while rendering. */
export function ErrorPage() {
  const error = useRouteError();

  return (
    <Container component="main" maxWidth="lg" sx={{ pt: 3 }}>
      <Stack spacing={2}>
        <ErrorMessage error={error} />
        <HomeLink />
      </Stack>
    </Container>
  );
}

export function NotFoundPage() {
  const { t } = useTranslation();

  return (
    <Stack spacing={2}>
      <Typography>{t('app.notFound')}</Typography>
      <HomeLink />
    </Stack>
  );
}

function HomeLink() {
  const { t } = useTranslation();

  return <Link href="/">{t('app.goHome')}</Link>;
}
