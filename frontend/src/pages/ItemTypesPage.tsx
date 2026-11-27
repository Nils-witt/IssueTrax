import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Collapse from '@mui/material/Collapse';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import CategoryIcon from '@mui/icons-material/CategoryOutlined';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMoveOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import HomeIcon from '@mui/icons-material/HomeOutlined';
import RestoreIcon from '@mui/icons-material/RestoreFromTrashOutlined';
import { useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/client';
import {
  useCreateItemType,
  useDeleteItemType,
  useItemType,
  useItemTypeChildren,
  useMe,
  useNextAssetTag,
  useRestoreItemType,
  useUpdateItemType,
} from '../api/hooks';
import { isMultiModule, moduleKinds, type ItemType, type ModuleDefault, type ModuleKind } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { Loading } from '../components/Loading';
import { fieldErrorText } from '../lib/errors';

type DialogState =
  | { kind: 'create'; parent: ItemType | null }
  | { kind: 'edit'; type: ItemType }
  | { kind: 'move'; type: ItemType };

interface Picker {
  /** Selected parent ID; null selects the top level. */
  selected: string | null;
  onSelect: (id: string) => void;
  /** The type being moved: it and its subtree cannot be picked. */
  excludeId: string;
}

interface TreeProps {
  parent: string | null;
  depth: number;
  includeDeleted: boolean;
  actions?: (type: ItemType) => ReactNode;
  picker?: Picker;
}

const indent = (depth: number) => 1 + depth * 3;

/** Merges inherited defaults with a type's own, like the server: own counts win, single modules once. */
function mergeDefaults(inherited: ModuleDefault[], own: ModuleDefault[]): ModuleDefault[] {
  const counts = new Map<ModuleKind, number>();

  for (const d of [...inherited, ...own]) counts.set(d.module, isMultiModule(d.module) ? d.count : 1);

  return moduleKinds.filter((k) => counts.has(k)).map((k) => ({ module: k, count: counts.get(k)! }));
}

export function ItemTypesPage() {
  const { t } = useTranslation();
  const me = useMe();
  const admin = !!me.data?.admin;
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const [showDeleted, setShowDeleted] = useState(false);
  const del = useDeleteItemType();
  const restore = useRestoreItemType();
  const close = () => setDialog(null);

  const actions = (type: ItemType) =>
    admin && (
      <Stack direction="row" spacing={0.5}>
        {type.deletedAt ? (
          <Tooltip title={t('itemTypes.restore')}>
            <IconButton
              size="small"
              aria-label={t('itemTypes.restoreNamed', { name: type.name })}
              loading={restore.isPending && restore.variables === type.id}
              onClick={() => restore.mutate(type.id)}
            >
              <RestoreIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        ) : (
          <>
            <Tooltip title={t('itemTypes.addChild')}>
              <IconButton size="small" aria-label={t('itemTypes.addChildTo', { name: type.name })} onClick={() => setDialog({ kind: 'create', parent: type })}>
                <AddIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('itemTypes.edit')}>
              <IconButton size="small" aria-label={t('itemTypes.editNamed', { name: type.name })} onClick={() => setDialog({ kind: 'edit', type })}>
                <EditIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('itemTypes.move')}>
              <IconButton size="small" aria-label={t('itemTypes.moveNamed', { name: type.name })} onClick={() => setDialog({ kind: 'move', type })}>
                <DriveFileMoveIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('itemTypes.delete')}>
              <IconButton
                size="small"
                aria-label={t('itemTypes.deleteNamed', { name: type.name })}
                loading={del.isPending && del.variables === type.id}
                onClick={() => del.mutate(type.id)}
              >
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </>
        )}
      </Stack>
    );

  return (
    <Stack spacing={3}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        <FormControlLabel
            control={<Switch size="small" checked={showDeleted} onChange={(e) => setShowDeleted(e.target.checked)} />}
            label={t('itemTypes.showDeleted')}
        />
        {admin && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialog({ kind: 'create', parent: null })}>
            {t('itemTypes.newRoot')}
          </Button>
        )}
      </Stack>
      {me.isSuccess && !admin && <Alert severity="info">{t('itemTypes.adminOnly')}</Alert>}

      <ErrorMessage error={del.error ?? restore.error} />
      <Paper variant="outlined">
        <TypeTree parent={null} depth={0} includeDeleted={showDeleted} actions={actions} />
      </Paper>

      {dialog?.kind === 'create' && <TypeDialog parent={dialog.parent} onClose={close} />}
      {dialog?.kind === 'edit' && <TypeDialog type={dialog.type} onClose={close} />}
      {dialog?.kind === 'move' && <MoveTypeDialog type={dialog.type} onClose={close} />}
    </Stack>
  );
}

/** One level of the type tree, loaded page by page; deeper levels load when expanded. */
function TypeTree({ parent, depth, includeDeleted, actions, picker }: TreeProps) {
  const { t } = useTranslation();
  const children = useItemTypeChildren(parent, includeDeleted);
  const types = children.data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <List dense disablePadding aria-label={depth === 0 ? t('itemTypes.title') : undefined}>
      {children.isPending && <Loading />}
      <ErrorMessage error={children.error} />
      {children.isSuccess && types.length === 0 && depth === 0 && (
        <ListItem>
          <ListItemText secondary={t('itemTypes.empty')} />
        </ListItem>
      )}
      {types.map((type) => (
        <TypeNode key={type.id} type={type} depth={depth} includeDeleted={includeDeleted} actions={actions} picker={picker} />
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

function TypeNode({ type, depth, includeDeleted, actions, picker }: { type: ItemType } & Omit<TreeProps, 'parent'>) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const deleted = !!type.deletedAt;
  // In a picker, the moved type's subtree stays closed and deleted types cannot be picked.
  const excluded = picker?.excludeId === type.id;
  const expandable = type.childCount > 0 && !excluded;

  const toggle = (
    <IconButton
      size="small"
      edge="start"
      aria-label={open ? t('itemTypes.collapse', { name: type.name }) : t('itemTypes.expand', { name: type.name })}
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

  const defaults = type.defaults.map((d) => (isMultiModule(d.module) ? `${t(`modules.kinds.${d.module}`)} ×${d.count}` : t(`modules.kinds.${d.module}`)));

  const label = (
    <>
      <ListItemIcon sx={{ minWidth: 32 }}>
        <CategoryIcon fontSize="small" color={deleted || excluded ? 'disabled' : 'action'} />
      </ListItemIcon>
      <ListItemText
        primary={
          <Stack direction="row" spacing={1} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap', opacity: deleted ? 0.6 : 1 }}>
            <span>{type.name}</span>
            {type.tagPrefix && <Chip size="small" variant="outlined" label={type.tagPrefix} />}
            {deleted && <Chip size="small" color="warning" variant="outlined" label={t('itemTypes.deleted')} />}
            {type.childCount > 0 && <Chip size="small" variant="outlined" label={t('itemTypes.children', { count: type.childCount })} />}
            {type.itemCount > 0 && <Chip size="small" variant="outlined" color="primary" label={t('itemTypes.items', { count: type.itemCount })} />}
          </Stack>
        }
        secondary={defaults.join(', ') || undefined}
      />
    </>
  );

  return (
    <>
      {picker ? (
        <ListItemButton selected={picker.selected === type.id} disabled={excluded || deleted} onClick={() => picker.onSelect(type.id)} sx={{ pl: indent(depth) }}>
          {toggle}
          {label}
        </ListItemButton>
      ) : (
        <ListItem
          sx={{ pl: indent(depth), pr: 22, '&:hover': { bgcolor: 'action.hover' } }}
          secondaryAction={actions?.(type)}
          slotProps={{ secondaryAction: { sx: { right: 8 } } }}
        >
          {toggle}
          <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1 }}>{label}</Box>
        </ListItem>
      )}
      {expandable && (
        <Collapse in={open} unmountOnExit>
          <TypeTree parent={type.id} depth={depth + 1} includeDeleted={includeDeleted} actions={actions} picker={picker} />
        </Collapse>
      )}
    </>
  );
}

/** Creates a type under parent, or edits type's name, prefix and defaults. */
function TypeDialog({ parent, type, onClose }: { parent?: ItemType | null; type?: ItemType; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateItemType();
  const update = useUpdateItemType();
  const mutation = type ? update : create;
  const [name, setName] = useState(type?.name ?? '');
  const [tagPrefix, setTagPrefix] = useState(type?.tagPrefix ?? '');
  const [defaults, setDefaults] = useState<ModuleDefault[]>(type?.defaults ?? []);
  const parentId = type ? type.parentId : (parent?.id ?? null);
  const parentView = useItemType(parentId);
  const nextTag = useNextAssetTag(type?.id ?? null);

  const err = mutation.error;
  const fieldError = (field: string) => fieldErrorText(t, err, field);
  const inherited = parentView.data?.effectiveDefaults ?? [];
  const effective = mergeDefaults(inherited, defaults);
  const used = new Set(defaults.map((d) => d.module));
  const free = moduleKinds.filter((k) => !used.has(k));
  const kindLabel = (d: ModuleDefault) => (isMultiModule(d.module) ? `${t(`modules.kinds.${d.module}`)} ×${d.count}` : t(`modules.kinds.${d.module}`));

  const setDefault = (index: number, next: ModuleDefault) => setDefaults((all) => all.map((d, i) => (i === index ? next : d)));

  function submit(e: FormEvent) {
    e.preventDefault();

    const input = { name, tagPrefix, defaults };

    if (type) update.mutate({ id: type.id, input }, { onSuccess: onClose });
    else create.mutate({ ...input, parentId }, { onSuccess: onClose });
  }

  const title = type ? t('itemTypes.editTitle', { name: type.name }) : parent ? t('itemTypes.createChildTitle', { name: parent.name }) : t('itemTypes.createRootTitle');

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm" slotProps={{ paper: { component: 'form', onSubmit: submit } }}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <TextField
            autoFocus
            required
            size="small"
            label={t('itemTypes.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={!!fieldError('name')}
            helperText={fieldError('name')}
            slotProps={{ htmlInput: { maxLength: 100, autoComplete: 'off' } }}
          />
          <TextField
            size="small"
            label={t('itemTypes.tagPrefix')}
            value={tagPrefix}
            onChange={(e) => setTagPrefix(e.target.value)}
            error={!!fieldError('tagPrefix')}
            helperText={
              fieldError('tagPrefix') ??
              [t('itemTypes.tagPrefixHint'), nextTag.data && t('itemTypes.nextTag', { tag: nextTag.data.assetTag })].filter(Boolean).join(' ')
            }
            slotProps={{ htmlInput: { maxLength: 20, autoComplete: 'off', spellCheck: false } }}
          />

          <Typography variant="subtitle2" component="h3">
            {t('itemTypes.defaults')}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t('itemTypes.defaultsHint')}
          </Typography>
          {inherited.length > 0 && parentView.data && (
            <Stack direction="row" spacing={0.5} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
              <Typography variant="body2">{t('itemTypes.inherited', { name: parentView.data.name })}</Typography>
              {inherited.map((d) => (
                <Chip key={d.module} size="small" label={kindLabel(d)} />
              ))}
            </Stack>
          )}
          {defaults.map((d, index) => (
            <Stack key={d.module} direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
              <TextField
                select
                size="small"
                label={t('modules.title')}
                value={d.module}
                onChange={(e) => setDefault(index, { module: e.target.value as ModuleKind, count: 1 })}
                error={!!fieldError(`defaults[${index}].module`)}
                helperText={fieldError(`defaults[${index}].module`)}
                sx={{ flexGrow: 1 }}
              >
                {moduleKinds
                  .filter((k) => k === d.module || !used.has(k))
                  .map((k) => (
                    <MenuItem key={k} value={k}>
                      {t(`modules.kinds.${k}`)}
                    </MenuItem>
                  ))}
              </TextField>
              <TextField
                size="small"
                type="number"
                label={t('itemTypes.count')}
                value={d.count}
                disabled={!isMultiModule(d.module)}
                onChange={(e) => setDefault(index, { ...d, count: Number(e.target.value) })}
                error={!!fieldError(`defaults[${index}].count`)}
                helperText={fieldError(`defaults[${index}].count`)}
                slotProps={{ htmlInput: { min: 1, max: 512 } }}
                sx={{ width: 110 }}
              />
              <Tooltip title={t('itemTypes.removeDefault', { name: t(`modules.kinds.${d.module}`) })}>
                <IconButton
                  aria-label={t('itemTypes.removeDefault', { name: t(`modules.kinds.${d.module}`) })}
                  onClick={() => setDefaults((all) => all.filter((_, i) => i !== index))}
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </Stack>
          ))}
          <Box>
            <Button
              size="small"
              startIcon={<AddIcon />}
              disabled={free.length === 0}
              onClick={() => setDefaults((all) => [...all, { module: free[0]!, count: 1 }])}
            >
              {t('itemTypes.addDefault')}
            </Button>
          </Box>
          <Typography variant="body2" color="text.secondary">
            {t('itemTypes.effective', { list: effective.map(kindLabel).join(', ') || t('itemTypes.noDefaults') })}
          </Typography>
          <ErrorMessage error={err && !(err instanceof ApiError && Object.keys(err.fields).length > 0) ? err : null} />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button type="submit" variant="contained" loading={mutation.isPending}>
          {type ? t('app.save') : t('app.create')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** Picks a new parent (or the top level) for type. */
function MoveTypeDialog({ type, onClose }: { type: ItemType; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateItemType();
  const [selected, setSelected] = useState<string | null>(type.parentId);

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{t('itemTypes.moveTitle', { name: type.name })}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 1 }}>{t('itemTypes.moveHint')}</DialogContentText>
        <Paper variant="outlined" sx={{ maxHeight: '50vh', overflowY: 'auto' }}>
          <List dense disablePadding>
            <ListItemButton selected={selected === null} onClick={() => setSelected(null)}>
              <ListItemIcon sx={{ minWidth: 32 }}>
                <HomeIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText primary={t('itemTypes.root')} />
            </ListItemButton>
          </List>
          <TypeTree parent={null} depth={1} includeDeleted picker={{ selected, onSelect: setSelected, excludeId: type.id }} />
        </Paper>
        <Box sx={{ mt: 2 }}>
          <ErrorMessage error={update.error} />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button
          variant="contained"
          disabled={selected === type.parentId}
          loading={update.isPending}
          onClick={() => update.mutate({ id: type.id, input: { parentId: selected } }, { onSuccess: onClose })}
        >
          {t('itemTypes.moveHere')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
