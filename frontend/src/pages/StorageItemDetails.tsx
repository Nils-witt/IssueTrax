import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Grid from '@mui/material/Grid';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMoveOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import RestoreIcon from '@mui/icons-material/RestoreFromTrashOutlined';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useDeleteStorageItem, useRestoreStorageItem, useStorage, useStorageItem, useStorageItemIssues } from '../api/hooks';
import type { CatalogRef, IPConfig, IPv4Mode, IPv6Mode, Module, StorageItem } from '../api/types';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { StatusBadge } from '../components/StatusBadge';

type T = ReturnType<typeof useTranslation>['t'];

interface DetailsProps {
  id: string;
  onEdit: (item: StorageItem) => void;
  onMove: (item: StorageItem) => void;
  onSelectStorage: (id: string) => void;
}

/** Everything about one item: general fields, lifecycle, modules and linked issues. */
export function StorageItemDetails({ id, onEdit, onMove, onSelectStorage }: DetailsProps) {
  const { t } = useTranslation();
  const query = useStorageItem(id);
  const del = useDeleteStorageItem();
  const restore = useRestoreStorageItem();

  if (query.isPending) return <Loading />;
  if (query.isError) return <ErrorMessage error={query.error} />;

  const item = query.data;

  return (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Stack spacing={3}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ alignItems: { sm: 'flex-start' } }}>
          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            <KeyText>{item.assetTag}</KeyText>
            <Typography variant="h5" component="h2" sx={{ overflowWrap: 'anywhere', textDecoration: item.deletedAt ? 'line-through' : undefined }}>
              {item.name}
            </Typography>
            <Stack direction="row" spacing={1} useFlexGap sx={{ mt: 1, flexWrap: 'wrap' }}>
              <Chip size="small" variant="outlined" label={item.type.name} />
              <StorageLink id={item.storageId} onSelect={onSelectStorage} />
              {!item.usable && <Chip size="small" color="error" variant="outlined" label={t('storageItems.notUsable')} />}
              {item.decommissionedAt && <Chip size="small" variant="outlined" label={t('storageItems.decommissionedBadge')} />}
              {item.deletedAt && <Chip size="small" color="warning" variant="outlined" label={t('storageItems.deleted')} />}
            </Stack>
          </Box>
          <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
            {item.deletedAt ? (
              <Button size="small" variant="outlined" startIcon={<RestoreIcon />} loading={restore.isPending} onClick={() => restore.mutate(item.id)}>
                {t('storageItems.restore')}
              </Button>
            ) : (
              <>
                <Button size="small" variant="outlined" startIcon={<EditIcon />} onClick={() => onEdit(item)}>
                  {t('storageItems.edit')}
                </Button>
                <Button size="small" variant="outlined" startIcon={<DriveFileMoveIcon />} onClick={() => onMove(item)}>
                  {t('storageItems.move')}
                </Button>
                <Button size="small" color="error" startIcon={<DeleteIcon />} loading={del.isPending} onClick={() => del.mutate(item.id)}>
                  {t('storageItems.delete')}
                </Button>
              </>
            )}
          </Stack>
        </Stack>
        <ErrorMessage error={del.error ?? restore.error} />

        {item.description && <Typography sx={{ whiteSpace: 'pre-line', overflowWrap: 'anywhere' }}>{item.description}</Typography>}

        <Section title={t('storageItems.general')}>
          <Fields>
            <Field label={t('storageItems.type')}>{item.type.name}</Field>
            <Field label={t('storageItems.usable')}>{item.usable ? t('storageItems.yes') : t('storageItems.no')}</Field>
            <Field label={t('storageItems.personResponsible')}>{item.personResponsible?.displayName}</Field>
            <Field label={t('storageItems.created')}>
              <DateTime value={item.createdAt} />
            </Field>
            <Field label={t('storageItems.updated')}>
              <DateTime value={item.updatedAt} />
            </Field>
            {item.deletedAt && (
              <Field label={t('storageItems.deleted')}>
                <DateTime value={item.deletedAt} />
              </Field>
            )}
          </Fields>
        </Section>

        <Section title={t('storageItems.lifecycle')}>
          <Fields>
            <Field label={t('storageItems.orderDate')}>
              <DateOnly value={item.orderDate} />
            </Field>
            <Field label={t('storageItems.commissionedAt')}>
              <DateOnly value={item.commissionedAt} />
            </Field>
            <Field label={t('storageItems.decommissionedAt')}>
              <DateOnly value={item.decommissionedAt} />
            </Field>
            {item.decommissioningReason && (
              <Field label={t('storageItems.decommissioningReason')} wide>
                {item.decommissioningReason}
              </Field>
            )}
          </Fields>
        </Section>

        <Section title={t('modules.title')}>
          {item.modules.length === 0 ? (
            <Typography color="text.secondary">{t('storageItems.noModules')}</Typography>
          ) : (
            <Stack spacing={2}>
              {item.modules.map((m) => (
                <ModuleView key={m.id} module={m} />
              ))}
            </Stack>
          )}
        </Section>

        <Section title={t('storageItems.linkedIssues')}>
          <LinkedIssues id={item.id} />
        </Section>
      </Stack>
    </Paper>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Stack spacing={1.5}>
      <Typography variant="subtitle1" component="h3" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>
      {children}
    </Stack>
  );
}

function Fields({ children }: { children: ReactNode }) {
  return (
    <Grid container spacing={2} component="dl" sx={{ m: 0 }}>
      {children}
    </Grid>
  );
}

/** A labelled value; empty values show a dash. */
function Field({ label, children, wide = false }: { label: string; children: ReactNode; wide?: boolean }) {
  const empty = children === undefined || children === null || children === '';

  return (
    <Grid size={wide ? 12 : { xs: 12, sm: 6, md: 4 }}>
      <Typography component="dt" variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography component="dd" sx={{ m: 0, overflowWrap: 'anywhere' }} color={empty ? 'text.secondary' : undefined}>
        {empty ? '—' : children}
      </Typography>
    </Grid>
  );
}

/** A timestamp shown as its date only, or nothing. */
function DateOnly({ value }: { value: string | null }) {
  const { i18n } = useTranslation();
  if (!value) return null;

  // Dates are stored at midnight UTC; format in UTC so they don't shift a day.
  const text = new Intl.DateTimeFormat(i18n.resolvedLanguage, { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(value));

  return <time dateTime={value.slice(0, 10)}>{text}</time>;
}

/** The item's storage, selecting it on click. */
function StorageLink({ id, onSelect }: { id: string; onSelect: (id: string) => void }) {
  const storage = useStorage(id);

  return (
    <Chip
      size="small"
      variant="outlined"
      icon={<InventoryIcon />}
      label={storage.data?.name ?? '…'}
      onClick={() => onSelect(id)}
    />
  );
}

function ModuleView({ module: m }: { module: Module }) {
  const { t } = useTranslation();
  const title = m.networkInterface?.name ? `${t(`modules.kinds.${m.kind}`)} · ${m.networkInterface.name}` : t(`modules.kinds.${m.kind}`);

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 1.5 }}>
        <Typography variant="subtitle2" component="h4">
          {title}
        </Typography>
        {m.primary && <Chip size="small" color="primary" variant="outlined" label={t('modules.primary')} />}
      </Stack>
      <Fields>{moduleFields(t, m)}</Fields>
    </Paper>
  );
}

/** The name of a manufacturer or model, marked when it was deleted from the list. */
function catalogName(t: T, ref: CatalogRef | null): string | undefined {
  if (!ref) return undefined;

  return ref.deleted ? t('catalog.deletedName', { name: ref.name }) : ref.name;
}

function moduleFields(t: T, m: Module): ReactNode {
  if (m.hardware) {
    return (
      <>
        <Field label={t('modules.manufacturer')}>{catalogName(t, m.hardware.manufacturer)}</Field>
        <Field label={t('modules.model')}>{catalogName(t, m.hardware.model)}</Field>
        <Field label={t('modules.serialNumber')}>{m.hardware.serialNumber && <code>{m.hardware.serialNumber}</code>}</Field>
      </>
    );
  }

  if (m.networkHost) {
    return (
      <>
        <Field label={t('modules.role')}>{t(`modules.roles.${m.networkHost.role}`)}</Field>
        <Field label={t('modules.hostname')}>{m.networkHost.hostname && <code>{m.networkHost.hostname}</code>}</Field>
      </>
    );
  }

  if (m.purchase) {
    const p = m.purchase;

    return (
      <>
        <Field label={t('modules.supplier')}>{p.supplier}</Field>
        <Field label={t('modules.invoiceNumber')}>{p.invoiceNumber}</Field>
        <Field label={t('modules.price')}>{p.price && `${p.price} ${p.currency}`.trim()}</Field>
        <Field label={t('modules.warrantyEnd')}>
          <DateOnly value={p.warrantyEnd} />
        </Field>
      </>
    );
  }

  if (m.networkInterface) {
    const n = m.networkInterface;

    return (
      <>
        <Field label={t('modules.mac')}>{n.mac && <code>{n.mac}</code>}</Field>
        <Field label={t('modules.type')}>{t(`modules.types.${n.type}`)}</Field>
        {n.type === 'wireless' && <Field label={t('modules.ssid')}>{n.ssid}</Field>}
        <Field label={t('modules.nativeVlan')}>{n.nativeVlan ?? undefined}</Field>
        <Field label={t('modules.taggedVlans')}>{n.taggedVlans.join(', ')}</Field>
        <Field label={t('modules.ipv4Mode')}>{ipText(t, n.ipv4)}</Field>
        <Field label={t('modules.ipv6Mode')}>{ipText(t, n.ipv6)}</Field>
      </>
    );
  }

  return null;
}

/** An IP family's mode, plus address, gateway and DNS when static. */
function ipText(t: T, ip: IPConfig<IPv4Mode | IPv6Mode>): ReactNode {
  const mode = t(`modules.modes.${ip.mode}`);
  if (ip.mode !== 'static') return mode;

  return (
    <Stack component="span" sx={{ display: 'flex' }}>
      <span>
        {mode}: <code>{ip.address ? `${ip.address}/${ip.prefixLength ?? ''}` : ''}</code>
      </span>
      {ip.gateway && (
        <Typography component="span" variant="body2" color="text.secondary">
          {t('modules.gateway')}: <code>{ip.gateway}</code>
        </Typography>
      )}
      {ip.dnsServers.length > 0 && (
        <Typography component="span" variant="body2" color="text.secondary">
          {t('modules.dnsServers')}: <code>{ip.dnsServers.join(', ')}</code>
        </Typography>
      )}
    </Stack>
  );
}

/** The issues of every project linking the item, each opening the issue. */
function LinkedIssues({ id }: { id: string }) {
  const { t } = useTranslation();
  const issues = useStorageItemIssues(id);
  const list = issues.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <>
      {issues.isPending && <Loading />}
      <ErrorMessage error={issues.error} />
      {issues.isSuccess && list.length === 0 && <Typography color="text.secondary">{t('storageItems.noLinkedIssues')}</Typography>}
      {list.length > 0 && (
        <Paper variant="outlined">
          <List dense disablePadding>
            {list.map((issue, i) => (
              <ListItem key={issue.id} disablePadding divider={i < list.length - 1}>
                <ListItemButton component={Link} to={`/p/${issue.projectKey}/issues/${issue.number}`}>
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                        <KeyText>{issue.key}</KeyText>
                        <span>{issue.title}</span>
                      </Stack>
                    }
                  />
                  <StatusBadge status={issue.status} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Paper>
      )}
      {issues.hasNextPage && (
        <Box>
          <Button size="small" loading={issues.isFetchingNextPage} onClick={() => void issues.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        </Box>
      )}
    </>
  );
}
