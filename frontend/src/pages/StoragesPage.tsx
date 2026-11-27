import Autocomplete from '@mui/material/Autocomplete';
import Badge from '@mui/material/Badge';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Collapse from '@mui/material/Collapse';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import Divider from '@mui/material/Divider';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import InputAdornment from '@mui/material/InputAdornment';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import ClearIcon from '@mui/icons-material/Clear';
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMoveOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import FilterListIcon from '@mui/icons-material/FilterList';
import HomeIcon from '@mui/icons-material/HomeOutlined';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import ItemIcon from '@mui/icons-material/WidgetsOutlined';
import MoreVertIcon from '@mui/icons-material/MoreVert';
import SearchIcon from '@mui/icons-material/Search';
import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import {
  useCreateStorage,
  useStorage,
  useStorageChildren,
  useStorageItems,
  useStorageItemSearch,
  useStoragePath,
  useUpdateStorage,
  useUpdateStorageItem,
  useUsers,
} from '../api/hooks';
import type { ItemTypeRef, Page, Storage, StorageItem, StorageItemFilter, User } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { ItemTypePicker } from '../components/ItemTypePicker';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { fieldErrorText } from '../lib/errors';
import { StorageItemDetails } from './StorageItemDetails';
import { StorageItemDialog } from './StorageItemDialog';

/** The dialog currently open on the page, if any. */
type DialogState =
  | { kind: 'create'; parent: Storage | null }
  | { kind: 'rename'; storage: Storage }
  | { kind: 'move'; storage: Storage }
  | { kind: 'createItem'; storage: Storage }
  | { kind: 'moveItem'; item: StorageItem };

/** What a storage can do, offered in its menu and overview. */
interface StorageActions {
  onAddItem: (storage: Storage) => void;
  onAddChild: (storage: Storage) => void;
  onRename: (storage: Storage) => void;
  onMove: (storage: Storage) => void;
}

/** The sidebar tree: storages with their items as leaves. */
interface Browse {
  storageId: string | null;
  itemId: string | null;
  onSelectStorage: (id: string) => void;
  onSelectItem: (id: string) => void;
  /** Narrows the items shown under each storage. */
  filter: StorageItemFilter;
  actions: StorageActions;
  /** Storages to open so the selection shows, e.g. after following a deep link; a new array opens them again. */
  reveal: readonly string[];
}

const nothingToReveal: readonly string[] = [];

/** Options of a tree used to pick a new parent instead of browsing. */
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
  picker?: Picker;
  browse?: Browse;
}

export function StoragesPage() {
  const { t } = useTranslation();
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const close = () => setDialog(null);
  // ?storage= or ?item= selects what the center shows, e.g. when following a link from an issue.
  const [params, setParams] = useSearchParams();
  const storageId = params.get('storage');
  const itemId = params.get('item');
  const [search, setSearch] = useState('');
  const [q, setQ] = useState('');
  const [filters, setFilters] = useState(noFilters);
  const [showFilters, setShowFilters] = useState(false);

  // Debounce the search box before querying.
  useEffect(() => {
    const timer = setTimeout(() => setQ(search.trim()), 300);
    return () => clearTimeout(timer);
  }, [search]);

  const selectStorage = (id: string) => {
    setParams({ storage: id });
    setSearch('');
    setQ('');
  };
  const selectItem = (id: string) => setParams({ item: id });

  const actions: StorageActions = {
    onAddItem: (storage) => setDialog({ kind: 'createItem', storage }),
    onAddChild: (parent) => setDialog({ kind: 'create', parent }),
    onRename: (storage) => setDialog({ kind: 'rename', storage }),
    onMove: (storage) => setDialog({ kind: 'move', storage }),
  };

  // Open the tree down to whatever is selected, wherever the selection came
  // from: a deep link, a search result or the storage chip of an item.
  const path = useStoragePath(itemId ? { item: itemId } : storageId ? { storage: storageId } : null);
  const reveal = path.data ?? nothingToReveal;

  const filter = toFilter(filters);
  const browse: Browse = { storageId, itemId, onSelectStorage: selectStorage, onSelectItem: selectItem, filter, actions, reveal };
  const activeFilters = countActive(filters);

  return (
    <Stack spacing={2}>
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

      <Box sx={{ display: 'flex', flexDirection: { xs: 'column', md: 'row' }, gap: 3, alignItems: { md: 'flex-start' } }}>
        <Paper
          component="nav"
          aria-label={t('storages.browse')}
          variant="outlined"
          sx={{
            width: { md: 340 },
            flexShrink: 0,
            display: 'flex',
            flexDirection: 'column',
            position: { md: 'sticky' },
            top: { md: 16 },
            maxHeight: { xs: '60vh', md: 'calc(100vh - 32px)' },
          }}
        >
          <Stack spacing={1} sx={{ p: 1.5 }}>
            <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
              <TextField
                type="search"
                size="small"
                fullWidth
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t('storageItems.searchShort')}
                slotProps={{
                  htmlInput: { maxLength: 200, 'aria-label': t('storageItems.searchLabel'), title: t('storageItems.searchPlaceholder') },
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <SearchIcon fontSize="small" />
                      </InputAdornment>
                    ),
                  },
                }}
              />
              <Tooltip title={t('storageItems.filters')}>
                <IconButton
                  aria-label={t('storageItems.filters')}
                  aria-expanded={showFilters}
                  color={showFilters || activeFilters > 0 ? 'primary' : 'default'}
                  onClick={() => setShowFilters((s) => !s)}
                >
                  <Badge badgeContent={activeFilters} color="primary">
                    <FilterListIcon />
                  </Badge>
                </IconButton>
              </Tooltip>
            </Stack>
            <Collapse in={showFilters}>
              <Box sx={{ pt: 1 }}>
                <ItemFilters value={filters} onChange={setFilters} />
              </Box>
            </Collapse>
          </Stack>
          <Divider />
          <Box sx={{ overflowY: 'auto', flexGrow: 1, py: 0.5 }}>
            {q ? (
              <SearchResults q={q} filter={filter} selectedId={itemId} onSelect={selectItem} onClear={() => setSearch('')} />
            ) : (
              <StorageTree parent={null} depth={0} browse={browse} />
            )}
          </Box>
        </Paper>

        <Box component="section" sx={{ flexGrow: 1, minWidth: 0 }}>
          {itemId ? (
            <StorageItemDetails
              key={itemId}
              id={itemId}
              onMove={(item) => setDialog({ kind: 'moveItem', item })}
              onSelectStorage={selectStorage}
            />
          ) : storageId ? (
            <StorageOverview key={storageId} storageId={storageId} filter={filter} actions={actions} onSelectItem={selectItem} />
          ) : (
            <Paper variant="outlined" sx={{ p: 4, textAlign: 'center' }}>
              <Typography color="text.secondary">{t('storageItems.pickItem')}</Typography>
            </Paper>
          )}
        </Box>
      </Box>

      {dialog?.kind === 'create' && <NameDialog parent={dialog.parent} onClose={close} />}
      {dialog?.kind === 'rename' && <NameDialog storage={dialog.storage} onClose={close} />}
      {dialog?.kind === 'move' && <MoveDialog storage={dialog.storage} onClose={close} />}
      {dialog?.kind === 'createItem' && (
        <StorageItemDialog storage={dialog.storage} onClose={close} onSaved={(item) => selectItem(item.id)} />
      )}
      {dialog?.kind === 'moveItem' && <MoveItemDialog item={dialog.item} onClose={close} />}
    </Stack>
  );
}

/** One level of the tree, loaded page by page; deeper levels load when expanded. */
function StorageTree({ parent, depth, picker, browse }: TreeProps) {
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
        <StorageNode key={s.id} storage={s} depth={depth} picker={picker} browse={browse} />
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

const indent = (depth: number) => 1 + depth * 2.5;

function StorageNode({ storage, depth, picker, browse }: { storage: Storage } & Omit<TreeProps, 'parent'>) {
  const { t } = useTranslation();
  const reveal = browse?.reveal ?? nothingToReveal;
  const [open, setOpen] = useState(() => reveal.includes(storage.id));
  // Opens the node when a new path to reveal runs through it; collapsing it
  // afterwards sticks until the selection changes. Adjusted while rendering.
  const [revealed, setRevealed] = useState(reveal);
  if (reveal !== revealed) {
    setRevealed(reveal);
    if (reveal.includes(storage.id)) setOpen(true);
  }
  const selectedRef = useScrollIntoView<HTMLDivElement>(!!browse && browse.storageId === storage.id && !browse.itemId);

  // In a picker, the moved storage's subtree stays closed: none of it can
  // become the new parent. While browsing, items count as content too; deleted
  // ones are not counted, so with them shown every storage may hold some.
  const excluded = picker?.excludeId === storage.id;
  const expandable = picker
    ? storage.childCount > 0 && !excluded
    : storage.childCount > 0 || storage.itemCount > 0 || !!browse?.filter.deleted;

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

  const icon = (
    <ListItemIcon sx={{ minWidth: 30 }}>
      <InventoryIcon fontSize="small" color={excluded ? 'disabled' : 'action'} />
    </ListItemIcon>
  );

  if (picker) {
    return (
      <>
        <ListItemButton
          selected={picker.selected === storage.id}
          disabled={excluded}
          onClick={() => picker.onSelect(storage.id)}
          sx={{ pl: indent(depth) }}
        >
          {toggle}
          {icon}
          <ListItemText primary={storage.name} />
          {storage.childCount > 0 && (
            <Chip size="small" variant="outlined" label={t('storages.children', { count: storage.childCount })} sx={{ mr: 1 }} />
          )}
        </ListItemButton>
        {expandable && (
          <Collapse in={open} unmountOnExit>
            <StorageTree parent={storage.id} depth={depth + 1} picker={picker} />
          </Collapse>
        )}
      </>
    );
  }

  if (!browse) return null;

  const selected = browse.storageId === storage.id && !browse.itemId;

  return (
    <>
      <ListItem disablePadding secondaryAction={<StorageMenu storage={storage} actions={browse.actions} />}>
        <ListItemButton
          ref={selectedRef}
          selected={selected}
          aria-current={selected ? 'true' : undefined}
          onClick={() => {
            browse.onSelectStorage(storage.id);
            setOpen(true);
          }}
          sx={{ pl: indent(depth), pr: 6 }}
        >
          {toggle}
          {icon}
          <ListItemText primary={storage.name} slotProps={{ primary: { noWrap: true } }} />
          {storage.itemCount > 0 && (
            <Typography variant="caption" color="text.secondary" sx={{ ml: 1 }} title={t('storages.items', { count: storage.itemCount })}>
              {storage.itemCount}
            </Typography>
          )}
        </ListItemButton>
      </ListItem>
      {expandable && (
        <Collapse in={open} unmountOnExit>
          {storage.childCount > 0 && <StorageTree parent={storage.id} depth={depth + 1} browse={browse} />}
          <StorageItemLeaves storage={storage} depth={depth + 1} browse={browse} />
        </Collapse>
      )}
    </>
  );
}

/** The add, rename and move actions of one storage. */
function StorageMenu({ storage, actions }: { storage: Storage; actions: StorageActions }) {
  const { t } = useTranslation();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  const entry = (label: string, icon: ReactNode, run: (s: Storage) => void) => (
    <MenuItem
      onClick={() => {
        setAnchor(null);
        run(storage);
      }}
    >
      <ListItemIcon>{icon}</ListItemIcon>
      <ListItemText>{label}</ListItemText>
    </MenuItem>
  );

  return (
    <>
      <IconButton size="small" edge="end" aria-label={t('storages.actionsFor', { name: storage.name })} onClick={(e) => setAnchor(e.currentTarget)}>
        <MoreVertIcon fontSize="small" />
      </IconButton>
      <Menu anchorEl={anchor} open={!!anchor} onClose={() => setAnchor(null)}>
        {entry(t('storageItems.add'), <ItemIcon fontSize="small" />, actions.onAddItem)}
        {entry(t('storages.addChild'), <AddIcon fontSize="small" />, actions.onAddChild)}
        {entry(t('storages.rename'), <EditIcon fontSize="small" />, actions.onRename)}
        {entry(t('storages.move'), <DriveFileMoveIcon fontSize="small" />, actions.onMove)}
      </Menu>
    </>
  );
}

/** The items directly in a storage, as leaves of the tree. */
function StorageItemLeaves({ storage, depth, browse }: { storage: Storage; depth: number; browse: Browse }) {
  const { t } = useTranslation();
  const query = useStorageItems(storage.id, browse.filter);
  const items = query.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <List dense disablePadding aria-label={t('storageItems.inStorage', { name: storage.name })}>
      {query.isPending && <Loading />}
      <ErrorMessage error={query.error} />
      {query.isSuccess && items.length === 0 && storage.childCount === 0 && (
        <ListItem sx={{ pl: indent(depth) + 3.5 }}>
          <ListItemText secondary={browse.filter.deleted ? t('storageItems.emptyDeleted') : t('storageItems.empty')} />
        </ListItem>
      )}
      {items.map((item) => (
        <ItemRow key={item.id} item={item} depth={depth} selected={browse.itemId === item.id} onSelect={browse.onSelectItem} />
      ))}
      {query.hasNextPage && (
        <ListItem sx={{ pl: indent(depth) + 3.5 }}>
          <Button size="small" loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        </ListItem>
      )}
    </List>
  );
}

/** One selectable item in the sidebar. */
function ItemRow({
  item,
  depth,
  selected,
  onSelect,
  showStorage = false,
}: {
  item: StorageItem;
  depth: number;
  selected: boolean;
  onSelect: (id: string) => void;
  showStorage?: boolean;
}) {
  const ref = useScrollIntoView<HTMLDivElement>(selected);

  return (
    <ListItemButton
      ref={ref}
      selected={selected}
      aria-current={selected ? 'true' : undefined}
      onClick={() => onSelect(item.id)}
      // Leaves line up with the storage names beside the expand toggles.
      sx={{ pl: indent(depth) + 3.5, opacity: item.deletedAt ? 0.6 : 1 }}
    >
      <ListItemIcon sx={{ minWidth: 30 }}>
        <ItemIcon fontSize="small" color={item.usable ? 'action' : 'error'} />
      </ListItemIcon>
      <ListItemText
        primary={item.name}
        secondary={
          <>
            <KeyText>{item.assetTag}</KeyText>
            {' · '}
            {item.type.name}
            {showStorage && (
              <>
                {' · '}
                <StorageName id={item.storageId} />
              </>
            )}
          </>
        }
        slotProps={{
          primary: { noWrap: true, sx: { textDecoration: item.deletedAt ? 'line-through' : undefined } },
          secondary: { noWrap: true },
        }}
      />
    </ListItemButton>
  );
}

/**
 * Scrolls the element into view when it becomes selected, also when it mounts
 * selected inside a node that is just opening; waits for the Collapse
 * animation so the final position is used.
 */
function useScrollIntoView<E extends HTMLElement>(selected: boolean) {
  const ref = useRef<E>(null);

  useEffect(() => {
    if (!selected) return;
    const timer = setTimeout(() => ref.current?.scrollIntoView({ block: 'nearest' }), 350);
    return () => clearTimeout(timer);
  }, [selected]);

  return ref;
}

/** The name of the storage an item is kept in. */
function StorageName({ id }: { id: string }) {
  const storage = useStorage(id);

  return <>{storage.data?.name ?? '…'}</>;
}

/** Items across all storages matching q and the filters, in the sidebar. */
function SearchResults({
  q,
  filter,
  selectedId,
  onSelect,
  onClear,
}: {
  q: string;
  filter: StorageItemFilter;
  selectedId: string | null;
  onSelect: (id: string) => void;
  onClear: () => void;
}) {
  const { t } = useTranslation();
  const query = useStorageItemSearch({ ...filter, q });
  const items = query.data?.pages.flatMap((p) => p.items) ?? [];
  const total = query.data?.pages[0]?.total ?? 0;

  return (
    <>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', px: 2, py: 0.5 }}>
        <Typography variant="body2" color="text.secondary" sx={{ flexGrow: 1 }}>
          {query.isSuccess && (items.length === 0 ? t('storageItems.noMatches') : t('storageItems.count', { count: total }))}
        </Typography>
        <Button size="small" startIcon={<ClearIcon />} onClick={onClear}>
          {t('storageItems.clearSearch')}
        </Button>
      </Stack>
      {query.isPending && <Loading />}
      <ErrorMessage error={query.error} />
      <List dense disablePadding aria-label={t('storageItems.searchTitle', { q })}>
        {items.map((item) => (
          <ItemRow key={item.id} item={item} depth={0} selected={selectedId === item.id} onSelect={onSelect} showStorage />
        ))}
      </List>
      {query.hasNextPage && (
        <Box sx={{ px: 2 }}>
          <Button size="small" loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        </Box>
      )}
    </>
  );
}

/** The center view of a selected storage: its actions and items, optionally with its subtree. */
function StorageOverview({
  storageId,
  filter,
  actions,
  onSelectItem,
}: {
  storageId: string;
  filter: StorageItemFilter;
  actions: StorageActions;
  onSelectItem: (id: string) => void;
}) {
  const { t } = useTranslation();
  const storage = useStorage(storageId);
  const [recursive, setRecursive] = useState(false);
  const items = useStorageItems(storageId, { ...filter, recursive });

  if (storage.isPending) return <Loading />;
  if (storage.isError) return <ErrorMessage error={storage.error} />;

  const st = storage.data;

  return (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Stack spacing={2}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ alignItems: { sm: 'flex-start' } }}>
          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            <Typography variant="h5" component="h2" sx={{ overflowWrap: 'anywhere' }}>
              {st.name}
            </Typography>
            <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
              <Chip size="small" variant="outlined" label={t('storages.children', { count: st.childCount })} />
              <Chip size="small" variant="outlined" color="primary" label={t('storages.items', { count: st.itemCount })} />
            </Stack>
          </Box>
          <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap', flexShrink: 0 }}>
            <Button size="small" variant="contained" startIcon={<AddIcon />} onClick={() => actions.onAddItem(st)}>
              {t('storageItems.add')}
            </Button>
            <Button size="small" variant="outlined" startIcon={<AddIcon />} onClick={() => actions.onAddChild(st)}>
              {t('storages.addChild')}
            </Button>
            <Tooltip title={t('storages.rename')}>
              <IconButton size="small" aria-label={t('storages.renameNamed', { name: st.name })} onClick={() => actions.onRename(st)}>
                <EditIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('storages.move')}>
              <IconButton size="small" aria-label={t('storages.moveNamed', { name: st.name })} onClick={() => actions.onMove(st)}>
                <DriveFileMoveIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Stack>
        </Stack>
        <Divider />
        <Stack direction="row" sx={{ alignItems: 'center' }}>
          <Typography variant="subtitle1" component="h3" sx={{ fontWeight: 600, flexGrow: 1 }}>
            {t('storageItems.inStorage', { name: st.name })}
          </Typography>
          <FormControlLabel
            control={<Switch size="small" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />}
            label={t('storageItems.recursive')}
          />
        </Stack>
        <ItemList
          query={items}
          showStorage={recursive}
          emptyText={filter.deleted ? t('storageItems.emptyDeleted') : t('storageItems.empty')}
          onSelect={onSelectItem}
        />
      </Stack>
    </Paper>
  );
}

/** The list filters, as edited. */
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

/** How many filters narrow the lists. */
const countActive = (f: FilterState) =>
  [f.type !== null, f.usable !== '', f.decommissioned !== '', f.responsible !== null, f.deleted].filter(Boolean).length;

/** Filters by type (with subtypes), usable, decommissioning, responsible person and deletion. */
function ItemFilters({ value, onChange }: { value: FilterState; onChange: (next: FilterState) => void }) {
  const { t } = useTranslation();
  const users = useUsers();
  const people = (users.data ?? []).filter((u) => !u.serviceAccount);
  const set = <K extends keyof FilterState>(key: K, v: FilterState[K]) => onChange({ ...value, [key]: v });

  const tri = (label: string, key: 'usable' | 'decommissioned') => (
    <TextField select fullWidth size="small" label={label} value={value[key]} onChange={(e) => set(key, e.target.value as FilterState[typeof key])}>
      <MenuItem value="">{t('storageItems.any')}</MenuItem>
      <MenuItem value="true">{t('storageItems.yes')}</MenuItem>
      <MenuItem value="false">{t('storageItems.no')}</MenuItem>
    </TextField>
  );

  return (
    <Stack spacing={1.5}>
      <ItemTypePicker id="filter-type" label={t('storageItems.filterType')} value={value.type} onChange={(v) => set('type', v)} />
      {value.type && (
        <FormControlLabel
          control={<Switch size="small" checked={value.includeSubtypes} onChange={(e) => set('includeSubtypes', e.target.checked)} />}
          label={t('storageItems.includeSubtypes')}
        />
      )}
      <Stack direction="row" spacing={1}>
        {tri(t('storageItems.filterUsable'), 'usable')}
        {tri(t('storageItems.filterDecommissioned'), 'decommissioned')}
      </Stack>
      <Autocomplete
        size="small"
        options={people}
        value={value.responsible}
        onChange={(_, u) => set('responsible', u)}
        getOptionLabel={(u) => u.displayName}
        getOptionKey={(u) => u.id}
        isOptionEqualToValue={(a, b) => a.id === b.id}
        renderInput={(params) => <TextField {...params} label={t('storageItems.filterResponsible')} />}
      />
      <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between' }}>
        <FormControlLabel
          control={<Switch size="small" checked={value.deleted} onChange={(e) => set('deleted', e.target.checked)} />}
          label={t('storageItems.showDeleted')}
        />
        {countActive(value) > 0 && (
          <Button size="small" onClick={() => onChange(noFilters)}>
            {t('storageItems.resetFilters')}
          </Button>
        )}
      </Stack>
    </Stack>
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

/** Items with a summary line each, selecting one on click. */
function ItemList({
  query,
  showStorage,
  emptyText,
  onSelect,
}: {
  query: ItemQuery;
  showStorage: boolean;
  emptyText: string;
  onSelect: (id: string) => void;
}) {
  const { t } = useTranslation();
  const items = query.data?.pages.flatMap((p) => p.items) ?? [];
  const total = query.data?.pages[0]?.total ?? 0;

  return (
    <>
      {query.isPending && <Loading />}
      <ErrorMessage error={query.error} />
      {query.isSuccess && (
        <Typography variant="body2" color="text.secondary">
          {items.length === 0 ? emptyText : t('storageItems.count', { count: total })}
        </Typography>
      )}
      {items.length > 0 && (
        <Paper variant="outlined">
          <List dense disablePadding>
            {items.map((item, i) => (
              <ListItem key={item.id} disablePadding divider={i < items.length - 1}>
                <ListItemButton onClick={() => onSelect(item.id)}>
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
                        <KeyText>{item.assetTag}</KeyText>
                        <span>{item.name}</span>
                        <Chip size="small" variant="outlined" label={item.type.name} />
                        {!item.usable && <Chip size="small" color="error" variant="outlined" label={t('storageItems.notUsable')} />}
                        {item.decommissionedAt && <Chip size="small" variant="outlined" label={t('storageItems.decommissionedBadge')} />}
                        {item.deletedAt && <Chip size="small" color="warning" variant="outlined" label={t('storageItems.deleted')} />}
                        {showStorage && <Chip size="small" variant="outlined" icon={<InventoryIcon />} label={<StorageName id={item.storageId} />} />}
                      </Stack>
                    }
                    secondary={itemSummary(t, item) || undefined}
                    slotProps={{ secondary: { sx: { whiteSpace: 'pre-line', overflowWrap: 'anywhere' } } }}
                  />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Paper>
      )}
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

/** One line about an item (responsible person, model, host, SIM phone number, mobile technology, operating system, SIP extension, primary MAC, interface count), then its description. */
function itemSummary(t: ReturnType<typeof useTranslation>['t'], item: StorageItem): string {
  const parts: string[] = [];
  const ifaces = item.modules.filter((m) => m.networkInterface);

  if (item.personResponsible) parts.push(item.personResponsible.displayName);

  for (const m of item.modules) {
    if (m.hardware) parts.push([m.hardware.manufacturer?.name, m.hardware.model?.name].filter(Boolean).join(' '));
    if (m.networkHost) parts.push([t(`modules.roles.${m.networkHost.role}`), m.networkHost.hostname].filter(Boolean).join(' '));
    if (m.simCard) parts.push(m.simCard.phoneNumber);
    if (m.wwan?.technology) parts.push(m.wwan.technology.toUpperCase());
    if (m.operatingSystem) parts.push([m.operatingSystem.name, m.operatingSystem.version].filter(Boolean).join(' '));
    if (m.sipClient?.extension) parts.push(t('modules.sipExtensionShort', { extension: m.sipClient.extension }));
  }

  const primary = ifaces.find((m) => m.primary) ?? ifaces[0];
  if (primary?.networkInterface) parts.push(primary.networkInterface.mac);
  if (ifaces.length > 1) parts.push(t('modules.moreInterfaces', { count: ifaces.length }));

  const line = parts.filter(Boolean).join(' · ');

  return item.description ? `${line}\n${item.description}` : line;
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
