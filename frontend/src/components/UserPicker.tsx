import Autocomplete from '@mui/material/Autocomplete';
import Chip from '@mui/material/Chip';
import type { SxProps, Theme } from '@mui/material/styles';
import TextField from '@mui/material/TextField';
import { useTranslation } from 'react-i18next';
import type { User } from '../api/types';

interface UserPickerProps {
  id: string;
  users: User[];
  value: string[];
  onChange: (ids: string[]) => void;
  label: string;
  error?: string;
  size?: 'small' | 'medium';
  sx?: SxProps<Theme>;
  autoFocus?: boolean;
}

/** Multi-select autocomplete of users (only users who have signed in once exist). */
export function UserPicker({ id, users, value, onChange, label, error, size, sx, autoFocus }: UserPickerProps) {
  const { t } = useTranslation();
  const selected = users.filter((u) => value.includes(u.id));

  return (
    <Autocomplete
      multiple
      id={id}
      size={size}
      sx={sx}
      options={users}
      value={selected}
      onChange={(_, next) => onChange(next.map((u) => u.id))}
      getOptionLabel={(u) => u.displayName}
      getOptionKey={(u) => u.id}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      filterSelectedOptions
      // Match by e-mail too, so people with similar names are easy to tell apart.
      filterOptions={(options, { inputValue }) => {
        const q = inputValue.trim().toLowerCase();
        return options.filter((u) => u.displayName.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
      }}
      renderOption={({ key, ...props }, u) => (
        <li key={key} {...props}>
          <span>
            {u.displayName}
            <br />
            {/* Service accounts have no e-mail; label them instead. */}
            <small style={{ opacity: 0.7 }}>{u.serviceAccount ? t('serviceAccounts.bot') : u.email}</small>
          </span>
        </li>
      )}
      renderValue={(users, getItemProps) =>
        users.map((u, index) => {
          const { key, ...itemProps } = getItemProps({ index });
          return <Chip key={key} size="small" label={u.displayName} title={u.email} {...itemProps} />;
        })
      }
      renderInput={(params) => <TextField {...params} label={label} autoFocus={autoFocus} error={!!error} helperText={error} />}
    />
  );
}
