import Autocomplete from '@mui/material/Autocomplete';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import Grid from '@mui/material/Grid';
import IconButton from '@mui/material/IconButton';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import StarIcon from '@mui/icons-material/StarBorder';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/client';
import { useCreateNetworkDevice, useUpdateNetworkDevice } from '../api/hooks';
import type {
  IPConfig,
  IPConfigInput,
  IPv4Mode,
  IPv6Mode,
  NetworkDevice,
  NetworkInterface,
  NetworkInterfaceInput,
  NetworkInterfaceType,
  NetworkRole,
  Storage,
} from '../api/types';
import { ErrorMessage } from '../components/ErrorMessage';
import { fieldErrorText, fieldErrorTextUnder } from '../lib/errors';

const roles: NetworkRole[] = ['router', 'switch', 'client'];
const interfaceTypes: NetworkInterfaceType[] = ['wired', 'wireless'];
const ipv4Modes: IPv4Mode[] = ['dhcp', 'static', 'off'];
const ipv6Modes: IPv6Mode[] = ['dhcp', 'slaac', 'static', 'off'];

/** One IP family as edited: numbers stay text until submit. */
interface IPForm<Mode extends string> {
  mode: Mode;
  address: string;
  prefixLength: string;
  gateway: string;
  dnsServers: string[];
}

interface InterfaceForm {
  /** Stable React key; interfaces are reordered when one becomes primary. */
  key: number;
  name: string;
  mac: string;
  type: NetworkInterfaceType;
  ssid: string;
  nativeVlan: string;
  taggedVlans: string[];
  ipv4: IPForm<IPv4Mode>;
  ipv6: IPForm<IPv6Mode>;
}

let nextKey = 0;

function ipForm<Mode extends string>(c: IPConfig<Mode>): IPForm<Mode> {
  return {
    mode: c.mode,
    address: c.address ?? '',
    prefixLength: c.prefixLength === null ? '' : String(c.prefixLength),
    gateway: c.gateway ?? '',
    dnsServers: c.dnsServers,
  };
}

function interfaceForm(n?: NetworkInterface): InterfaceForm {
  return {
    key: nextKey++,
    name: n?.name ?? '',
    mac: n?.mac ?? '',
    type: n?.type ?? 'wired',
    ssid: n?.ssid ?? '',
    nativeVlan: n?.nativeVlan == null ? '' : String(n.nativeVlan),
    taggedVlans: n?.taggedVlans.map(String) ?? [],
    ipv4: n ? ipForm(n.ipv4) : { mode: 'dhcp', address: '', prefixLength: '', gateway: '', dnsServers: [] },
    ipv6: n ? ipForm(n.ipv6) : { mode: 'off', address: '', prefixLength: '', gateway: '', dnsServers: [] },
  };
}

/** Text to a number, or null when empty; unparsable text becomes NaN, which the server rejects. */
function optionalNumber(s: string): number | null {
  return s.trim() === '' ? null : Number(s);
}

function ipInput<Mode extends string>(f: IPForm<Mode>): IPConfigInput<Mode> {
  if (f.mode !== 'static') return { mode: f.mode };

  return {
    mode: f.mode,
    address: f.address.trim() || null,
    prefixLength: optionalNumber(f.prefixLength),
    gateway: f.gateway.trim() || null,
    dnsServers: f.dnsServers,
  };
}

function interfaceInput(f: InterfaceForm): NetworkInterfaceInput {
  return {
    name: f.name,
    mac: f.mac,
    type: f.type,
    ssid: f.type === 'wireless' ? f.ssid : null,
    nativeVlan: optionalNumber(f.nativeVlan),
    taggedVlans: f.taggedVlans.map(Number),
    ipv4: ipInput(f.ipv4),
    ipv6: ipInput(f.ipv6),
  };
}

/** The API field path of the interface at index (0 is the primary one). */
const interfacePath = (index: number) => (index === 0 ? 'primaryInterface' : `secondaryInterfaces[${index - 1}]`);

/** Creates a network device in storage, or edits device. */
export function NetworkDeviceDialog({ storage, device, onClose }: { storage?: Storage; device?: NetworkDevice; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateNetworkDevice();
  const update = useUpdateNetworkDevice();
  const mutation = device ? update : create;

  const [name, setName] = useState(device?.name ?? '');
  const [description, setDescription] = useState(device?.description ?? '');
  const [role, setRole] = useState<NetworkRole>(device?.role ?? 'client');
  const [hostname, setHostname] = useState(device?.hostname ?? '');
  const [manufacturer, setManufacturer] = useState(device?.manufacturer ?? '');
  const [model, setModel] = useState(device?.model ?? '');
  const [serialNumber, setSerialNumber] = useState(device?.serialNumber ?? '');
  const [interfaces, setInterfaces] = useState<InterfaceForm[]>(() =>
    device ? [device.primaryInterface, ...device.secondaryInterfaces].map(interfaceForm) : [interfaceForm()],
  );

  const err = mutation.error;
  const fieldError = (field: string) => fieldErrorText(t, err, field);
  const storageError = fieldError('storageId');
  const anyFieldError = err instanceof ApiError && Object.keys(err.fields).length > 0;

  const changeInterface = (index: number, next: InterfaceForm) => setInterfaces((all) => all.map((f, i) => (i === index ? next : f)));
  const makePrimary = (index: number) =>
    setInterfaces((all) => [all[index]!, ...all.slice(0, index), ...all.slice(index + 1)]);
  const removeInterface = (index: number) => setInterfaces((all) => all.filter((_, i) => i !== index));

  function submit(e: FormEvent) {
    e.preventDefault();

    const [primary, ...secondary] = interfaces.map(interfaceInput);
    if (!primary) return;

    const input = {
      name,
      description,
      role,
      hostname,
      manufacturer,
      model,
      serialNumber,
      primaryInterface: primary,
      secondaryInterfaces: secondary,
    };

    if (device) update.mutate({ id: device.id, input }, { onSuccess: onClose });
    else if (storage) create.mutate({ storageId: storage.id, ...input }, { onSuccess: onClose });
  }

  const text = (label: string, value: string, set: (v: string) => void, field: string, maxLength: number, autoFocus = false) => (
    <TextField
      fullWidth
      autoFocus={autoFocus}
      required={field === 'name'}
      size="small"
      label={label}
      value={value}
      onChange={(e) => set(e.target.value)}
      error={!!fieldError(field)}
      helperText={fieldError(field)}
      slotProps={{ htmlInput: { maxLength, autoComplete: 'off' } }}
    />
  );

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md" slotProps={{ paper: { component: 'form', onSubmit: submit } }}>
      <DialogTitle>
        {device ? t('networkDevices.editTitle', { name: device.name }) : t('networkDevices.createTitle', { name: storage?.name ?? '' })}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={3} sx={{ pt: 1 }}>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 8 }}>{text(t('networkDevices.name'), name, setName, 'name', 100, true)}</Grid>
            <Grid size={{ xs: 12, sm: 4 }}>
              <TextField
                select
                fullWidth
                required
                size="small"
                label={t('networkDevices.role')}
                value={role}
                onChange={(e) => setRole(e.target.value as NetworkRole)}
                error={!!fieldError('role')}
                helperText={fieldError('role')}
              >
                {roles.map((r) => (
                  <MenuItem key={r} value={r}>
                    {t(`networkDevices.roles.${r}`)}
                  </MenuItem>
                ))}
              </TextField>
            </Grid>
            <Grid size={{ xs: 12, sm: 6 }}>{text(t('networkDevices.hostname'), hostname, setHostname, 'hostname', 253)}</Grid>
            <Grid size={{ xs: 12, sm: 6 }}>{text(t('networkDevices.serialNumber'), serialNumber, setSerialNumber, 'serialNumber', 100)}</Grid>
            <Grid size={{ xs: 12, sm: 6 }}>{text(t('networkDevices.manufacturer'), manufacturer, setManufacturer, 'manufacturer', 100)}</Grid>
            <Grid size={{ xs: 12, sm: 6 }}>{text(t('networkDevices.model'), model, setModel, 'model', 100)}</Grid>
            <Grid size={12}>
              <TextField
                fullWidth
                multiline
                minRows={2}
                size="small"
                label={t('networkDevices.description')}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                error={!!fieldError('description')}
                helperText={fieldError('description')}
              />
            </Grid>
          </Grid>

          <Stack spacing={2}>
            <Stack direction="row" sx={{ alignItems: 'center' }}>
              <Typography variant="subtitle1" component="h3" sx={{ flexGrow: 1 }}>
                {t('networkDevices.interfaces')}
              </Typography>
              <Button size="small" startIcon={<AddIcon />} onClick={() => setInterfaces((all) => [...all, interfaceForm()])}>
                {t('networkDevices.addInterface')}
              </Button>
            </Stack>
            {interfaces.map((f, index) => (
              <InterfaceEditor
                key={f.key}
                index={index}
                value={f}
                onChange={(next) => changeInterface(index, next)}
                onMakePrimary={() => makePrimary(index)}
                onRemove={() => removeInterface(index)}
                error={err}
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
        <Button type="submit" variant="contained" loading={mutation.isPending}>
          {device ? t('app.save') : t('app.create')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

interface InterfaceEditorProps {
  index: number;
  value: InterfaceForm;
  onChange: (next: InterfaceForm) => void;
  onMakePrimary: () => void;
  onRemove: () => void;
  error: unknown;
}

function InterfaceEditor({ index, value, onChange, onMakePrimary, onRemove, error }: InterfaceEditorProps) {
  const { t } = useTranslation();
  const path = interfacePath(index);
  const fieldError = (field: string) => fieldErrorText(t, error, `${path}.${field}`);
  const set = <K extends keyof InterfaceForm>(key: K, v: InterfaceForm[K]) => onChange({ ...value, [key]: v });
  const label = value.name || (index === 0 ? t('networkDevices.primary') : t('networkDevices.secondary', { n: index }));

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack spacing={2}>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Typography variant="subtitle2" sx={{ flexGrow: 1 }}>
            {index === 0 ? t('networkDevices.primary') : t('networkDevices.secondary', { n: index })}
          </Typography>
          {index === 0 ? (
            <Chip size="small" color="primary" variant="outlined" label={t('networkDevices.primaryBadge')} />
          ) : (
            <>
              <Tooltip title={t('networkDevices.makePrimary')}>
                <IconButton size="small" aria-label={t('networkDevices.makePrimaryNamed', { name: label })} onClick={onMakePrimary}>
                  <StarIcon fontSize="small" />
                </IconButton>
              </Tooltip>
              <Tooltip title={t('networkDevices.removeInterface')}>
                <IconButton size="small" aria-label={t('networkDevices.removeInterfaceNamed', { name: label })} onClick={onRemove}>
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </>
          )}
        </Stack>

        <Grid container spacing={2}>
          <Grid size={{ xs: 12, sm: 4 }}>
            <TextField
              fullWidth
              required
              size="small"
              label={t('networkDevices.interfaceName')}
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
              label={t('networkDevices.mac')}
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
              label={t('networkDevices.type')}
              value={value.type}
              onChange={(e) => set('type', e.target.value as NetworkInterfaceType)}
              error={!!fieldError('type')}
              helperText={fieldError('type')}
            >
              {interfaceTypes.map((type) => (
                <MenuItem key={type} value={type}>
                  {t(`networkDevices.types.${type}`)}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          {value.type === 'wireless' && (
            <Grid size={{ xs: 12, sm: 4 }}>
              <TextField
                fullWidth
                size="small"
                label={t('networkDevices.ssid')}
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
              label={t('networkDevices.nativeVlan')}
              value={value.nativeVlan}
              onChange={(e) => set('nativeVlan', e.target.value)}
              error={!!fieldError('nativeVlan')}
              helperText={fieldError('nativeVlan')}
              slotProps={{ htmlInput: { min: 1, max: 4094 } }}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: value.type === 'wireless' ? 5 : 8 }}>
            <ChipListInput
              label={t('networkDevices.taggedVlans')}
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
    </Paper>
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
          label={t(`networkDevices.${family}Mode`)}
          value={value.mode}
          onChange={(e) => set('mode', e.target.value as Mode)}
          error={!!fieldError('mode')}
          helperText={fieldError('mode')}
        >
          {modes.map((m) => (
            <MenuItem key={m} value={m}>
              {t(`networkDevices.modes.${m as IPv6Mode}`)}
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
              label={t('networkDevices.address')}
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
              label={t('networkDevices.prefixLength')}
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
              label={t('networkDevices.gateway')}
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
              label={t('networkDevices.dnsServers')}
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
          return <Chip key={key} size="small" label={entry} title={t('networkDevices.removeEntry', { entry })} {...itemProps} />;
        })
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          error={!!error}
          helperText={error ?? t('networkDevices.chipHint')}
          slotProps={{ ...params.slotProps, htmlInput: { ...params.slotProps.htmlInput, inputMode, spellCheck: false } }}
        />
      )}
    />
  );
}
