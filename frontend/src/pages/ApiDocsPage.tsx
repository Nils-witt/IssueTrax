import Button from '@mui/material/Button';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useTranslation } from 'react-i18next';
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

/** Served by the backend from docs/openapi.yaml. */
export const openApiUrl = '/api/openapi.yaml';

/**
 * Interactive API reference. Loaded lazily: Swagger UI is large and only
 * needed here. "Try it out" calls go same-origin with the session cookie.
 */
export function ApiDocsPage() {
  const { t } = useTranslation();

  return (
    <Stack spacing={2}>
      <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography variant="h4" component="h1">
          {t('apiDocs.title')}
        </Typography>
        <Button href={openApiUrl} download="issuetrax-openapi.yaml" variant="outlined" size="small">
          {t('apiDocs.download')}
        </Button>
      </Stack>
      {/* Swagger UI only has a light theme, so keep it on a light surface. */}
      <Paper variant="outlined" sx={{ bgcolor: '#fff', color: '#3b4151', colorScheme: 'light', overflow: 'hidden' }}>
        <SwaggerUI url={openApiUrl} docExpansion="list" />
      </Paper>
    </Stack>
  );
}
