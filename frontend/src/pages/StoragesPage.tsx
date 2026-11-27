import Autocomplete from '@mui/material/Autocomplete';
import Box from '@mui/material/Box';
import FormControlLabel from '@mui/material/FormControlLabel';
import Grid from '@mui/material/Grid';
import InputAdornment from '@mui/material/InputAdornment';
import Switch from '@mui/material/Switch';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Collapse from '@mui/material/Collapse';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import IconButton from '@mui/material/IconButton';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import ClearIcon from '@mui/icons-material/Clear';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import RestoreIcon from '@mui/icons-material/RestoreFromTrashOutlined';
import SearchIcon from '@mui/icons-material/Search';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMoveOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import HomeIcon from '@mui/icons-material/HomeOutlined';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import LinkIcon from '@mui/icons-material/Link';
import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import MenuItem from '@mui/material/MenuItem';
import {
  useCreateStorage,
  useDeleteStorageItem,
  useRestoreStorageItem,
  useStorage,
  useStorageChildren,
  useStorageItemIssues,
  useStorageItems,
  useStorageItemSearch,
  useUpdateStorage,
  useUpdateStorageItem,
  useUsers,
} from '../api/hooks';
import type { ItemTypeRef, Page, Storage, StorageItem, StorageItemFilter, User } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { ItemTypePicker } from '../components/ItemTypePicker';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { StatusBadge } from '../components/StatusBadge';
import { fieldErrorText } from '../lib/errors';
import { StorageItemDialog } from './StorageItemDialog';

/** The dialog currently open on the page, if any. */
type DialogState =
  | { kind: 'create'; parent: Storage | null }
  | { kind: 'rename'; storage: Storage }
  | { kind: 'move'; storage: Storage }
  | { kind: 'createItem'; storage: Storage }
  | { kind: 'editItem'; item: StorageItem }
  | { kind: 'moveItem'; item: StorageItem }
  | { kind: 'linkedIssues'; item: StorageItem };

/** What the item panel shows: a storage's items or search results. */
interface Selection {
  selectedId: string | null;
  onSelect: (storage: Storage) => void;
}

/** Options of a tree used to pick a new parent instead of editing. */
interface Picker {
  /** Selected parent ID; null selects the root. */
  selected: string | null;
  onSelect: (id: string) => void;
  /** The storage being moved: it and its subtree cannot be picked. */
  excludeId: string;
}

interface TreeProps {
  /** Whose children to show; null shows the roots. */
  parent: string | null;
  depth: number;
  actions?: (storage: Storage) => ReactNode;
  picker?: Picker;
  /** Outside a picker: which storage's items are shown. */
  selection?: Selection;
}

export function StoragesPage() {
  const { t } = useTranslation();
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const close = () => setDialog(null);
  // ?storage= preselects a storage, e.g. when following a link from an issue.
  const [params] = useSearchParams();
  const [selectedId, setSelectedId] = useState<string | null>(() => params.get('storage'));
  const [search, setSearch] = useState('');
  const [q, setQ] = useState('');

  // Debounce the search box before querying.
  useEffect(() => {
    const timer = setTimeout(() => setQ(search.trim()), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const selection: Selection = {
    selectedId,
    onSelect: (st) => {
      setSelectedId(st.id);
      setSearch('');
      setQ('');
    },
  };

  const itemDialogs = {
    onCreate: (storage: Storage) => setDialog({ kind: 'createItem', storage }),
    onEdit: (item: StorageItem) => setDialog({ kind: 'editItem', item }),
    onMove: (item: StorageItem) => setDialog({ kind: 'moveItem', item }),
    onShowIssues: (item: StorageItem) => setDialog({ kind: 'linkedIssues', item }),
  };

  const actions = (s: Storage) => (
    <Stack direction="row" spacing={0.5}>
      <Tooltip title={t('storages.addChild')}>
        <IconButton size="small" aria-label={t('storages.addChildTo', { name: s.name })} onClick={() => setDialog({ kind: 'create', parent: s })}>
          <AddIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('storages.rename')}>
        <IconButton size="small" aria-label={t('storages.renameNamed', { name: s.name })} onClick={() => setDialog({ kind: 'rename', storage: s })}>
          <EditIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('storages.move')}>
        <IconButton size="small" aria-label={t('storages.moveNamed', { name: s.name })} onClick={() => setDialog({ kind: 'move', storage: s })}>
          <DriveFileMoveIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </Stack>
  );

  return (
    <Stack spacing={3}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        <Typography variant="h4" component="h1" sx={{ flexGrow: 1 }}>
          {t('storages.title')}
        </Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialog({ kind: 'create', parent: null })}>
          {t('storages.newRoot')}
        </Button>
      </Stack>
      <Typography color="text.secondary" sx={{ maxWidth: '50rem' }}>
        {t('storages.intro')}
      </Typography>

      <TextField
        type="search"
        size="small"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        placeholder={t('storageItems.searchPlaceholder')}
        sx={{ maxWidth: '40rem' }}
        slotProps={{
          htmlInput: { maxLength: 200, 'aria-label': t('storageItems.searchLabel') },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          },
        }}
      />

      <Grid container spacing={3}>
        <Grid size={{ xs: 12, md: 6 }}>
          <Paper variant="outlined">
            <StorageTree parent={null} depth={0} actions={actions} selection={selection} />
          </Paper>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          {q ? (
            <SearchPanel q={q} onClear={() => setSearch('')} {...itemDialogs} />
          ) : selectedId ? (
            <ItemsPanel key={selectedId} storageId={selectedId} {...itemDialogs} />
          ) : (
            <Typography color="text.secondary">{t('storageItems.pickStorage')}</Typography>
          )}
        </Grid>
      </Grid>

      {dialog?.kind === 'create' && <NameDialog parent={dialog.parent} onClose={close} />}
      {dialog?.kind === 'rename' && <NameDialog storage={dialog.storage} onClose={close} />}
      {dialog?.kind === 'move' && <MoveDialog storage={dialog.storage} onClose={close} />}
      {dialog?.kind === 'createItem' && <StorageItemDialog storage={dialog.storage} onClose={close} />}
      {dialog?.kind === 'editItem' && <StorageItemDialog item={dialog.item} onClose={close} />}
      {dialog?.kind === 'moveItem' && <MoveItemDialog item={dialog.item} onClose={close} />}
      {dialog?.kind === 'linkedIssues' && <LinkedIssuesDialog item={dialog.item} onClose={close} />}
    </Stack>
  );
}

/** One level of the tree, loaded page by page; deeper levels load when expanded. */
function StorageTree({ parent, depth, actions, picker, selection }: TreeProps) {
  const { t } = useTranslation();
  const children = useStorageChildren(parent);
  const items = children.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <List dense disablePadding aria-label={depth === 0 ? t('storages.title') : undefined}>
      {children.isPending && <Loading />}
      <ErrorMessage error={children.error} />
      {children.isSuccess && items.length === 0 && depth === 0 && (
        <ListItem>
          <ListItemText secondary={t('storages.empty')} />
        </ListItem>
      )}
      {items.map((s) => (
        <StorageNode key={s.id} storage={s} depth={depth} actions={actions} picker={picker} selection={selection} />
      ))}
      {children.hasNextPage && (
        <ListItem sx={{ pl: indent(depth) }}>
          <Button size="small" loading={children.isFetchingNextPage} onClick={() => void children.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        </ListItem>
      )}
    </List>
  );
}

const indent = (depth: number) => 1 + depth * 3;

function StorageNode({ storage, depth, actions, picker, selection }: { storage: Storage } & Omit<TreeProps, 'parent'>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  // In a picker, the moved storage's subtree stays closed: none of it can
  // become the new parent.
  const excluded = picker?.excludeId === storage.id;
  const expandable = storage.childCount > 0 && !excluded;

  const toggle = (
    <IconButton
      size="small"
      edge="start"
      aria-label={open ? t('storages.collapse', { name: storage.name }) : t('storages.expand', { name: storage.name })}
      aria-expanded={open}
      onClick={(e) => {
        e.stopPropagation();
        setOpen((o) => !o);
      }}
      sx={{ visibility: expandable ? 'visible' : 'hidden' }}
    >
      {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
    </IconButton>
  );

  const label = (
    <>
      <ListItemIcon sx={{ minWidth: 32 }}>
        <InventoryIcon fontSize="small" color={excluded ? 'disabled' : 'action'} />
      </ListItemIcon>
      <ListItemText primary={storage.name} />
      {storage.childCount > 0 && (
        <Chip size="small" variant="outlined" label={t('storages.children', { count: storage.childCount })} sx={{ mr: 1 }} />
      )}
      {storage.itemCount > 0 && (
        <Chip size="small" variant="outlined" color="primary" label={t('storages.items', { count: storage.itemCount })} sx={{ mr: 1 }} />
      )}
    </>
  );

  return (
    <>
      {picker ? (
        <ListItemButton
          selected={picker.selected === storage.id}
          disabled={excluded}
          onClick={() => picker.onSelect(storage.id)}
          sx={{ pl: indent(depth) }}
        >
          {toggle}
          {label}
        </ListItemButton>
      ) : (
        <ListItem
          sx={{ pl: indent(depth), '&:hover': { bgcolor: 'action.hover' } }}
          secondaryAction={actions?.(storage)}
          slotProps={{ secondaryAction: { sx: { right: 8 } } }}
        >
          {toggle}
          {selection ? (
            <ListItemButton
              selected={selection.selectedId === storage.id}
              aria-label={t('storages.select', { name: storage.name })}
              onClick={() => selection.onSelect(storage)}
              sx={{ flexGrow: 1, mr: 14, py: 0.25, borderRadius: 1 }}
            >
              {label}
            </ListItemButton>
          ) : (
            <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1, pr: 14 }}>{label}</Box>
          )}
        </ListItem>
      )}
      {expandable && (
        <Collapse in={open} unmountOnExit>
          <StorageTree parent={storage.id} depth={depth + 1} actions={actions} picker={picker} selection={selection} />
        </Collapse>
      )}
    </>
  );
}

/** Creates a storage under parent, or renames storage. */
function NameDialog({ parent, storage, onClose }: { parent?: Storage | null; storage?: Storage; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateStorage();
  const update = useUpdateStorage();
  const mutation = storage ? update : create;
  const [name, setName] = useState(storage?.name ?? '');

  const fieldError = fieldErrorText(t, mutation.error, 'name');
  const parentError = fieldErrorText(t, mutation.error, 'parentId');

  function submit(e: FormEvent) {
    e.preventDefault();

    if (storage) update.mutate({ id: storage.id, input: { name } }, { onSuccess: onClose });
    else create.mutate({ name, parentId: parent?.id ?? null }, { onSuccess: onClose });
  }

  const title = storage
    ? t('storages.renameTitle', { name: storage.name })
    : parent
      ? t('storages.createChildTitle', { name: parent.name })
      : t('storages.createRootTitle');

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="xs" slotProps={{ paper: { component: 'form', onSubmit: submit } }}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <TextField
            autoFocus
            required
            size="small"
            label={t('storages.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={!!fieldError}
            helperText={fieldError}
            slotProps={{ htmlInput: { maxLength: 100, autoComplete: 'off' } }}
          />
          <ErrorMessage error={mutation.error && !fieldError ? mutation.error : null} />
          {parentError && <Typography color="error">{t('storages.parentGone')}</Typography>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button type="submit" variant="contained" loading={mutation.isPending}>
          {storage ? t('app.save') : t('app.create')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** Picks a new parent (or the root) for storage. */
function MoveDialog({ storage, onClose }: { storage: Storage; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateStorage();
  const [selected, setSelected] = useState<string | null>(storage.parentId);

  const unchanged = selected === storage.parentId;

  function submit() {
    update.mutate({ id: storage.id, input: { parentId: selected } }, { onSuccess: onClose });
  }

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('storages.moveTitle', { name: storage.name })}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 1 }}>{t('storages.moveHint')}</DialogContentText>
        <Paper variant="outlined" sx={{ maxHeight: '50vh', overflowY: 'auto' }}>
          <List dense disablePadding>
            <ListItemButton selected={selected === null} onClick={() => setSelected(null)}>
              <ListItemIcon sx={{ minWidth: 32 }}>
                <HomeIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText primary={t('storages.root')} />
            </ListItemButton>
          </List>
          <StorageTree parent={null} depth={1} picker={{ selected, onSelect: setSelected, excludeId: storage.id }} />
        </Paper>
        <Box sx={{ mt: 2 }}>
          <ErrorMessage error={update.error} />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button variant="contained" disabled={unchanged} loading={update.isPending} onClick={submit}>
          {t('storages.moveHere')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface ItemDialogs {
  onCreate: (storage: Storage) => void;
  onEdit: (item: StorageItem) => void;
  onMove: (item: StorageItem) => void;
  onShowIssues: (item: StorageItem) => void;
}

type ItemActions = Pick<ItemDialogs, 'onEdit' | 'onMove' | 'onShowIssues'>;

/** The list filters both item panels share, as edited. */
interface FilterState {
  type: ItemTypeRef | null;
  includeSubtypes: boolean;
  usable: '' | 'true' | 'false';
  decommissioned: '' | 'true' | 'false';
  responsible: User | null;
  deleted: boolean;
}

const noFilters: FilterState = { type: null, includeSubtypes: true, usable: '', decommissioned: '', responsible: null, deleted: false };

const triState = (v: '' | 'true' | 'false') => (v === '' ? undefined : v === 'true');

function toFilter(f: FilterState): StorageItemFilter {
  return {
    typeId: f.type?.id,
    includeSubtypes: f.type ? f.includeSubtypes : undefined,
    usable: triState(f.usable),
    decommissioned: triState(f.decommissioned),
    responsibleId: f.responsible?.id,
    deleted: f.deleted,
  };
}

/** Filters by type (with subtypes), usable, decommissioning, responsible person and deletion. */
function ItemFilters({ value, onChange }: { value: FilterState; onChange: (next: FilterState) => void }) {
  const { t } = useTranslation();
  const users = useUsers();
  const people = (users.data ?? []).filter((u) => !u.serviceAccount);
  const set = <K extends keyof FilterState>(key: K, v: FilterState[K]) => onChange({ ...value, [key]: v });

  const tri = (label: string, key: 'usable' | 'decommissioned') => (
    <TextField select size="small" label={label} value={value[key]} onChange={(e) => set(key, e.target.value as FilterState[typeof key])} sx={{ minWidth: 140 }}>
      <MenuItem value="">{t('storageItems.any')}</MenuItem>
      <MenuItem value="true">{t('storageItems.yes')}</MenuItem>
      <MenuItem value="false">{t('storageItems.no')}</MenuItem>
    </TextField>
  );

  return (
    <Stack spacing={1}>
      <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
        <ItemTypePicker id="filter-type" label={t('storageItems.filterType')} value={value.type} onChange={(v) => set('type', v)} sx={{ minWidth: 220 }} />
        {tri(t('storageItems.filterUsable'), 'usable')}
        {tri(t('storageItems.filterDecommissioned'), 'decommissioned')}
        <Autocomplete
          size="small"
          options={people}
          value={value.responsible}
          onChange={(_, u) => set('responsible', u)}
          getOptionLabel={(u) => u.displayName}
          getOptionKey={(u) => u.id}
          isOptionEqualToValue={(a, b) => a.id === b.id}
          sx={{ minWidth: 200 }}
          renderInput={(params) => <TextField {...params} label={t('storageItems.filterResponsible')} />}
        />
      </Stack>
      <Stack direction="row" sx={{ flexWrap: 'wrap' }}>
        {value.type && (
          <FormControlLabel
            control={<Switch size="small" checked={value.includeSubtypes} onChange={(e) => set('includeSubtypes', e.target.checked)} />}
            label={t('storageItems.includeSubtypes')}
          />
        )}
        <FormControlLabel
          control={<Switch size="small" checked={value.deleted} onChange={(e) => set('deleted', e.target.checked)} />}
          label={t('storageItems.showDeleted')}
        />
      </Stack>
    </Stack>
  );
}

/** The items kept in one storage, optionally with its subtree, narrowed by the filters. */
function ItemsPanel({ storageId, onCreate, ...actions }: { storageId: string } & ItemDialogs) {
  const { t } = useTranslation();
  const storage = useStorage(storageId);
  const [recursive, setRecursive] = useState(false);
  const [filters, setFilters] = useState(noFilters);
  const items = useStorageItems(storageId, { ...toFilter(filters), recursive });

  if (storage.isPending) return <Loading />;
  if (storage.isError) return <ErrorMessage error={storage.error} />;

  const st = storage.data;

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack spacing={1}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Typography variant="h6" component="h2" sx={{ flexGrow: 1 }}>
            {t('storageItems.inStorage', { name: st.name })}
          </Typography>
          <Button size="small" variant="outlined" startIcon={<AddIcon />} onClick={() => onCreate(st)}>
            {t('storageItems.add')}
          </Button>
        </Stack>
        <FormControlLabel
          control={<Switch size="small" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />}
          label={t('storageItems.recursive')}
        />
        <ItemFilters value={filters} onChange={setFilters} />
        <ItemList
          query={items}
          showStorage={recursive}
          emptyText={filters.deleted ? t('storageItems.emptyDeleted') : t('storageItems.empty')}
          {...actions}
        />
      </Stack>
    </Paper>
  );
}

/** Items across all storages matching q and the filters. */
function SearchPanel({ q, onClear, ...actions }: { q: string; onClear: () => void } & ItemActions) {
  const { t } = useTranslation();
  const [filters, setFilters] = useState(noFilters);
  const items = useStorageItemSearch({ ...toFilter(filters), q });

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack spacing={1}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Typography variant="h6" component="h2" sx={{ flexGrow: 1 }}>
            {t('storageItems.searchTitle', { q })}
          </Typography>
          <Button size="small" startIcon={<ClearIcon />} onClick={onClear}>
            {t('storageItems.clearSearch')}
          </Button>
        </Stack>
        <ItemFilters value={filters} onChange={setFilters} />
        <ItemList query={items} showStorage emptyText={t('storageItems.noMatches')} {...actions} />
      </Stack>
    </Paper>
  );
}

/** What ItemList needs of an infinite item query. */
interface ItemQuery {
  data?: { pages: Page<StorageItem>[] };
  error: unknown;
  isPending: boolean;
  isSuccess: boolean;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
}

function ItemList({
  query,
  showStorage,
  emptyText,
  onEdit,
  onMove,
  onShowIssues,
}: { query: ItemQuery; showStorage: boolean; emptyText: string } & ItemActions) {
  const { t } = useTranslation();
  const del = useDeleteStorageItem();
  const restore = useRestoreStorageItem();
  const items = query.data?.pages.flatMap((p) => p.items) ?? [];
  const total = query.data?.pages[0]?.total ?? 0;

  const busy = (m: { isPending: boolean; variables?: string }, item: StorageItem) => m.isPending && m.variables === item.id;

  const issuesButton = (item: StorageItem) => (
    <Tooltip title={t('storageItems.showLinkedIssues')}>
      <IconButton size="small" aria-label={t('storageItems.showLinkedIssuesNamed', { name: item.name })} onClick={() => onShowIssues(item)}>
        <LinkIcon fontSize="small" />
      </IconButton>
    </Tooltip>
  );

  return (
    <>
      {query.isPending && <Loading />}
      <ErrorMessage error={query.error ?? del.error ?? restore.error} />
      {query.isSuccess && (
        <Typography variant="body2" color="text.secondary">
          {items.length === 0 ? emptyText : t('storageItems.count', { count: total })}
        </Typography>
      )}
      <List dense disablePadding>
        {items.map((item) => (
          <ListItem
            key={item.id}
            divider
            secondaryAction={
              item.deletedAt ? (
                <Stack direction="row" spacing={0.5}>
                  {issuesButton(item)}
                  <Tooltip title={t('storageItems.restore')}>
                    <IconButton
                      size="small"
                      aria-label={t('storageItems.restoreNamed', { name: item.name })}
                      loading={busy(restore, item)}
                      onClick={() => restore.mutate(item.id)}
                    >
                      <RestoreIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </Stack>
              ) : (
                <Stack direction="row" spacing={0.5}>
                  {issuesButton(item)}
                  <Tooltip title={t('storageItems.edit')}>
                    <IconButton size="small" aria-label={t('storageItems.editNamed', { name: item.name })} onClick={() => onEdit(item)}>
                      <EditIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('storageItems.move')}>
                    <IconButton size="small" aria-label={t('storageItems.moveNamed', { name: item.name })} onClick={() => onMove(item)}>
                      <DriveFileMoveIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t('storageItems.delete')}>
                    <IconButton
                      size="small"
                      aria-label={t('storageItems.deleteNamed', { name: item.name })}
                      loading={busy(del, item)}
                      onClick={() => del.mutate(item.id)}
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </Stack>
              )
            }
            sx={{ pr: item.deletedAt ? 10 : 20 }}
          >
            <ListItemText
              primary={
                <Stack direction="row" spacing={1} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
                  <KeyText>{item.assetTag}</KeyText>
                  <span>{item.name}</span>
                  <Chip size="small" variant="outlined" label={item.type.name} />
                  {!item.usable && <Chip size="small" color="error" variant="outlined" label={t('storageItems.notUsable')} />}
                  {item.decommissionedAt && <Chip size="small" variant="outlined" label={t('storageItems.decommissionedBadge')} />}
                  {item.deletedAt && <Chip size="small" color="warning" variant="outlined" label={t('storageItems.deleted')} />}
                  {showStorage && <StorageChip id={item.storageId} />}
                </Stack>
              }
              secondary={itemSummary(t, item) || undefined}
              slotProps={{ secondary: { sx: { whiteSpace: 'pre-line', overflowWrap: 'anywhere' } } }}
            />
          </ListItem>
        ))}
      </List>
      {query.hasNextPage && (
        <Box>
          <Button size="small" loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        </Box>
      )}
    </>
  );
}

/** One line about an item (responsible person, model, host, primary MAC, interface count), then its description. */
function itemSummary(t: ReturnType<typeof useTranslation>['t'], item: StorageItem): string {
  const parts: string[] = [];
  const ifaces = item.modules.filter((m) => m.networkInterface);

  if (item.personResponsible) parts.push(item.personResponsible.displayName);

  for (const m of item.modules) {
    if (m.hardware) parts.push([m.hardware.manufacturer, m.hardware.model].filter(Boolean).join(' '));
    if (m.networkHost) parts.push([t(`modules.roles.${m.networkHost.role}`), m.networkHost.hostname].filter(Boolean).join(' '));
  }

  const primary = ifaces.find((m) => m.primary) ?? ifaces[0];
  if (primary?.networkInterface) parts.push(primary.networkInterface.mac);
  if (ifaces.length > 1) parts.push(t('modules.moreInterfaces', { count: ifaces.length }));

  const line = parts.filter(Boolean).join(' · ');

  return item.description ? `${line}\n${item.description}` : line;
}

/** The name of the storage an item is kept in. */
function StorageChip({ id }: { id: string }) {
  const storage = useStorage(id);

  if (!storage.data) return null;

  return <Chip size="small" variant="outlined" icon={<InventoryIcon />} label={storage.data.name} />;
}

/** Picks the storage to keep item in. */
function MoveItemDialog({ item, onClose }: { item: StorageItem; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateStorageItem();
  const [selected, setSelected] = useState<string>(item.storageId);

  const storageError = fieldErrorText(t, update.error, 'storageId');

  function submit() {
    update.mutate({ id: item.id, input: { storageId: selected } }, { onSuccess: onClose });
  }

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('storageItems.moveTitle', { name: item.name })}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 1 }}>{t('storageItems.moveHint')}</DialogContentText>
        <Paper variant="outlined" sx={{ maxHeight: '50vh', overflowY: 'auto' }}>
          <StorageTree parent={null} depth={0} picker={{ selected, onSelect: setSelected, excludeId: '' }} />
        </Paper>
        <Box sx={{ mt: 2 }}>
          {storageError ? <Typography color="error">{t('storageItems.storageGone')}</Typography> : <ErrorMessage error={update.error} />}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button variant="contained" disabled={selected === item.storageId} loading={update.isPending} onClick={submit}>
          {t('storageItems.moveHere')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** The issues of every project linking item, each opening the issue. */
function LinkedIssuesDialog({ item, onClose }: { item: StorageItem; onClose: () => void }) {
  const { t } = useTranslation();
  const issues = useStorageItemIssues(item.id);
  const list = issues.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('storageItems.linkedIssuesTitle', { name: item.name })}</DialogTitle>
      <DialogContent>
        {issues.isPending && <Loading />}
        <ErrorMessage error={issues.error} />
        {issues.isSuccess && list.length === 0 && <DialogContentText>{t('storageItems.noLinkedIssues')}</DialogContentText>}
        <List dense disablePadding>
          {list.map((issue) => (
            <ListItem key={issue.id} disablePadding divider>
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
        {issues.hasNextPage && (
          <Button size="small" loading={issues.isFetchingNextPage} onClick={() => void issues.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.close')}</Button>
      </DialogActions>
    </Dialog>
  );
}
