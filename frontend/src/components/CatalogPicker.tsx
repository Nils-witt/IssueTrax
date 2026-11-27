import Autocomplete from '@mui/material/Autocomplete';
import TextField from '@mui/material/TextField';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useCreateHardwareModel, useCreateManufacturer, useHardwareModels, useManufacturers } from '../api/hooks';
import type { CatalogRef } from '../api/types';
import { errorText } from '../lib/errors';

/** A pickable entry, or the offer to add one called create. */
type Option = CatalogRef & { create?: string };

type CatalogPickerProps = {
  id: string;
  value: CatalogRef | null;
  onChange: (next: CatalogRef | null) => void;
  label: string;
  error?: string;
} & ({ kind: 'manufacturer' } | { kind: 'model'; manufacturerId: string | null });

/**
 * Picks a live manufacturer, or a live model of one, from the central list.
 * Typing searches the server; a name that is not listed yet can be added
 * right away. An entry the item already has stays shown when deleted.
 */
export function CatalogPicker(props: CatalogPickerProps) {
  const { id, value, onChange, label, error, kind } = props;
  const manufacturerId = kind === 'model' ? props.manufacturerId : null;
  const { t } = useTranslation();
  const [input, setInput] = useState('');
  const [q, setQ] = useState('');
  const [open, setOpen] = useState(false);
  const makers = useManufacturers(q, false, open && kind === 'manufacturer');
  const models = useHardwareModels(manufacturerId, q, false, open && kind === 'model');
  const createMaker = useCreateManufacturer();
  const createModel = useCreateHardwareModel();
  const create = kind === 'manufacturer' ? createMaker : createModel;
  const list = kind === 'manufacturer' ? makers : models;
  const pages: { items: { id: string; name: string }[] }[] = (kind === 'manufacturer' ? makers.data?.pages : models.data?.pages) ?? [];
  const disabled = kind === 'model' && !manufacturerId;

  // Debounce typing before searching.
  useEffect(() => {
    const timer = setTimeout(() => setQ(input.trim()), 250);
    return () => clearTimeout(timer);
  }, [input]);

  const options: Option[] = pages.flatMap((p) => p.items).map((e) => ({ id: e.id, name: e.name, deleted: false }));
  const typed = input.trim();

  if (typed && !options.some((o) => o.name.toLowerCase() === typed.toLowerCase())) {
    options.push({ id: '', name: typed, deleted: false, create: typed });
  }

  function pick(next: Option | null) {
    if (!next?.create) {
      onChange(next && { id: next.id, name: next.name, deleted: next.deleted });
      return;
    }

    const done = (created: { id: string; name: string }) => onChange({ id: created.id, name: created.name, deleted: false });

    if (kind === 'manufacturer') createMaker.mutate(next.create, { onSuccess: done });
    else if (manufacturerId) createModel.mutate({ manufacturerId, name: next.create }, { onSuccess: done });
  }

  const nameOf = (o: Option) => (o.deleted ? t('catalog.deletedName', { name: o.name }) : o.name);
  const helper = error ?? (create.error ? errorText(t, create.error) : undefined);

  return (
    <Autocomplete<Option>
      id={id}
      size="small"
      options={options}
      value={value}
      disabled={disabled}
      open={open}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      loading={list.isFetching || create.isPending}
      onChange={(_, next) => pick(next)}
      // Only typing searches; selecting shows the label without searching for it.
      onInputChange={(_, v, reason) => setInput(reason === 'input' ? v : '')}
      getOptionLabel={(o) => (o.create ? t('catalog.add', { name: o.create }) : nameOf(o))}
      getOptionKey={(o) => o.id || `create:${o.create}`}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      // The server already filtered by the search term.
      filterOptions={(o) => o}
      noOptionsText={kind === 'manufacturer' ? t('catalog.noManufacturers') : t('catalog.noModels')}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={disabled ? t('catalog.pickManufacturerFirst') : t('catalog.searchOrAdd')}
          error={!!helper}
          helperText={helper}
        />
      )}
    />
  );
}
