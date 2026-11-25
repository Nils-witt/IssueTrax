import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Chip from '@mui/material/Chip';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import Stack from '@mui/material/Stack';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useApiKeys, useCreateApiKey, useCreateServiceAccount, useRevokeApiKey, useServiceAccounts } from '../api/hooks';
import type { ApiKey, CreatedApiKey, ServiceAccount } from '../api/types';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { fieldErrorText } from '../lib/errors';

/** Mirrors domain.NormalizeServiceAccountName for the live preview. */
function normalizeName(raw: string): string {
  return raw.trim().toLowerCase().replace(/^bot-/, '');
}

export function ServiceAccountsPage() {
  const { t } = useTranslation();
  const accounts = useServiceAccounts();

  return (
    <Stack spacing={3}>
      <Typography variant="h4" component="h1">
        {t('serviceAccounts.title')}
      </Typography>
      <Typography color="text.secondary" sx={{ maxWidth: '50rem' }}>
        {t('serviceAccounts.intro')}
      </Typography>

      <CreateAccountForm />

      <ErrorMessage error={accounts.error} />
      {accounts.isPending && <Loading />}
      {accounts.data?.length === 0 && <Typography>{t('serviceAccounts.empty')}</Typography>}
      <Stack component="ul" spacing={2} sx={{ listStyle: 'none', p: 0, m: 0 }}>
        {accounts.data?.map((a) => (
          <li key={a.id}>
            <AccountCard account={a} />
          </li>
        ))}
      </Stack>
    </Stack>
  );
}

function CreateAccountForm() {
  const { t } = useTranslation();
  const create = useCreateServiceAccount();
  const [name, setName] = useState('');

  const fieldError = fieldErrorText(t, create.error, 'name');
  const preview = `bot-${normalizeName(name) || '…'}`;

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(name, { onSuccess: () => setName('') });
  }

  return (
    <Stack component="form" onSubmit={submit} spacing={1} sx={{ maxWidth: '44rem' }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
        <TextField
          id="service-account-name"
          size="small"
          label={t('serviceAccounts.name')}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          error={!!fieldError}
          helperText={fieldError ?? t('serviceAccounts.nameHint', { displayName: preview })}
          slotProps={{ htmlInput: { maxLength: 44, autoComplete: 'off', spellCheck: false } }}
          sx={{ flexGrow: 1 }}
        />
        <Button type="submit" variant="contained" startIcon={<AddIcon />} loading={create.isPending} sx={{ whiteSpace: 'nowrap', mt: 0.25 }}>
          {t('serviceAccounts.new')}
        </Button>
      </Stack>
      <ErrorMessage error={create.error && !fieldError ? create.error : null} />
    </Stack>
  );
}

function AccountCard({ account }: { account: ServiceAccount }) {
  const { t } = useTranslation();
  const keys = useApiKeys(account.id);
  const [creating, setCreating] = useState(false);

  return (
    <Card variant="outlined">
      <CardContent>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 2 }}>
          <Typography variant="h6" component="h2" sx={{ fontFamily: 'monospace', flexGrow: 1 }}>
            {account.displayName}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t('serviceAccounts.created')} <DateTime value={account.createdAt} />
          </Typography>
          <Button size="small" variant="outlined" startIcon={<AddIcon />} onClick={() => setCreating(true)}>
            {t('serviceAccounts.newKey')}
          </Button>
        </Stack>

        <ErrorMessage error={keys.error} />
        {keys.isPending && <Loading />}
        {keys.data?.length === 0 && <Typography color="text.secondary">{t('serviceAccounts.noKeys')}</Typography>}
        {keys.data && keys.data.length > 0 && <KeyTable accountId={account.id} keys={keys.data} />}
      </CardContent>
      {creating && <CreateKeyDialog account={account} onClose={() => setCreating(false)} />}
    </Card>
  );
}

function keyState(k: ApiKey, now: number): 'active' | 'expired' | 'revoked' {
  if (k.revokedAt) return 'revoked';
  if (k.expiresAt && new Date(k.expiresAt).getTime() <= now) return 'expired';
  return 'active';
}

function KeyTable({ accountId, keys }: { accountId: string; keys: ApiKey[] }) {
  const { t } = useTranslation();
  const revoke = useRevokeApiKey(accountId);
  const [confirming, setConfirming] = useState<ApiKey | null>(null);
  const [now] = useState(() => Date.now());

  const stateColor = { active: 'success', expired: 'default', revoked: 'error' } as const;

  return (
    <>
      <ErrorMessage error={revoke.error} />
      <TableContainer>
        <Table size="small" aria-label={t('serviceAccounts.keys')}>
          <TableHead>
            <TableRow>
              <TableCell>{t('serviceAccounts.name')}</TableCell>
              <TableCell>{t('serviceAccounts.prefix')}</TableCell>
              <TableCell>{t('serviceAccounts.created')}</TableCell>
              <TableCell>{t('serviceAccounts.lastUsed')}</TableCell>
              <TableCell>{t('serviceAccounts.expires')}</TableCell>
              <TableCell>{t('serviceAccounts.status')}</TableCell>
              <TableCell />
            </TableRow>
          </TableHead>
          <TableBody>
            {keys.map((k) => {
              const state = keyState(k, now);

              return (
                <TableRow key={k.id}>
                  <TableCell>{k.name}</TableCell>
                  <TableCell>
                    <KeyText>{k.prefix}…</KeyText>
                  </TableCell>
                  <TableCell>
                    <DateTime value={k.createdAt} />
                  </TableCell>
                  <TableCell>{k.lastUsedAt ? <DateTime value={k.lastUsedAt} /> : t('serviceAccounts.never')}</TableCell>
                  <TableCell>{k.expiresAt ? <DateTime value={k.expiresAt} /> : t('serviceAccounts.never')}</TableCell>
                  <TableCell>
                    <Chip size="small" variant="outlined" color={stateColor[state]} label={t(`serviceAccounts.${state}`)} />
                  </TableCell>
                  <TableCell align="right">
                    {state !== 'revoked' && (
                      <Button size="small" color="error" onClick={() => setConfirming(k)}>
                        {t('serviceAccounts.revoke')}
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={!!confirming} onClose={() => setConfirming(null)}>
        <DialogContent>
          <DialogContentText>{t('serviceAccounts.revokeConfirm', { name: confirming?.name ?? '' })}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirming(null)}>{t('app.cancel')}</Button>
          <Button
            color="error"
            variant="contained"
            loading={revoke.isPending}
            onClick={() => confirming && revoke.mutate(confirming.id, { onSettled: () => setConfirming(null) })}
          >
            {t('serviceAccounts.revoke')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

/** The end of the given local calendar day ("2026-12-31") as RFC 3339. */
function endOfDay(date: string): string {
  return new Date(`${date}T23:59:59`).toISOString();
}

function CreateKeyDialog({ account, onClose }: { account: ServiceAccount; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateApiKey(account.id);
  const [name, setName] = useState('');
  const [expires, setExpires] = useState('');
  const [created, setCreated] = useState<CreatedApiKey | null>(null);

  const nameError = fieldErrorText(t, create.error, 'name');
  const expiresError = fieldErrorText(t, create.error, 'expiresAt');

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate({ name, expiresAt: expires ? endOfDay(expires) : null }, { onSuccess: setCreated });
  }

  // Once the token is shown, closing by backdrop or Escape is disabled so
  // it is not lost by accident.
  if (created) {
    return (
      <Dialog open onClose={(_, reason) => reason !== 'backdropClick' && reason !== 'escapeKeyDown' && onClose()} maxWidth="sm" fullWidth>
        <DialogTitle>{t('serviceAccounts.tokenTitle')}</DialogTitle>
        <DialogContent>
          <Stack spacing={2}>
            <Alert severity="warning">{t('serviceAccounts.tokenWarning')}</Alert>
            <TokenField token={created.token} />
            <Typography variant="body2" color="text.secondary">
              {t('serviceAccounts.tokenUsage')}
            </Typography>
            <Box component="code" sx={{ fontFamily: 'monospace', fontSize: '0.85rem', wordBreak: 'break-all' }}>
              Authorization: Bearer {created.prefix}…
            </Box>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button variant="contained" onClick={onClose}>
            {t('serviceAccounts.done')}
          </Button>
        </DialogActions>
      </Dialog>
    );
  }

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <Box component="form" onSubmit={submit}>
        <DialogTitle>
          {t('serviceAccounts.newKey')} · <Box component="span" sx={{ fontFamily: 'monospace' }}>{account.displayName}</Box>
        </DialogTitle>
        <DialogContent>
          <Stack spacing={3} sx={{ pt: 1 }}>
            <TextField
              autoFocus
              id="api-key-name"
              label={t('serviceAccounts.keyName')}
              placeholder={t('serviceAccounts.keyNamePlaceholder')}
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              error={!!nameError}
              helperText={nameError}
              slotProps={{ htmlInput: { maxLength: 100 } }}
            />
            <TextField
              id="api-key-expires"
              type="date"
              label={`${t('serviceAccounts.expiresAt')} (${t('app.optional')})`}
              value={expires}
              onChange={(e) => setExpires(e.target.value)}
              error={!!expiresError}
              helperText={expiresError ?? t('serviceAccounts.expiresHint')}
              slotProps={{ inputLabel: { shrink: true } }}
            />
            <ErrorMessage error={create.error && !nameError && !expiresError ? create.error : null} />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>{t('app.cancel')}</Button>
          <Button type="submit" variant="contained" loading={create.isPending}>
            {t('app.create')}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}

function TokenField({ token }: { token: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  function copy() {
    void navigator.clipboard.writeText(token).then(() => setCopied(true));
  }

  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
      <TextField
        value={token}
        fullWidth
        size="small"
        slotProps={{
          htmlInput: { readOnly: true, 'aria-label': t('serviceAccounts.tokenTitle'), style: { fontFamily: 'monospace' } },
        }}
        onFocus={(e) => e.target.select()}
      />
      <Button variant="outlined" onClick={copy} sx={{ whiteSpace: 'nowrap' }}>
        {copied ? t('serviceAccounts.copied') : t('serviceAccounts.copy')}
      </Button>
    </Stack>
  );
}
