// In-place editors for one section of a storage item each: general fields,
// lifecycle dates, or a single module. Each saves only its own part.
import Alert from '@mui/material/Alert';
import Autocomplete from '@mui/material/Autocomplete';
import Button from '@mui/material/Button';
import FormControlLabel from '@mui/material/FormControlLabel';
import Grid from '@mui/material/Grid';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/client';
import { useItemType, useUpdateStorageItem, useUsers } from '../api/hooks';
import { isMultiModule, type ItemTypeRef, type ModuleDefault, type ModuleInput, type ModuleKind, type StorageItem, type UpdateStorageItemInput, type User } from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { ItemTypePicker } from '../components/ItemTypePicker';
import { ModuleCard } from '../components/ModuleEditors';
import { fieldErrorText } from '../lib/errors';
import { dateInput, moduleForm, moduleInput, type ModuleForm } from '../lib/moduleForm';

/** A timestamp as the value of a date input (yyyy-mm-dd), or empty. */
const dateValue = (at: string | null) => (at ? at.slice(0, 10) : '');

/** The item's stored modules as sent back to the API, unchanged. */
const storedModules = (item: StorageItem): ModuleInput[] => item.modules.map((m) => moduleInput(moduleForm(m)));

/** The kinds the defaults ask for that the item does not have enough of, one entry per missing instance. */
function missingKinds(defaults: ModuleDefault[], item: StorageItem): ModuleKind[] {
  return defaults.flatMap((d) => {
    const have = item.modules.filter((m) => m.kind === d.module).length;
    const want = isMultiModule(d.module) ? d.count : 1;

    return Array.from({ length: Math.max(0, want - have) }, () => d.module);
  });
}

type Update = ReturnType<typeof useUpdateStorageItem>;

/** The form around a section editor: its fields, errors and Cancel/Save. */
function SectionForm({
  update,
  onSubmit,
  onCancel,
  canSave = true,
  children,
}: {
  update: Update;
  onSubmit: () => void;
  onCancel: () => void;
  canSave?: boolean;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const err = update.error;
  const anyFieldError = err instanceof ApiError && Object.keys(err.fields).length > 0;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit();
  };

  return (
    <Stack component="form" spacing={2} onSubmit={submit} onKeyDown={(e) => e.key === 'Escape' && onCancel()}>
      {children}
      <ErrorMessage error={err && !anyFieldError ? err : null} />
      {anyFieldError && <Typography color="error">{t('errors.validation_failed')}</Typography>}
      <Stack direction="row" spacing={1} sx={{ justifyContent: 'flex-end' }}>
        <Button size="small" onClick={onCancel}>
          {t('app.cancel')}
        </Button>
        <Button size="small" type="submit" variant="contained" disabled={!canSave} loading={update.isPending}>
          {t('app.save')}
        </Button>
      </Stack>
    </Stack>
  );
}

interface EditorProps {
  item: StorageItem;
  onDone: () => void;
}

/** Saves input to item, ending the editor on success. */
function useSave(item: StorageItem, onDone: () => void) {
  const update = useUpdateStorageItem();
  const save = (input: UpdateStorageItemInput, onSaved?: () => void) =>
    update.mutate(
      { id: item.id, input },
      {
        onSuccess: () => {
          onSaved?.();
          onDone();
        },
      },
    );

  return { update, save };
}

/**
 * Name, description, type, usability and the person responsible. After a type
 * change, onAddModules gets the kinds the new type starts with that the item lacks.
 */
export function GeneralEditor({ item, onDone, onAddModules }: EditorProps & { onAddModules: (kinds: ModuleKind[]) => void }) {
  const { t } = useTranslation();
  const { update, save } = useSave(item, onDone);
  const users = useUsers();

  const [name, setName] = useState(item.name);
  const [description, setDescription] = useState(item.description);
  const [type, setType] = useState<ItemTypeRef | null>(item.type);
  const [usable, setUsable] = useState(item.usable);
  const [responsible, setResponsible] = useState<User | null>(item.personResponsible);

  // A changed type offers the modules it starts with that the item lacks.
  const typeChanged = !!type && type.id !== item.type.id;
  const typeView = useItemType(typeChanged ? type.id : null);
  const added = typeChanged && typeView.data?.id === type.id ? missingKinds(typeView.data.effectiveDefaults, item) : [];
  const typeReady = !typeChanged || typeView.data?.id === type.id;

  const fieldError = (field: string) => fieldErrorText(t, update.error, field);
  const people = (users.data ?? []).filter((u) => !u.serviceAccount);

  const submit = () => {
    if (!type) return;

    save({ name, description, typeId: type.id, usable, personResponsibleId: responsible?.id ?? null }, () => added.length > 0 && onAddModules(added));
  };

  return (
    <SectionForm update={update} onSubmit={submit} onCancel={onDone} canSave={!!type && typeReady}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 8 }}>
          <TextField
            fullWidth
            autoFocus
            required
            size="small"
            label={t('storageItems.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={!!fieldError('name')}
            helperText={fieldError('name')}
            slotProps={{ htmlInput: { maxLength: 100, autoComplete: 'off' } }}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }} sx={{ display: 'flex', alignItems: 'center' }}>
          <FormControlLabel control={<Switch checked={usable} onChange={(e) => setUsable(e.target.checked)} />} label={t('storageItems.usable')} />
        </Grid>
        <Grid size={{ xs: 12, sm: 6 }}>
          <ItemTypePicker id={`item-type-${item.id}`} value={type} onChange={setType} required error={fieldError('typeId')} />
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
      </Grid>
      {added.length > 0 && (
        <Alert severity="info">{t('storageItems.typeAddsModules', { kinds: added.map((k) => t(`modules.kinds.${k}`)).join(', ') })}</Alert>
      )}
    </SectionForm>
  );
}

/** Order, commissioning and decommissioning dates plus the decommissioning reason. */
export function LifecycleEditor({ item, onDone }: EditorProps) {
  const { t } = useTranslation();
  const { update, save } = useSave(item, onDone);

  const [orderDate, setOrderDate] = useState(dateValue(item.orderDate));
  const [commissionedAt, setCommissionedAt] = useState(dateValue(item.commissionedAt));
  const [decommissionedAt, setDecommissionedAt] = useState(dateValue(item.decommissionedAt));
  const [reason, setReason] = useState(item.decommissioningReason);

  const fieldError = (field: string) => fieldErrorText(t, update.error, field);

  const date = (label: string, value: string, set: (v: string) => void, field: string, autoFocus = false) => (
    <TextField
      fullWidth
      autoFocus={autoFocus}
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

  const submit = () =>
    save({
      orderDate: dateInput(orderDate),
      commissionedAt: dateInput(commissionedAt),
      decommissionedAt: dateInput(decommissionedAt),
      decommissioningReason: reason,
    });

  return (
    <SectionForm update={update} onSubmit={submit} onCancel={onDone}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 4 }}>{date(t('storageItems.orderDate'), orderDate, setOrderDate, 'orderDate', true)}</Grid>
        <Grid size={{ xs: 12, sm: 4 }}>{date(t('storageItems.commissionedAt'), commissionedAt, setCommissionedAt, 'commissionedAt')}</Grid>
        <Grid size={{ xs: 12, sm: 4 }}>{date(t('storageItems.decommissionedAt'), decommissionedAt, setDecommissionedAt, 'decommissionedAt')}</Grid>
        <Grid size={12}>
          <TextField
            fullWidth
            size="small"
            label={t('storageItems.decommissioningReason')}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            error={!!fieldError('decommissioningReason')}
            helperText={fieldError('decommissioningReason')}
            slotProps={{ htmlInput: { maxLength: 1000, autoComplete: 'off' } }}
          />
        </Grid>
      </Grid>
    </SectionForm>
  );
}

/**
 * One module: a stored one (by moduleId) or a new one of kind. Saving sends the
 * item's other modules unchanged; removing a stored module saves right away.
 */
export function ModuleEditor({ item, moduleId, kind, onDone }: EditorProps & { moduleId?: string; kind?: ModuleKind }) {
  const { update, save } = useSave(item, onDone);
  const [value, setValue] = useState<ModuleForm>(() => {
    const stored = item.modules.find((m) => m.id === moduleId);
    return moduleForm(stored ?? kind ?? 'hardware');
  });

  // Where the module sits in the submitted list, so field errors find it.
  const storedIndex = item.modules.findIndex((m) => m.id === value.id);
  const index = storedIndex >= 0 ? storedIndex : item.modules.length;

  const submit = () => {
    const others = item.modules.map((m, i) => {
      if (i === storedIndex) return moduleInput(value);
      const input = moduleInput(moduleForm(m));
      // At most one primary instance per kind.
      return value.primary && m.kind === value.kind ? { ...input, primary: false } : input;
    });
    const modules = storedIndex >= 0 ? others : [...others, moduleInput(value)];

    save({ modules });
  };

  const remove = () => {
    if (storedIndex < 0) return onDone();
    save({ modules: storedModules(item).filter((_, i) => i !== storedIndex) });
  };

  return (
    <SectionForm update={update} onSubmit={submit} onCancel={onDone}>
      <ModuleCard value={value} index={index} onChange={setValue} onRemove={remove} error={update.error} />
    </SectionForm>
  );
}
