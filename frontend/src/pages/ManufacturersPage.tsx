import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Collapse from '@mui/material/Collapse';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import InputAdornment from '@mui/material/InputAdornment';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import EditIcon from '@mui/icons-material/EditOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import FactoryIcon from '@mui/icons-material/FactoryOutlined';
import MemoryIcon from '@mui/icons-material/MemoryOutlined';
import RestoreIcon from '@mui/icons-material/RestoreFromTrashOutlined';
import SearchIcon from '@mui/icons-material/Search';
import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import {
  useCreateHardwareModel,
  useCreateManufacturer,
  useDeleteHardwareModel,
  useDeleteManufacturer,
  useHardwareModels,
  useManufacturers,
  useRenameHardwareModel,
  useRenameManufacturer,
  useRestoreHardwareModel,
  useRestoreManufacturer,
} from '../api/hooks';
import type { HardwareModel, Manufacturer } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { Loading } from '../components/Loading';
import { fieldErrorText } from '../lib/errors';

/** What the name dialog creates or renames. */
type DialogState =
  | { kind: 'createManufacturer' }
  | { kind: 'renameManufacturer'; manufacturer: Manufacturer }
  | { kind: 'createModel'; manufacturer: Manufacturer }
  | { kind: 'renameModel'; model: HardwareModel };

/** The central list of manufacturers and their models that hardware modules pick from. */
export function ManufacturersPage() {
  const { t } = useTranslation();
  const [dialog, setDialog] = useState<DialogState | null>(null);
  const [showDeleted, setShowDeleted] = useState(false);
  const [input, setInput] = useState('');
  const [q, setQ] = useState('');
  const makers = useManufacturers(q, showDeleted);
  const items = makers.data?.pages.flatMap((p) => p.items) ?? [];
  const del = useDeleteManufacturer();
  const restore = useRestoreManufacturer();

  useEffect(() => {
    const timer = setTimeout(() => setQ(input.trim()), 250);
    return () => clearTimeout(timer);
  }, [input]);

  return (
    <Stack spacing={3}>
      <Box>
        <Typography variant="h5" component="h1" gutterBottom>
          {t('catalog.title')}
        </Typography>
        <Typography color="text.secondary">{t('catalog.intro')}</Typography>
      </Box>
      <Stack direction="row" spacing={2} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
        <TextField
          size="small"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={t('catalog.search')}
          slotProps={{
            htmlInput: { 'aria-label': t('catalog.search') },
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon fontSize="small" />
                </InputAdornment>
              ),
            },
          }}
        />
        <FormControlLabel
          control={<Switch size="small" checked={showDeleted} onChange={(e) => setShowDeleted(e.target.checked)} />}
          label={t('catalog.showDeleted')}
        />
        <Box sx={{ flexGrow: 1 }} />
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialog({ kind: 'createManufacturer' })}>
          {t('catalog.newManufacturer')}
        </Button>
      </Stack>

      <ErrorMessage error={del.error ?? restore.error} />
      <Paper variant="outlined">
        <List dense disablePadding aria-label={t('catalog.title')}>
          {makers.isPending && <Loading />}
          <ErrorMessage error={makers.error} />
          {makers.isSuccess && items.length === 0 && (
            <ListItem>
              <ListItemText secondary={q ? t('catalog.noMatch') : t('catalog.noManufacturers')} />
            </ListItem>
          )}
          {items.map((m) => (
            <ManufacturerNode
              key={m.id}
              manufacturer={m}
              showDeleted={showDeleted}
              onDialog={setDialog}
              actions={
                <EntryActions
                  name={m.name}
                  deleted={!!m.deletedAt}
                  onRename={() => setDialog({ kind: 'renameManufacturer', manufacturer: m })}
                  onAdd={() => setDialog({ kind: 'createModel', manufacturer: m })}
                  onDelete={() => del.mutate(m.id)}
                  onRestore={() => restore.mutate(m.id)}
                  deleting={del.isPending && del.variables === m.id}
                  restoring={restore.isPending && restore.variables === m.id}
                />
              }
            />
          ))}
          {makers.hasNextPage && (
            <ListItem>
              <Button size="small" loading={makers.isFetchingNextPage} onClick={() => void makers.fetchNextPage()}>
                {t('app.loadMore')}
              </Button>
            </ListItem>
          )}
        </List>
      </Paper>

      {dialog && <NameDialog state={dialog} onClose={() => setDialog(null)} />}
    </Stack>
  );
}

/** Rename, add-model, delete and restore buttons of one entry. */
function EntryActions({
  name,
  deleted,
  onRename,
  onAdd,
  onDelete,
  onRestore,
  deleting,
  restoring,
}: {
  name: string;
  deleted: boolean;
  onRename: () => void;
  /** Adds a model; only manufacturers have it. */
  onAdd?: () => void;
  onDelete: () => void;
  onRestore: () => void;
  deleting: boolean;
  restoring: boolean;
}) {
  const { t } = useTranslation();

  if (deleted) {
    return (
      <Tooltip title={t('catalog.restore')}>
        <IconButton size="small" aria-label={t('catalog.restoreNamed', { name })} loading={restoring} onClick={onRestore}>
          <RestoreIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    );
  }

  return (
    <Stack direction="row" spacing={0.5}>
      {onAdd && (
        <Tooltip title={t('catalog.addModel')}>
          <IconButton size="small" aria-label={t('catalog.addModelTo', { name })} onClick={onAdd}>
            <AddIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      )}
      <Tooltip title={t('catalog.rename')}>
        <IconButton size="small" aria-label={t('catalog.renameNamed', { name })} onClick={onRename}>
          <EditIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t('catalog.delete')}>
        <IconButton size="small" aria-label={t('catalog.deleteNamed', { name })} loading={deleting} onClick={onDelete}>
          <DeleteIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </Stack>
  );
}

function EntryLabel({ icon, name, deleted, chips }: { icon: ReactNode; name: string; deleted: boolean; chips: ReactNode }) {
  const { t } = useTranslation();

  return (
    <>
      <ListItemIcon sx={{ minWidth: 32 }}>{icon}</ListItemIcon>
      <ListItemText
        primary={
          <Stack direction="row" spacing={1} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap', opacity: deleted ? 0.6 : 1 }}>
            <span>{name}</span>
            {deleted && <Chip size="small" color="warning" variant="outlined" label={t('catalog.deleted')} />}
            {chips}
          </Stack>
        }
      />
    </>
  );
}

function ManufacturerNode({
  manufacturer: m,
  showDeleted,
  onDialog,
  actions,
}: {
  manufacturer: Manufacturer;
  showDeleted: boolean;
  onDialog: (d: DialogState) => void;
  actions: ReactNode;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const deleted = !!m.deletedAt;

  return (
    <>
      <ListItem
        sx={{ pl: 1, pr: 16, '&:hover': { bgcolor: 'action.hover' } }}
        secondaryAction={actions}
        slotProps={{ secondaryAction: { sx: { right: 8 } } }}
      >
        <IconButton
          size="small"
          edge="start"
          aria-label={open ? t('catalog.collapse', { name: m.name }) : t('catalog.expand', { name: m.name })}
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
        >
          {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
        </IconButton>
        <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1 }}>
          <EntryLabel
            icon={<FactoryIcon fontSize="small" color={deleted ? 'disabled' : 'action'} />}
            name={m.name}
            deleted={deleted}
            chips={
              <>
                <Chip size="small" variant="outlined" label={t('catalog.models', { count: m.modelCount })} />
                {m.itemCount > 0 && <Chip size="small" variant="outlined" color="primary" label={t('catalog.items', { count: m.itemCount })} />}
              </>
            }
          />
        </Box>
      </ListItem>
      <Collapse in={open} unmountOnExit>
        <ModelList manufacturer={m} showDeleted={showDeleted} onDialog={onDialog} />
      </Collapse>
    </>
  );
}

function ModelList({
  manufacturer,
  showDeleted,
  onDialog,
}: {
  manufacturer: Manufacturer;
  showDeleted: boolean;
  onDialog: (d: DialogState) => void;
}) {
  const { t } = useTranslation();
  const models = useHardwareModels(manufacturer.id, '', showDeleted);
  const items = models.data?.pages.flatMap((p) => p.items) ?? [];
  const del = useDeleteHardwareModel();
  const restore = useRestoreHardwareModel();

  return (
    <List dense disablePadding aria-label={t('catalog.modelsOf', { name: manufacturer.name })}>
      {models.isPending && <Loading />}
      <ErrorMessage error={models.error ?? del.error ?? restore.error} />
      {models.isSuccess && items.length === 0 && (
        <ListItem sx={{ pl: 8 }}>
          <ListItemText secondary={t('catalog.noModels')} />
        </ListItem>
      )}
      {items.map((model) => (
        <ListItem
          key={model.id}
          sx={{ pl: 6, pr: 16, '&:hover': { bgcolor: 'action.hover' } }}
          secondaryAction={
            <EntryActions
              name={model.name}
              deleted={!!model.deletedAt}
              onRename={() => onDialog({ kind: 'renameModel', model })}
              onDelete={() => del.mutate(model.id)}
              onRestore={() => restore.mutate(model.id)}
              deleting={del.isPending && del.variables === model.id}
              restoring={restore.isPending && restore.variables === model.id}
            />
          }
          slotProps={{ secondaryAction: { sx: { right: 8 } } }}
        >
          <EntryLabel
            icon={<MemoryIcon fontSize="small" color={model.deletedAt ? 'disabled' : 'action'} />}
            name={model.name}
            deleted={!!model.deletedAt}
            chips={model.itemCount > 0 && <Chip size="small" variant="outlined" color="primary" label={t('catalog.items', { count: model.itemCount })} />}
          />
        </ListItem>
      ))}
      {models.hasNextPage && (
        <ListItem sx={{ pl: 6 }}>
          <Button size="small" loading={models.isFetchingNextPage} onClick={() => void models.fetchNextPage()}>
            {t('app.loadMore')}
          </Button>
        </ListItem>
      )}
    </List>
  );
}

/** Creates or renames a manufacturer or model. */
function NameDialog({ state, onClose }: { state: DialogState; onClose: () => void }) {
  const { t } = useTranslation();
  const createMaker = useCreateManufacturer();
  const renameMaker = useRenameManufacturer();
  const createModel = useCreateHardwareModel();
  const renameModel = useRenameHardwareModel();
  const initial = state.kind === 'renameManufacturer' ? state.manufacturer.name : state.kind === 'renameModel' ? state.model.name : '';
  const [name, setName] = useState(initial);

  const mutation = { createManufacturer: createMaker, renameManufacturer: renameMaker, createModel, renameModel }[state.kind];
  const title = {
    createManufacturer: () => t('catalog.newManufacturer'),
    renameManufacturer: () => t('catalog.renameTitle', { name: initial }),
    createModel: () => t('catalog.newModelTitle', { name: state.kind === 'createModel' ? state.manufacturer.name : '' }),
    renameModel: () => t('catalog.renameTitle', { name: initial }),
  }[state.kind]();
  const nameError = fieldErrorText(t, mutation.error, 'name');

  function submit(e: FormEvent) {
    e.preventDefault();

    const done = { onSuccess: onClose };

    switch (state.kind) {
      case 'createManufacturer':
        createMaker.mutate(name, done);
        break;
      case 'renameManufacturer':
        renameMaker.mutate({ id: state.manufacturer.id, name }, done);
        break;
      case 'createModel':
        createModel.mutate({ manufacturerId: state.manufacturer.id, name }, done);
        break;
      case 'renameModel':
        renameModel.mutate({ id: state.model.id, name }, done);
        break;
    }
  }

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="xs" slotProps={{ paper: { component: 'form', onSubmit: submit } }}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <TextField
            autoFocus
            required
            size="small"
            label={t('catalog.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={!!nameError}
            helperText={nameError}
            slotProps={{ htmlInput: { maxLength: 100, autoComplete: 'off' } }}
          />
          {!nameError && <ErrorMessage error={mutation.error} />}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button type="submit" variant="contained" loading={mutation.isPending}>
          {t('app.save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
