import Autocomplete from '@mui/material/Autocomplete';
import Chip from '@mui/material/Chip';
import TextField from '@mui/material/TextField';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Tag } from '../api/types';
import { tagChipSx } from '../lib/tagColor';
import { isValidTag, normalizeTag } from '../lib/tags';

interface TagInputProps {
  id: string;
  label: string;
  value: string[];
  onChange: (tags: string[]) => void;
  /** Existing tags of the project, offered as suggestions and used for chip colors. */
  tags: Tag[];
  error?: string;
  size?: 'small' | 'medium';
  autoFocus?: boolean;
}

/**
 * Chips plus a text field: pick an existing tag or type a new one and press
 * Enter or comma. New names are created by the server on save.
 */
export function TagInput({ id, label, value, onChange, tags, error, size, autoFocus }: TagInputProps) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');
  const suggestions = useMemo(() => tags.map((tag) => tag.name), [tags]);
  const colorOf = useMemo(() => new Map(tags.map((tag) => [tag.name, tag.color])), [tags]);

  const normalized = normalizeTag(draft);
  const draftInvalid = normalized !== '' && !isValidTag(normalized);

  function add(raw: string) {
    const name = normalizeTag(raw);
    if (!isValidTag(name)) return;

    if (!value.includes(name)) onChange([...value, name]);
    setDraft('');
  }

  return (
    <Autocomplete
      multiple
      freeSolo
      id={id}
      size={size}
      options={suggestions}
      filterSelectedOptions
      value={value}
      inputValue={draft}
      onInputChange={(_, v) => setDraft(v)}
      onChange={(_, next, reason) => {
        // A typed (not picked) entry is normalized and dropped if invalid,
        // which keeps the draft in the field with its error shown.
        if (reason === 'createOption') add(next[next.length - 1] ?? '');
        else onChange(next);
      }}
      onKeyDown={(e) => {
        if (e.key === ',') {
          e.preventDefault();
          add(draft);
        }
      }}
      renderValue={(names, getItemProps) =>
        names.map((name, index) => {
          const { key, ...itemProps } = getItemProps({ index });
          return (
            <Chip
              key={key}
              size="small"
              label={name}
              title={t('tags.remove', { name })}
              sx={tagChipSx(colorOf.get(name))}
              {...itemProps}
            />
          );
        })
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          autoFocus={autoFocus}
          placeholder={t('tags.placeholder')}
          error={draftInvalid || !!error}
          helperText={error ?? (draftInvalid ? t('fieldErrors.invalid') : undefined)}
          onBlur={() => draft && add(draft)}
        />
      )}
    />
  );
}
