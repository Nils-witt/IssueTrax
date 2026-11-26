import Box from '@mui/material/Box';
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
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DriveFileMoveIcon from '@mui/icons-material/DriveFileMoveOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import HomeIcon from '@mui/icons-material/HomeOutlined';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import { useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useCreateStorage, useStorageChildren, useUpdateStorage } from '../api/hooks';
import type { Storage } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { Loading } from '../components/Loading';
import { fieldErrorText } from '../lib/errors';

/** The dialog currently open on the page, if any. */
type DialogState =
  | { kind: 'create'; parent: Storage | null }
  | { kind: 'rename'; storage: Storage }
  | { kind: 'move'; storage: Storage };

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
}

export function StoragesPage() {
  const { t } = useTranslation();
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const close = () => setDialog(null);

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

      <Paper variant="outlined">
        <StorageTree parent={null} depth={0} actions={actions} />
      </Paper>

      {dialog?.kind === 'create' && <NameDialog parent={dialog.parent} onClose={close} />}
      {dialog?.kind === 'rename' && <NameDialog storage={dialog.storage} onClose={close} />}
      {dialog?.kind === 'move' && <MoveDialog storage={dialog.storage} onClose={close} />}
    </Stack>
  );
}

/** One level of the tree, loaded page by page; deeper levels load when expanded. */
function StorageTree({ parent, depth, actions, picker }: TreeProps) {
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
        <StorageNode key={s.id} storage={s} depth={depth} actions={actions} picker={picker} />
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

function StorageNode({ storage, depth, actions, picker }: { storage: Storage } & Omit<TreeProps, 'parent'>) {
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
          <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1, pr: 14 }}>{label}</Box>
        </ListItem>
      )}
      {expandable && (
        <Collapse in={open} unmountOnExit>
          <StorageTree parent={storage.id} depth={depth + 1} actions={actions} picker={picker} />
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
