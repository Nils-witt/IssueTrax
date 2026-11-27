import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Grid from '@mui/material/Grid';
import IconButton from '@mui/material/IconButton';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMoveOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import RestoreIcon from '@mui/icons-material/RestoreFromTrashOutlined';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useDeleteStorageItem, useRestoreStorageItem, useStorage, useStorageItem, useStorageItemIssues } from '../api/hooks';
import { isMultiModule, moduleKinds, type CatalogRef, type IPConfig, type IPv4Mode, type IPv6Mode, type Module, type ModuleKind, type StorageItem } from '../api/types';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { StatusBadge } from '../components/StatusBadge';
import { GeneralEditor, LifecycleEditor, ModuleEditor } from './StorageItemEditors';

type T = ReturnType<typeof useTranslation>['t'];

interface DetailsProps {
  id: string;
  onMove: (item: StorageItem) => void;
  onSelectStorage: (id: string) => void;
}

/** Main column plus a sidebar for related records; the sidebar stacks below on narrower screens. */
const detailGrid = { display: 'grid', gap: 4, gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1fr) 22rem' }, alignItems: 'start' } as const;

/** A module being added, not saved yet. */
interface Draft {
  key: number;
  kind: ModuleKind;
}

let nextDraftKey = 0;

/**
 * Everything about one item: general fields, lifecycle, modules and linked
 * issues. General, lifecycle and each module are edited in place on their own.
 */
export function StorageItemDetails({ id, onMove, onSelectStorage }: DetailsProps) {
  const { t } = useTranslation();
  const query = useStorageItem(id);
  const del = useDeleteStorageItem();
  const restore = useRestoreStorageItem();
  // Sections open for editing: 'general', 'lifecycle' or a module ID.
  const [editing, setEditing] = useState<ReadonlySet<string>>(new Set());
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [menu, setMenu] = useState<HTMLElement | null>(null);

  if (query.isPending) return <Loading />;
  if (query.isError) return <ErrorMessage error={query.error} />;

  const item = query.data;
  const editable = !item.deletedAt;
  const isEditing = (section: string) => editable && editing.has(section);
  const toggle = (section: string, on: boolean) =>
    setEditing((prev) => {
      const next = new Set(prev);
      if (on) next.add(section);
      else next.delete(section);
      return next;
    });

  const editButton = (section: string, name: string) =>
    editable &&
    !editing.has(section) && (
      <Tooltip title={t('storageItems.editNamed', { name })}>
        <IconButton size="small" aria-label={t('storageItems.editNamed', { name })} onClick={() => toggle(section, true)}>
          <EditIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    );

  const present = new Set([...item.modules.map((m) => m.kind), ...drafts.map((d) => d.kind)]);
  const addModules = (kinds: ModuleKind[]) => setDrafts((all) => [...all, ...kinds.map((kind) => ({ key: nextDraftKey++, kind }))]);
  const addModule = (kind: ModuleKind) => {
    addModules([kind]);
    setMenu(null);
  };
  const dropDraft = (key: number) => setDrafts((all) => all.filter((d) => d.key !== key));

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

        <Box sx={detailGrid}>
          <Stack spacing={3} sx={{ minWidth: 0 }}>
            <Section title={t('storageItems.general')} action={editButton('general', t('storageItems.general'))}>
              {isEditing('general') ? (
                <GeneralEditor item={item} onDone={() => toggle('general', false)} onAddModules={addModules} />
              ) : (
                <>
                  {item.description && <Typography sx={{ whiteSpace: 'pre-line', overflowWrap: 'anywhere' }}>{item.description}</Typography>}
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
                </>
              )}
            </Section>

            <Section title={t('storageItems.lifecycle')} action={editButton('lifecycle', t('storageItems.lifecycle'))}>
              {isEditing('lifecycle') ? (
                <LifecycleEditor item={item} onDone={() => toggle('lifecycle', false)} />
              ) : (
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
              )}
            </Section>

            <Section
              title={t('modules.title')}
              action={
                editable && (
                  <>
                    <Button size="small" startIcon={<AddIcon />} onClick={(e) => setMenu(e.currentTarget)}>
                      {t('modules.add')}
                    </Button>
                    <Menu anchorEl={menu} open={!!menu} onClose={() => setMenu(null)}>
                      {moduleKinds.map((kind) => (
                        <MenuItem key={kind} disabled={!isMultiModule(kind) && present.has(kind)} onClick={() => addModule(kind)}>
                          {t(`modules.kinds.${kind}`)}
                        </MenuItem>
                      ))}
                    </Menu>
                  </>
                )
              }
            >
              {item.modules.length === 0 && drafts.length === 0 && <Typography color="text.secondary">{t('storageItems.noModules')}</Typography>}
              <Stack spacing={2}>
                {item.modules.map((m) =>
                  isEditing(m.id) ? (
                    <ModuleEditor key={m.id} item={item} moduleId={m.id} onDone={() => toggle(m.id, false)} />
                  ) : (
                    <ModuleView key={m.id} module={m} action={editButton(m.id, moduleTitle(t, m))} />
                  ),
                )}
                {editable &&
                  drafts.map((d) => <ModuleEditor key={`draft-${d.key}`} item={item} kind={d.kind} onDone={() => dropDraft(d.key)} />)}
              </Stack>
            </Section>
          </Stack>

          <Stack component="aside" spacing={3} sx={{ minWidth: 0 }}>
            <Section title={t('storageItems.linkedIssues')}>
              <LinkedIssues id={item.id} />
            </Section>
          </Stack>
        </Box>
      </Stack>
    </Paper>
  );
}

/** A titled part of the details, with an optional action (such as edit) beside the title. */
function Section({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <Stack spacing={1.5}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minHeight: 30 }}>
        <Typography variant="subtitle1" component="h3" sx={{ fontWeight: 600, flexGrow: 1 }}>
          {title}
        </Typography>
        {action}
      </Stack>
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

/** A module's heading: its kind, plus the interface name for network interfaces. */
function moduleTitle(t: T, m: Module): string {
  return m.networkInterface?.name ? `${t(`modules.kinds.${m.kind}`)} · ${m.networkInterface.name}` : t(`modules.kinds.${m.kind}`);
}

function ModuleView({ module: m, action }: { module: Module; action?: ReactNode }) {
  const { t } = useTranslation();

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 1.5 }}>
        <Typography variant="subtitle2" component="h4">
          {moduleTitle(t, m)}
        </Typography>
        {m.primary && <Chip size="small" color="primary" variant="outlined" label={t('modules.primary')} />}
        <Box sx={{ flexGrow: 1 }} />
        {action}
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

  if (m.simCard) {
    const c = m.simCard;

    return (
      <>
        <Field label={t('modules.provider')}>{c.provider}</Field>
        <Field label={t('modules.phoneNumber')}>{c.phoneNumber && <code>{c.phoneNumber}</code>}</Field>
        <Field label={t('modules.iccid')}>{c.iccid && <code>{c.iccid}</code>}</Field>
      </>
    );
  }

  if (m.wwan) {
    const w = m.wwan;

    return (
      <>
        <Field label={t('modules.imei')}>{w.imei && <code>{w.imei}</code>}</Field>
        <Field label={t('modules.apn')}>{w.apn && <code>{w.apn}</code>}</Field>
        <Field label={t('modules.technology')}>{w.technology.toUpperCase()}</Field>
      </>
    );
  }

  if (m.operatingSystem) {
    const o = m.operatingSystem;

    return (
      <>
        <Field label={t('modules.osName')}>{o.name}</Field>
        <Field label={t('modules.osVersion')}>{o.version}</Field>
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
