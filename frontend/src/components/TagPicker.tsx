import Autocomplete from '@mui/material/Autocomplete';
import Chip from '@mui/material/Chip';
import type { SxProps, Theme } from '@mui/material/styles';
import TextField from '@mui/material/TextField';
import type { Tag } from '../api/types';

interface TagPickerProps {
  id: string;
  tags: Tag[];
  /** Selected tag IDs. */
  value: string[];
  onChange: (ids: string[]) => void;
  label: string;
  size?: 'small' | 'medium';
  sx?: SxProps<Theme>;
}

/**
 * Multi-select autocomplete of a project's existing tags, by ID. Unlike
 * TagInput it cannot create new tags, which suits filtering.
 */
export function TagPicker({ id, tags, value, onChange, label, size, sx }: TagPickerProps) {
  const selected = tags.filter((tag) => value.includes(tag.id));

  return (
    <Autocomplete
      multiple
      id={id}
      size={size}
      sx={sx}
      options={tags}
      value={selected}
      onChange={(_, next) => onChange(next.map((tag) => tag.id))}
      getOptionLabel={(tag) => tag.name}
      getOptionKey={(tag) => tag.id}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      filterSelectedOptions
      renderValue={(tags, getItemProps) =>
        tags.map((tag, index) => {
          const { key, ...itemProps } = getItemProps({ index });
          return <Chip key={key} size="small" label={tag.name} {...itemProps} />;
        })
      }
      renderInput={(params) => <TextField {...params} label={label} />}
    />
  );
}
