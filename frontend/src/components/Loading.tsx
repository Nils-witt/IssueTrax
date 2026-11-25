import Box from '@mui/material/Box';
import CircularProgress from '@mui/material/CircularProgress';
import { useTranslation } from 'react-i18next';

export function Loading() {
  const { t } = useTranslation();

  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
      <CircularProgress size={28} aria-label={t('app.loading')} />
    </Box>
  );
}
