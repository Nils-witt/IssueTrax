import Autocomplete from '@mui/material/Autocomplete';
import type { SxProps, Theme } from '@mui/material/styles';
import TextField from '@mui/material/TextField';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useItemTypeChildren, useItemTypeSearch } from '../api/hooks';
import type { ItemType, ItemTypeRef, ItemTypeView } from '../api/types';

/** A pickable type with the names of its ancestors, if known. */
interface Option {
  id: string;
  name: string;
  path: string[];
}

const toOption = (t: ItemType | ItemTypeView | ItemTypeRef): Option => ({
  id: t.id,
  name: t.name,
  path: 'path' in t ? t.path.map((p) => p.name) : [],
});

const optionLabel = (o: Option) => [...o.path, o.name].join(' › ');

interface ItemTypePickerProps {
  id: string;
  value: ItemTypeRef | null;
  onChange: (type: ItemTypeRef | null) => void;
  label?: string;
  required?: boolean;
  error?: string;
  size?: 'small' | 'medium';
  sx?: SxProps<Theme>;
}

/**
 * Picks one live item type. Without input it offers the root types; typing
 * searches the whole tree on the server and shows each match with its path.
 */
export function ItemTypePicker({ id, value, onChange, label, required, error, size = 'small', sx }: ItemTypePickerProps) {
  const { t } = useTranslation();
  const [input, setInput] = useState('');
  const [q, setQ] = useState('');
  const search = useItemTypeSearch(q);
  const roots = useItemTypeChildren(null, false, q === '');

  // Debounce typing before searching.
  useEffect(() => {
    const timer = setTimeout(() => setQ(input.trim()), 250);
    return () => clearTimeout(timer);
  }, [input]);

  const options: Option[] = q
    ? (search.data?.items ?? []).map(toOption)
    : (roots.data?.pages.flatMap((p) => p.items) ?? []).filter((r) => !r.deletedAt).map(toOption);

  // The current value keeps its label while a search no longer returns it.
  const selected = value ? (options.find((o) => o.id === value.id) ?? toOption(value)) : null;

  return (
    <Autocomplete
      id={id}
      size={size}
      sx={sx}
      options={options}
      value={selected}
      loading={search.isFetching || roots.isFetching}
      onChange={(_, next) => onChange(next ? { id: next.id, name: next.name, deleted: false } : null)}
      // Only typing searches; selecting shows the label without searching for it.
      onInputChange={(_, v, reason) => setInput(reason === 'input' ? v : '')}
      getOptionLabel={optionLabel}
      getOptionKey={(o) => o.id}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      // The server already filtered by the search term.
      filterOptions={(o) => o}
      noOptionsText={t('itemTypes.empty')}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label ?? t('itemTypes.pick')}
          placeholder={t('itemTypes.pickPlaceholder')}
          required={required}
          error={!!error}
          helperText={error}
        />
      )}
    />
  );
}
