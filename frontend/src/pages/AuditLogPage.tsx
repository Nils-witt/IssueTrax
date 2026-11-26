import Autocomplete from '@mui/material/Autocomplete';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Link from '@mui/material/Link';
import Stack from '@mui/material/Stack';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link as RouterLink } from 'react-router';
import { useAuditLog, useUsers } from '../api/hooks';
import { auditActions, type AuditAction, type AuditChange, type AuditEntry } from '../api/types';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { Loading } from '../components/Loading';

/** "TRAX-12" → the issue's path; other labels are not links. */
function issuePath(label: string): string | null {
  const m = /^([A-Z][A-Z0-9]*)-(\d+)$/.exec(label);
  return m ? `/p/${m[1]}/issues/${m[2]}` : null;
}

function Target({ entry }: { entry: AuditEntry }) {
  const path = entry.targetType === 'issue' || entry.targetType === 'comment' ? issuePath(entry.targetLabel) : null;

  if (!path) return <>{entry.targetLabel}</>;

  return (
    <Link component={RouterLink} to={path}>
      {entry.targetLabel}
    </Link>
  );
}

function Change({ field, change }: { field: string; change: AuditChange }) {
  const { t } = useTranslation();

  let text: string;
  if (change.redacted) text = t('auditLog.changed');
  else if (change.old === undefined) text = change.new ?? '';
  else text = `${change.old || '∅'} → ${change.new || '∅'}`;

  return (
    <div>
      <Typography component="span" variant="body2" sx={{ fontWeight: 600 }}>
        {field}
      </Typography>
      <Typography component="span" variant="body2" sx={{ ml: 0.5, overflowWrap: 'anywhere' }}>
        {text}
      </Typography>
    </div>
  );
}

function EntryRow({ entry }: { entry: AuditEntry }) {
  const { t } = useTranslation();
  const changes = Object.entries(entry.changes).sort(([a], [b]) => a.localeCompare(b));

  return (
    <TableRow>
      <TableCell sx={{ whiteSpace: 'nowrap' }}>
        <Tooltip title={t('auditLog.source', { ip: entry.ip || '–', userAgent: entry.userAgent || '–' })}>
          <span>
            <DateTime value={entry.at} />
          </span>
        </Tooltip>
      </TableCell>
      <TableCell>
        {entry.actor ? entry.actor.displayName || entry.actor.id : <em>{t('auditLog.unknownActor')}</em>}
        {entry.actor?.serviceAccount && <Chip size="small" label={t('serviceAccounts.bot')} sx={{ ml: 1 }} />}
      </TableCell>
      <TableCell>{t(`auditLog.actions.${entry.action}`)}</TableCell>
      <TableCell>
        <Target entry={entry} />
      </TableCell>
      <TableCell>
        {changes.map(([field, change]) => (
          <Change key={field} field={field} change={change} />
        ))}
      </TableCell>
    </TableRow>
  );
}

export function AuditLogPage() {
  const { t } = useTranslation();
  const users = useUsers();
  const [actions, setActions] = useState<AuditAction[]>([]);
  const [actor, setActor] = useState<string | null>(null);
  const log = useAuditLog({ actions, actor });

  const entries = log.data?.pages.flatMap((p) => p.items) ?? [];
  const total = log.data?.pages[0]?.total ?? 0;
  const selectedActor = users.data?.find((u) => u.id === actor) ?? null;

  return (
    <Stack spacing={2}>
      <Typography variant="h5" component="h1">
        {t('auditLog.title')}
      </Typography>
      <Typography color="text.secondary">{t('auditLog.intro')}</Typography>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
        <Autocomplete
          multiple
          size="small"
          sx={{ flex: 2 }}
          options={auditActions}
          value={actions}
          onChange={(_, next) => setActions(next)}
          getOptionLabel={(a) => t(`auditLog.actions.${a}`)}
          filterSelectedOptions
          renderInput={(params) => <TextField {...params} label={t('auditLog.filterActions')} />}
        />
        <Autocomplete
          size="small"
          sx={{ flex: 1 }}
          options={users.data ?? []}
          value={selectedActor}
          onChange={(_, next) => setActor(next?.id ?? null)}
          getOptionLabel={(u) => u.displayName}
          getOptionKey={(u) => u.id}
          isOptionEqualToValue={(a, b) => a.id === b.id}
          renderInput={(params) => <TextField {...params} label={t('auditLog.filterActor')} />}
        />
      </Stack>

      {log.isPending && <Loading />}
      {log.isError && <ErrorMessage error={log.error} />}
      {log.isSuccess && (
        <>
          <Typography variant="body2" color="text.secondary">
            {t('auditLog.count', { count: total })}
          </Typography>
          {entries.length === 0 ? (
            <Typography color="text.secondary">{t('auditLog.empty')}</Typography>
          ) : (
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>{t('auditLog.time')}</TableCell>
                    <TableCell>{t('auditLog.actor')}</TableCell>
                    <TableCell>{t('auditLog.action')}</TableCell>
                    <TableCell>{t('auditLog.target')}</TableCell>
                    <TableCell>{t('auditLog.changes')}</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {entries.map((e) => (
                    <EntryRow key={e.id} entry={e} />
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
          {log.hasNextPage && (
            <Box sx={{ textAlign: 'center' }}>
              <Button variant="outlined" onClick={() => void log.fetchNextPage()} loading={log.isFetchingNextPage}>
                {t('app.loadMore')}
              </Button>
            </Box>
          )}
        </>
      )}
    </Stack>
  );
}
