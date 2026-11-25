import Autocomplete from '@mui/material/Autocomplete';
import Chip from '@mui/material/Chip';
import TextField from '@mui/material/TextField';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isValidTag, normalizeTag } from '../lib/tags';

interface TagInputProps {
  id: string;
  label: string;
  value: string[];
  onChange: (tags: string[]) => void;
  /** Existing tag names of the project, offered as suggestions. */
  suggestions: string[];
  error?: string;
}

/**
 * Chips plus a text field: pick an existing tag or type a new one and press
 * Enter or comma. New names are created by the server on save.
 */
export function TagInput({ id, label, value, onChange, suggestions, error }: TagInputProps) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');

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
      renderValue={(tags, getItemProps) =>
        tags.map((name, index) => {
          const { key, ...itemProps } = getItemProps({ index });
          return <Chip key={key} size="small" label={name} title={t('tags.remove', { name })} {...itemProps} />;
        })
      }
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={t('tags.placeholder')}
          error={draftInvalid || !!error}
          helperText={error ?? (draftInvalid ? t('fieldErrors.invalid') : undefined)}
          onBlur={() => draft && add(draft)}
        />
      )}
    />
  );
}
