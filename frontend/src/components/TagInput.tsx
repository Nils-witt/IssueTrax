import { useId, useState, type KeyboardEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { isValidTag, normalizeTag } from '../lib/tags';

interface TagInputProps {
  id: string;
  value: string[];
  onChange: (tags: string[]) => void;
  /** Existing tag names of the project, offered as suggestions. */
  suggestions: string[];
}

/**
 * Chips plus a text field: pick an existing tag or type a new one and press
 * Enter or comma. New names are created by the server on save.
 */
export function TagInput({ id, value, onChange, suggestions }: TagInputProps) {
  const { t } = useTranslation();
  const listId = useId();
  const [draft, setDraft] = useState('');

  const normalized = normalizeTag(draft);
  const draftInvalid = normalized !== '' && !isValidTag(normalized);

  function add(raw: string) {
    const name = normalizeTag(raw);
    if (!isValidTag(name)) return;

    if (!value.includes(name)) onChange([...value, name]);
    setDraft('');
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      add(draft);
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  }

  return (
    <div className="tag-input">
      <ul className="chips">
        {value.map((name) => (
          <li key={name} className="chip">
            {name}
            <button
              type="button"
              aria-label={t('tags.remove', { name })}
              onClick={() => onChange(value.filter((v) => v !== name))}
            >
              ×
            </button>
          </li>
        ))}
      </ul>
      <input
        id={id}
        list={listId}
        value={draft}
        placeholder={t('tags.placeholder')}
        aria-invalid={draftInvalid}
        onChange={(e) => {
          // Picking a datalist entry replaces the value with a full suggestion.
          if (suggestions.includes(e.target.value)) add(e.target.value);
          else setDraft(e.target.value);
        }}
        onKeyDown={onKeyDown}
        onBlur={() => draft && add(draft)}
      />
      <datalist id={listId}>
        {suggestions
          .filter((s) => !value.includes(s))
          .map((s) => (
            <option key={s} value={s} />
          ))}
      </datalist>
    </div>
  );
}
