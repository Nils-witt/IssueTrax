import Autocomplete from '@mui/material/Autocomplete';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControlLabel from '@mui/material/FormControlLabel';
import Grid from '@mui/material/Grid';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/client';
import { useCreateStorageItem, useItemType, useNextAssetTag, useUpdateStorageItem, useUsers } from '../api/hooks';
import {
  isMultiModule,
  moduleKinds,
  type ItemTypeRef,
  type ModuleDefault,
  type ModuleKind,
  type Storage,
  type StorageItem,
  type User,
} from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { ItemTypePicker } from '../components/ItemTypePicker';
import { ModuleCard } from '../components/ModuleEditors';
import { dateInput, moduleForm, moduleInput, type ModuleForm } from '../lib/moduleForm';
import { fieldErrorText } from '../lib/errors';

/** A timestamp as the value of a date input (yyyy-mm-dd), or empty. */
const dateValue = (at: string | null) => (at ? at.slice(0, 10) : '');

/** Module forms for defaults, each kind count times (single kinds once). */
function defaultForms(defaults: ModuleDefault[]): ModuleForm[] {
  return defaults.flatMap((d) => Array.from({ length: isMultiModule(d.module) ? d.count : 1 }, () => moduleForm(d.module)));
}

/** The defaults the modules do not cover yet, as new forms. */
function missingDefaults(defaults: ModuleDefault[], modules: ModuleForm[]): ModuleForm[] {
  return defaults.flatMap((d) => {
    const have = modules.filter((m) => m.kind === d.module).length;
    const want = isMultiModule(d.module) ? d.count : 1;

    return Array.from({ length: Math.max(0, want - have) }, () => moduleForm(d.module));
  });
}

/** Creates an item in storage, or edits item. */
export function StorageItemDialog({
  storage,
  item,
  onClose,
  onSaved,
}: {
  storage?: Storage;
  item?: StorageItem;
  onClose: () => void;
  /** Called with the saved item before the dialog closes. */
  onSaved?: (item: StorageItem) => void;
}) {
  const { t } = useTranslation();
  const create = useCreateStorageItem();
  const update = useUpdateStorageItem();
  const mutation = item ? update : create;
  const users = useUsers();

  const [type, setType] = useState<ItemTypeRef | null>(item?.type ?? null);
  // Null until the user edits it; until then a new item takes the proposal.
  const [editedTag, setEditedTag] = useState<string | null>(null);
  const [name, setName] = useState(item?.name ?? '');
  const [description, setDescription] = useState(item?.description ?? '');
  const [usable, setUsable] = useState(item?.usable ?? true);
  const [orderDate, setOrderDate] = useState(dateValue(item?.orderDate ?? null));
  const [commissionedAt, setCommissionedAt] = useState(dateValue(item?.commissionedAt ?? null));
  const [decommissionedAt, setDecommissionedAt] = useState(dateValue(item?.decommissionedAt ?? null));
  const [reason, setReason] = useState(item?.decommissioningReason ?? '');
  const [responsible, setResponsible] = useState<User | null>(item?.personResponsible ?? null);
  const [modules, setModules] = useState<ModuleForm[]>(() => (item ? item.modules.map(moduleForm) : []));
  // Keys of modules the form added for a type change, highlighted until saved.
  const [added, setAdded] = useState<Set<number>>(new Set());
  // The type whose defaults were applied, so they are applied once per pick.
  const [appliedType, setAppliedType] = useState<string | null>(item?.type.id ?? null);
  const [menu, setMenu] = useState<HTMLElement | null>(null);

  const typeView = useItemType(type?.id ?? null);
  const nextTag = useNextAssetTag(item ? null : (type?.id ?? null));

  const assetTag = editedTag ?? nextTag.data?.assetTag ?? '';

  // Once the picked type has loaded, a new item starts with its defaults; an
  // existing item keeps its modules and gets the missing defaults added.
  // Adjusting state while rendering runs once per picked type.
  const view = typeView.data;
  if (view && view.id === type?.id && view.id !== appliedType) {
    setAppliedType(view.id);

    if (item) {
      const extra = missingDefaults(view.effectiveDefaults, modules);
      setAdded(new Set(extra.map((m) => m.key)));
      setModules([...modules, ...extra]);
    } else {
      setModules(defaultForms(view.effectiveDefaults));
    }
  }

  const err = mutation.error;
  const fieldError = (field: string) => fieldErrorText(t, err, field);
  const storageError = fieldError('storageId');
  const anyFieldError = err instanceof ApiError && Object.keys(err.fields).length > 0;

  // Modules are shown and submitted in display order (by kind, then as added).
  const ordered = moduleKinds.flatMap((kind) => modules.filter((m) => m.kind === kind));

  const changeModule = (next: ModuleForm) =>
    setModules((all) =>
      all.map((m) => {
        if (m.key === next.key) return next;
        // At most one primary instance per kind.
        if (next.primary && m.kind === next.kind) return { ...m, primary: false };
        return m;
      }),
    );
  const removeModule = (key: number) => setModules((all) => all.filter((m) => m.key !== key));
  const addModule = (kind: ModuleKind) => {
    setModules((all) => [...all, moduleForm(kind)]);
    setMenu(null);
  };

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!type) return;

    const common = {
      name,
      description,
      typeId: type.id,
      usable,
      orderDate: dateInput(orderDate),
      commissionedAt: dateInput(commissionedAt),
      decommissionedAt: dateInput(decommissionedAt),
      decommissioningReason: reason,
      personResponsibleId: responsible?.id ?? null,
      modules: ordered.map(moduleInput),
    };

    const onSuccess = (saved: StorageItem) => {
      onSaved?.(saved);
      onClose();
    };

    if (item) update.mutate({ id: item.id, input: common }, { onSuccess });
    else if (storage) create.mutate({ ...common, assetTag, storageId: storage.id }, { onSuccess });
  }

  const text = (label: string, value: string, set: (v: string) => void, field: string, maxLength: number, autoFocus = false) => (
    <TextField
      fullWidth
      autoFocus={autoFocus}
      required={field === 'name' || field === 'assetTag'}
      size="small"
      label={label}
      value={value}
      onChange={(e) => set(e.target.value)}
      error={!!fieldError(field)}
      helperText={fieldError(field) ?? (field === 'assetTag' ? t('storageItems.assetTagHint') : undefined)}
      slotProps={{ htmlInput: { maxLength, autoComplete: 'off' } }}
    />
  );

  const date = (label: string, value: string, set: (v: string) => void, field: string) => (
    <TextField
      fullWidth
      type="date"
      size="small"
      label={label}
      value={value}
      onChange={(e) => set(e.target.value)}
      error={!!fieldError(field)}
      helperText={fieldError(field)}
      slotProps={{ inputLabel: { shrink: true } }}
    />
  );

  const people = (users.data ?? []).filter((u) => !u.serviceAccount);
  const present = new Set(modules.map((m) => m.kind));

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md" slotProps={{ paper: { component: 'form', onSubmit: submit } }}>
      <DialogTitle>
        {item ? t('storageItems.editTitle', { name: item.name }) : t('storageItems.createTitle', { name: storage?.name ?? '' })}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={3} sx={{ pt: 1 }}>
          <Typography variant="subtitle1" component="h3">
            {t('storageItems.general')}
          </Typography>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6 }}>
              <ItemTypePicker id="item-type" value={type} onChange={setType} required error={fieldError('typeId')} />
            </Grid>
            <Grid size={{ xs: 12, sm: 6 }}>
              {item ? (
                <TextField fullWidth size="small" label={t('storageItems.assetTag')} value={item.assetTag} disabled />
              ) : (
                text(
                  t('storageItems.assetTag'),
                  assetTag,
                  setEditedTag,
                  'assetTag',
                  50,
                )
              )}
            </Grid>
            <Grid size={{ xs: 12, sm: 8 }}>{text(t('storageItems.name'), name, setName, 'name', 100, true)}</Grid>
            <Grid size={{ xs: 12, sm: 4 }} sx={{ display: 'flex', alignItems: 'center' }}>
              <FormControlLabel control={<Switch checked={usable} onChange={(e) => setUsable(e.target.checked)} />} label={t('storageItems.usable')} />
            </Grid>
            <Grid size={12}>
              <TextField
                fullWidth
                multiline
                minRows={2}
                size="small"
                label={t('storageItems.description')}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                error={!!fieldError('description')}
                helperText={fieldError('description')}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6 }}>
              <Autocomplete
                size="small"
                options={people}
                value={responsible}
                onChange={(_, u) => setResponsible(u)}
                getOptionLabel={(u) => u.displayName}
                getOptionKey={(u) => u.id}
                isOptionEqualToValue={(a, b) => a.id === b.id}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    label={t('storageItems.personResponsible')}
                    placeholder={t('storageItems.nobody')}
                    error={!!fieldError('personResponsibleId')}
                    helperText={fieldError('personResponsibleId')}
                  />
                )}
              />
            </Grid>
          </Grid>

          <Typography variant="subtitle1" component="h3">
            {t('storageItems.lifecycle')}
          </Typography>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 4 }}>{date(t('storageItems.orderDate'), orderDate, setOrderDate, 'orderDate')}</Grid>
            <Grid size={{ xs: 12, sm: 4 }}>{date(t('storageItems.commissionedAt'), commissionedAt, setCommissionedAt, 'commissionedAt')}</Grid>
            <Grid size={{ xs: 12, sm: 4 }}>
              {date(t('storageItems.decommissionedAt'), decommissionedAt, setDecommissionedAt, 'decommissionedAt')}
            </Grid>
            <Grid size={12}>{text(t('storageItems.decommissioningReason'), reason, setReason, 'decommissioningReason', 1000)}</Grid>
          </Grid>

          <Stack spacing={2}>
            <Stack direction="row" sx={{ alignItems: 'center' }}>
              <Typography variant="subtitle1" component="h3" sx={{ flexGrow: 1 }}>
                {t('modules.title')}
              </Typography>
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
            </Stack>
            {added.size > 0 && <Alert severity="info">{t('storageItems.typeChangedHint')}</Alert>}
            {ordered.length === 0 && <Typography color="text.secondary">{t('modules.none')}</Typography>}
            {ordered.map((m, index) => (
              <ModuleCard
                key={m.key}
                value={m}
                index={index}
                onChange={changeModule}
                onRemove={() => removeModule(m.key)}
                error={err}
                highlight={added.has(m.key)}
              />
            ))}
          </Stack>

          <ErrorMessage error={err && !anyFieldError ? err : null} />
          {anyFieldError && !storageError && <Typography color="error">{t('errors.validation_failed')}</Typography>}
          {storageError && <Typography color="error">{t('storageItems.storageGone')}</Typography>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('app.cancel')}</Button>
        <Button type="submit" variant="contained" disabled={!type} loading={mutation.isPending}>
          {item ? t('app.save') : t('app.create')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
