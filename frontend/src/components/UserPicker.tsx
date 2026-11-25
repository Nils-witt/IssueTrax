import type { User } from '../api/types';

interface UserPickerProps {
  users: User[];
  value: string[];
  onChange: (ids: string[]) => void;
  label: string;
}

/** Checkbox list of users (only users who have signed in once exist). */
export function UserPicker({ users, value, onChange, label }: UserPickerProps) {
  return (
    <fieldset className="checks">
      <legend>{label}</legend>
      {users.map((u) => (
        <label key={u.id} title={u.email}>
          <input
            type="checkbox"
            checked={value.includes(u.id)}
            onChange={(e) => onChange(e.target.checked ? [...value, u.id] : value.filter((id) => id !== u.id))}
          />
          {u.displayName}
        </label>
      ))}
    </fieldset>
  );
}
