import Autocomplete from '@mui/material/Autocomplete';
import Chip from '@mui/material/Chip';
import FormControlLabel from '@mui/material/FormControlLabel';
import Grid from '@mui/material/Grid';
import IconButton from '@mui/material/IconButton';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isMultiModule, type IPv4Mode, type IPv6Mode, type ModuleKind, type NetworkInterfaceType, type NetworkRole } from '../api/types';
import { fieldErrorText, fieldErrorTextUnder } from '../lib/errors';
import type { InterfaceForm, IPForm, ModuleForm, PurchaseForm } from '../lib/moduleForm';

const roles: NetworkRole[] = ['router', 'switch', 'client'];
const interfaceTypes: NetworkInterfaceType[] = ['wired', 'wireless'];
const ipv4Modes: IPv4Mode[] = ['dhcp', 'static', 'off'];
const ipv6Modes: IPv6Mode[] = ['dhcp', 'slaac', 'static', 'off'];

/** The API field of a module's payload. */
const payloadField: Record<ModuleKind, string> = {
  hardware: 'hardware',
  network_host: 'networkHost',
  purchase: 'purchase',
  network_interface: 'networkInterface',
};

interface ModuleCardProps {
  value: ModuleForm;
  /** Index in the submitted modules list, for field error paths. */
  index: number;
  onChange: (next: ModuleForm) => void;
  onRemove: () => void;
  error: unknown;
  /** Highlights a module the form added on its own (e.g. after a type change). */
  highlight?: boolean;
}

/** One module instance with its kind's editor, a primary toggle for multi modules and a remove button. */
export function ModuleCard({ value, index, onChange, onRemove, error, highlight }: ModuleCardProps) {
  const { t } = useTranslation();
  const path = `modules[${index}].${payloadField[value.kind]}`;
  const label =
    value.kind === 'network_interface' && value.networkInterface.name
      ? `${t('modules.kinds.network_interface')} ${value.networkInterface.name}`
      : t(`modules.kinds.${value.kind}`);
  const moduleError = fieldErrorText(t, error, `modules[${index}].kind`) ?? fieldErrorText(t, error, `modules[${index}].id`);

  return (
    <Paper variant="outlined" sx={{ p: 2, borderColor: highlight ? 'warning.main' : undefined }}>
      <Stack spacing={2}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Typography variant="subtitle2" sx={{ flexGrow: 1 }}>
            {label}
          </Typography>
          {isMultiModule(value.kind) && (
            <FormControlLabel
              control={<Switch size="small" checked={value.primary} onChange={(e) => onChange({ ...value, primary: e.target.checked })} />}
              label={t('modules.primary')}
            />
          )}
          <Tooltip title={t('modules.remove')}>
            <IconButton size="small" aria-label={t('modules.removeNamed', { name: label })} onClick={onRemove}>
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Stack>
        {moduleError && <Typography color="error">{moduleError}</Typography>}
        {fieldErrorText(t, error, `modules[${index}].primary`) && (
          <Typography color="error">{fieldErrorText(t, error, `modules[${index}].primary`)}</Typography>
        )}
        {value.kind === 'hardware' && <HardwareEditor value={value} onChange={onChange} path={path} error={error} />}
        {value.kind === 'network_host' && <NetworkHostEditor value={value} onChange={onChange} path={path} error={error} />}
        {value.kind === 'purchase' && <PurchaseEditor value={value} onChange={onChange} path={path} error={error} />}
        {value.kind === 'network_interface' && (
          <InterfaceEditor
            value={value.networkInterface}
            onChange={(networkInterface) => onChange({ ...value, networkInterface })}
            path={path}
            error={error}
          />
        )}
      </Stack>
    </Paper>
  );
}

interface PayloadEditorProps {
  value: ModuleForm;
  onChange: (next: ModuleForm) => void;
  path: string;
  error: unknown;
}

/** A text field bound to one field of a module payload, showing its validation error. */
function PayloadText({
  label,
  value,
  onChange,
  field,
  error,
  maxLength = 100,
  ...rest
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  field: string;
  error: unknown;
  maxLength?: number;
  type?: string;
  required?: boolean;
  placeholder?: string;
}) {
  const { t } = useTranslation();
  const err = fieldErrorText(t, error, field);

  return (
    <TextField
      fullWidth
      size="small"
      label={label}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      error={!!err}
      helperText={err}
      slotProps={{ htmlInput: { maxLength, autoComplete: 'off' }, inputLabel: rest.type === 'date' ? { shrink: true } : undefined }}
      {...rest}
    />
  );
}

function HardwareEditor({ value, onChange, path, error }: PayloadEditorProps) {
  const { t } = useTranslation();
  const set = (key: keyof ModuleForm['hardware']) => (v: string) => onChange({ ...value, hardware: { ...value.hardware, [key]: v } });

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, sm: 4 }}>
        <PayloadText label={t('modules.manufacturer')} value={value.hardware.manufacturer} onChange={set('manufacturer')} field={`${path}.manufacturer`} error={error} />
      </Grid>
      <Grid size={{ xs: 12, sm: 4 }}>
        <PayloadText label={t('modules.model')} value={value.hardware.model} onChange={set('model')} field={`${path}.model`} error={error} />
      </Grid>
      <Grid size={{ xs: 12, sm: 4 }}>
        <PayloadText label={t('modules.serialNumber')} value={value.hardware.serialNumber} onChange={set('serialNumber')} field={`${path}.serialNumber`} error={error} />
      </Grid>
    </Grid>
  );
}

function NetworkHostEditor({ value, onChange, path, error }: PayloadEditorProps) {
  const { t } = useTranslation();
  const roleError = fieldErrorText(t, error, `${path}.role`);

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, sm: 4 }}>
        <TextField
          select
          fullWidth
          required
          size="small"
          label={t('modules.role')}
          value={value.networkHost.role}
          onChange={(e) => onChange({ ...value, networkHost: { ...value.networkHost, role: e.target.value as NetworkRole } })}
          error={!!roleError}
          helperText={roleError}
        >
          {roles.map((r) => (
            <MenuItem key={r} value={r}>
              {t(`modules.roles.${r}`)}
            </MenuItem>
          ))}
        </TextField>
      </Grid>
      <Grid size={{ xs: 12, sm: 8 }}>
        <PayloadText
          label={t('modules.hostname')}
          value={value.networkHost.hostname}
          onChange={(hostname) => onChange({ ...value, networkHost: { ...value.networkHost, hostname } })}
          field={`${path}.hostname`}
          error={error}
          maxLength={253}
        />
      </Grid>
    </Grid>
  );
}

function PurchaseEditor({ value, onChange, path, error }: PayloadEditorProps) {
  const { t } = useTranslation();
  const set = (key: keyof PurchaseForm) => (v: string) => onChange({ ...value, purchase: { ...value.purchase, [key]: v } });

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, sm: 6 }}>
        <PayloadText label={t('modules.supplier')} value={value.purchase.supplier} onChange={set('supplier')} field={`${path}.supplier`} error={error} />
      </Grid>
      <Grid size={{ xs: 12, sm: 6 }}>
        <PayloadText label={t('modules.invoiceNumber')} value={value.purchase.invoiceNumber} onChange={set('invoiceNumber')} field={`${path}.invoiceNumber`} error={error} />
      </Grid>
      <Grid size={{ xs: 8, sm: 4 }}>
        <PayloadText label={t('modules.price')} value={value.purchase.price} onChange={set('price')} field={`${path}.price`} error={error} maxLength={16} placeholder="1299.00" />
      </Grid>
      <Grid size={{ xs: 4, sm: 3 }}>
        <PayloadText
          label={t('modules.currency')}
          value={value.purchase.currency}
          onChange={(v) => set('currency')(v.toUpperCase())}
          field={`${path}.currency`}
          error={error}
          maxLength={3}
          placeholder="EUR"
        />
      </Grid>
      <Grid size={{ xs: 12, sm: 5 }}>
        <PayloadText label={t('modules.warrantyEnd')} type="date" value={value.purchase.warrantyEnd} onChange={set('warrantyEnd')} field={`${path}.warrantyEnd`} error={error} />
      </Grid>
    </Grid>
  );
}

interface InterfaceEditorProps {
  value: InterfaceForm;
  onChange: (next: InterfaceForm) => void;
  path: string;
  error: unknown;
}

function InterfaceEditor({ value, onChange, path, error }: InterfaceEditorProps) {
  const { t } = useTranslation();
  const fieldError = (field: string) => fieldErrorText(t, error, `${path}.${field}`);
  const set = <K extends keyof InterfaceForm>(key: K, v: InterfaceForm[K]) => onChange({ ...value, [key]: v });

  return (
    <Stack spacing={2}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 4 }}>
          <TextField
            fullWidth
            required
            size="small"
            label={t('modules.interfaceName')}
            placeholder="eth0"
            value={value.name}
            onChange={(e) => set('name', e.target.value)}
            error={!!fieldError('name')}
            helperText={fieldError('name')}
            slotProps={{ htmlInput: { maxLength: 100, autoComplete: 'off' } }}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 5 }}>
          <TextField
            fullWidth
            required
            size="small"
            label={t('modules.mac')}
            placeholder="aa:bb:cc:dd:ee:ff"
            value={value.mac}
            onChange={(e) => set('mac', e.target.value)}
            error={!!fieldError('mac')}
            helperText={fieldError('mac')}
            slotProps={{ htmlInput: { maxLength: 17, autoComplete: 'off', spellCheck: false } }}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 3 }}>
          <TextField
            select
            fullWidth
            required
            size="small"
            label={t('modules.type')}
            value={value.type}
            onChange={(e) => set('type', e.target.value as NetworkInterfaceType)}
            error={!!fieldError('type')}
            helperText={fieldError('type')}
          >
            {interfaceTypes.map((type) => (
              <MenuItem key={type} value={type}>
                {t(`modules.types.${type}`)}
              </MenuItem>
            ))}
          </TextField>
        </Grid>
        {value.type === 'wireless' && (
          <Grid size={{ xs: 12, sm: 4 }}>
            <TextField
              fullWidth
              size="small"
              label={t('modules.ssid')}
              value={value.ssid}
              onChange={(e) => set('ssid', e.target.value)}
              error={!!fieldError('ssid')}
              helperText={fieldError('ssid')}
              slotProps={{ htmlInput: { maxLength: 32, autoComplete: 'off' } }}
            />
          </Grid>
        )}
        <Grid size={{ xs: 12, sm: value.type === 'wireless' ? 3 : 4 }}>
          <TextField
            fullWidth
            size="small"
            type="number"
            label={t('modules.nativeVlan')}
            value={value.nativeVlan}
            onChange={(e) => set('nativeVlan', e.target.value)}
            error={!!fieldError('nativeVlan')}
            helperText={fieldError('nativeVlan')}
            slotProps={{ htmlInput: { min: 1, max: 4094 } }}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: value.type === 'wireless' ? 5 : 8 }}>
          <ChipListInput
            label={t('modules.taggedVlans')}
            value={value.taggedVlans}
            onChange={(v) => set('taggedVlans', v)}
            error={fieldErrorTextUnder(t, error, `${path}.taggedVlans`)}
            inputMode="numeric"
          />
        </Grid>
      </Grid>

      <IPEditor
        family="ipv4"
        modes={ipv4Modes}
        value={value.ipv4}
        onChange={(v) => set('ipv4', v)}
        path={`${path}.ipv4`}
        error={error}
      />
      <IPEditor
        family="ipv6"
        modes={ipv6Modes}
        value={value.ipv6}
        onChange={(v) => set('ipv6', v)}
        path={`${path}.ipv6`}
        error={error}
      />
    </Stack>
  );
}


interface IPEditorProps<Mode extends string> {
  family: 'ipv4' | 'ipv6';
  modes: Mode[];
  value: IPForm<Mode>;
  onChange: (next: IPForm<Mode>) => void;
  path: string;
  error: unknown;
}

function IPEditor<Mode extends string>({ family, modes, value, onChange, path, error }: IPEditorProps<Mode>) {
  const { t } = useTranslation();
  const fieldError = (field: string) => fieldErrorText(t, error, `${path}.${field}`);
  const set = <K extends keyof IPForm<Mode>>(key: K, v: IPForm<Mode>[K]) => onChange({ ...value, [key]: v });
  const isStatic = value.mode === 'static';
  const placeholder = family === 'ipv4' ? { address: '192.168.1.10', prefix: '24', gateway: '192.168.1.1' } : { address: '2001:db8::10', prefix: '64', gateway: 'fe80::1' };

  return (
    <Grid container spacing={2} sx={{ alignItems: 'flex-start' }}>
      <Grid size={{ xs: 12, sm: 3 }}>
        <TextField
          select
          fullWidth
          required
          size="small"
          label={t(`modules.${family}Mode`)}
          value={value.mode}
          onChange={(e) => set('mode', e.target.value as Mode)}
          error={!!fieldError('mode')}
          helperText={fieldError('mode')}
        >
          {modes.map((m) => (
            <MenuItem key={m} value={m}>
              {t(`modules.modes.${m as IPv6Mode}`)}
            </MenuItem>
          ))}
        </TextField>
      </Grid>
      {isStatic && (
        <>
          <Grid size={{ xs: 8, sm: 4 }}>
            <TextField
              fullWidth
              required
              size="small"
              label={t('modules.address')}
              placeholder={placeholder.address}
              value={value.address}
              onChange={(e) => set('address', e.target.value)}
              error={!!fieldError('address')}
              helperText={fieldError('address')}
              slotProps={{ htmlInput: { autoComplete: 'off', spellCheck: false } }}
            />
          </Grid>
          <Grid size={{ xs: 4, sm: 2 }}>
            <TextField
              fullWidth
              required
              size="small"
              type="number"
              label={t('modules.prefixLength')}
              placeholder={placeholder.prefix}
              value={value.prefixLength}
              onChange={(e) => set('prefixLength', e.target.value)}
              error={!!fieldError('prefixLength')}
              helperText={fieldError('prefixLength')}
              slotProps={{ htmlInput: { min: 0, max: family === 'ipv4' ? 32 : 128 } }}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: 3 }}>
            <TextField
              fullWidth
              size="small"
              label={t('modules.gateway')}
              placeholder={placeholder.gateway}
              value={value.gateway}
              onChange={(e) => set('gateway', e.target.value)}
              error={!!fieldError('gateway')}
              helperText={fieldError('gateway')}
              slotProps={{ htmlInput: { autoComplete: 'off', spellCheck: false } }}
            />
          </Grid>
          <Grid size={12}>
            <ChipListInput
              label={t('modules.dnsServers')}
              value={value.dnsServers}
              onChange={(v) => set('dnsServers', v)}
              error={fieldErrorTextUnder(t, error, `${path}.dnsServers`)}
            />
          </Grid>
        </>
      )}
    </Grid>
  );
}

/** Free text entries as chips: Enter, comma or space adds the draft, and so does leaving the field. */
function ChipListInput({
  label,
  value,
  onChange,
  error,
  inputMode,
}: {
  label: string;
  value: string[];
  onChange: (v: string[]) => void;
  error?: string;
  inputMode?: 'numeric';
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');

  function add(raw: string) {
    const entry = raw.trim();
    if (entry && !value.includes(entry)) onChange([...value, entry]);
    setDraft('');
  }

  return (
    <Autocomplete
      multiple
      freeSolo
      size="small"
      options={[]}
      value={value}
      inputValue={draft}
      onInputChange={(_, v, reason) => {
        if (reason === 'input') setDraft(v);
      }}
      onChange={(_, next, reason) => {
        if (reason === 'createOption') add(String(next[next.length - 1] ?? ''));
        else onChange(next.map(String));
      }}
      onKeyDown={(e) => {
        if (e.key === ',' || e.key === ' ') {
          e.preventDefault();
          add(draft);
        }
      }}
      onBlur={() => add(draft)}
      renderValue={(entries, getItemProps) =>
        entries.map((entry, index) => {
          const { key, ...itemProps } = getItemProps({ index });
          return <Chip key={key} size="small" label={entry} title={t('modules.removeEntry', { entry })} {...itemProps} />;
        })
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          error={!!error}
          helperText={error ?? t('modules.chipHint')}
          slotProps={{ ...params.slotProps, htmlInput: { ...params.slotProps.htmlInput, inputMode, spellCheck: false } }}
        />
      )}
    />
  );
}
