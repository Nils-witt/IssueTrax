import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Card from '@mui/material/Card';
import CardActionArea from '@mui/material/CardActionArea';
import CardContent from '@mui/material/CardContent';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import { useTranslation } from 'react-i18next';
import { useProjects } from '../api/hooks';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';

export function ProjectsPage() {
  const { t } = useTranslation();
  const projects = useProjects();

  const items = projects.data?.pages.flatMap((p) => p.items) ?? [];
  const total = projects.data?.pages[0]?.total ?? 0;

  return (
    <Stack spacing={2}>
      <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="h4" component="h1">
          {t('projects.title')}
        </Typography>
        <Button variant="contained" href="/projects/new" startIcon={<AddIcon />}>
          {t('projects.new')}
        </Button>
      </Stack>

      <ErrorMessage error={projects.error} />
      {projects.isPending && <Loading />}
      {projects.isSuccess && (
        <>
          <Typography color="text.secondary">{t('projects.count', { count: total })}</Typography>
          {items.length === 0 ? (
            <Typography>{t('projects.empty')}</Typography>
          ) : (
            <Box
              component="ul"
              sx={{ listStyle: 'none', p: 0, m: 0, display: 'grid', gap: 1.5, gridTemplateColumns: 'repeat(auto-fill, minmax(16rem, 1fr))' }}
            >
              {items.map((p) => (
                <li key={p.id}>
                  <Card variant="outlined" sx={{ height: '100%' }}>
                    <CardActionArea href={`/p/${p.key}`} sx={{ height: '100%', alignItems: 'flex-start' }}>
                      <CardContent>
                        <KeyText>{p.key}</KeyText>
                        <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                          {p.name}
                        </Typography>
                        {p.description && (
                          <Typography
                            variant="body2"
                            color="text.secondary"
                            sx={{ display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}
                          >
                            {p.description}
                          </Typography>
                        )}
                      </CardContent>
                    </CardActionArea>
                  </Card>
                </li>
              ))}
            </Box>
          )}
          {projects.hasNextPage && (
            <Box>
              <Button variant="outlined" onClick={() => void projects.fetchNextPage()} loading={projects.isFetchingNextPage}>
                {t('app.loadMore')}
              </Button>
            </Box>
          )}
        </>
      )}
    </Stack>
  );
}
