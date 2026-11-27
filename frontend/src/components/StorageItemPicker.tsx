import Autocomplete from '@mui/material/Autocomplete';
import Chip from '@mui/material/Chip';
import type { SxProps, Theme } from '@mui/material/styles';
import TextField from '@mui/material/TextField';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import RouterIcon from '@mui/icons-material/RouterOutlined';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useStorageItemLookup } from '../api/hooks';
import type { StorageItemRef } from '../api/types';

interface StorageItemPickerProps {
  id: string;
  /** IDs of the selected items. */
  value: string[];
  onChange: (ids: string[]) => void;
  /** Items already known, such as the issue's current links, so their names show before any search. */
  known?: StorageItemRef[];
  label: string;
  error?: string;
  size?: 'small' | 'medium';
  sx?: SxProps<Theme>;
  autoFocus?: boolean;
}

/** Icon for an item's kind. */
export function StorageItemKindIcon({ item }: { item: Pick<StorageItemRef, 'kind'> }) {
  return item.kind === 'network_device' ? <RouterIcon fontSize="small" /> : <InventoryIcon fontSize="small" />;
}

/** Multi-select autocomplete of storage items of any kind, searched on the server as you type. */
export function StorageItemPicker({ id, value, onChange, known = [], label, error, size, sx, autoFocus }: StorageItemPickerProps) {
  const { t } = useTranslation();
  const [input, setInput] = useState('');
  const [q, setQ] = useState('');
  const lookup = useStorageItemLookup(q);

  // Debounce typing before searching.
  useEffect(() => {
    const timer = setTimeout(() => setQ(input.trim()), 250);
    return () => clearTimeout(timer);
  }, [input]);

  // Items picked here, so they keep their names when a new search no
  // longer returns them.
  const [picked, setPicked] = useState<StorageItemRef[]>([]);
  const options = lookup.data?.items ?? [];

  // An ID not known yet (say, from a bookmarked filter URL) shows as a
  // placeholder, so it can still be removed.
  const selected = value.map(
    (v) =>
      [...picked, ...known, ...options].find((i) => i.id === v) ?? { id: v, kind: 'generic' as const, storageId: '', name: '…', deleted: false },
  );

  return (
    <Autocomplete
      multiple
      id={id}
      size={size}
      sx={sx}
      options={options}
      value={selected}
      loading={lookup.isFetching}
      onChange={(_, next) => {
        setPicked(next);
        onChange(next.map((i) => i.id));
      }}
      inputValue={input}
      onInputChange={(_, v, reason) => {
        if (reason !== 'reset') setInput(v);
      }}
      getOptionLabel={(i) => i.name}
      getOptionKey={(i) => i.id}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      filterSelectedOptions
      // The server already filtered by the search term.
      filterOptions={(o) => o}
      noOptionsText={t('storageItems.noMatches')}
      renderOption={({ key, ...props }, i) => (
        <li key={key} {...props}>
          <StorageItemKindIcon item={i} />
          <span style={{ marginLeft: 8 }}>{i.name}</span>
        </li>
      )}
      renderValue={(items, getItemProps) =>
        items.map((i, index) => {
          const { key, ...itemProps } = getItemProps({ index });
          return <Chip key={key} size="small" icon={<StorageItemKindIcon item={i} />} label={i.name} {...itemProps} />;
        })
      }
      renderInput={(params) => <TextField {...params} label={label} autoFocus={autoFocus} error={!!error} helperText={error} />}
    />
  );
}
