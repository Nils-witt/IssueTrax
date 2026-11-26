import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import FormControl from '@mui/material/FormControl';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Popover from '@mui/material/Popover';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useId, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useBulkUpdateIssues, useTags, useUsers, type BulkIssueUpdate } from '../api/hooks';
import { issueStatuses, type Issue, type IssueStatus, type UpdateIssueInput } from '../api/types';
import { TagPicker } from '../components/TagPicker';
import { UserPicker } from '../components/UserPicker';
import { errorText } from '../lib/errors';

interface IssueBulkBarProps {
  projectKey: string;
  /** The issues currently listed. */
  issues: Issue[];
  /** The selected ones among them. */
  selected: Issue[];
  /** Matching issues across all pages, shown while nothing is selected. */
  total: number;
  onSelectAll: (all: boolean) => void;
}

/** Select-all checkbox plus actions applied to every selected issue. */
export function IssueBulkBar({ projectKey, issues, selected, total, onSelectAll }: IssueBulkBarProps) {
  const { t } = useTranslation();
  const bulk = useBulkUpdateIssues(projectKey);
  const tags = useTags(projectKey);
  const users = useUsers();
  const statusLabelId = useId();

  const count = selected.length;
  const all = count > 0 && count === issues.length;
  const busy = bulk.isPending;

  /** Applies change to each selected issue, skipping those it leaves unchanged. */
  function apply(change: (issue: Issue) => UpdateIssueInput | null) {
    const updates: BulkIssueUpdate[] = [];
    for (const issue of selected) {
      const input = change(issue);
      if (input) updates.push({ number: issue.number, input });
    }
    if (updates.length > 0) bulk.mutate(updates);
  }

  function setStatus(status: IssueStatus) {
    apply((i) => (i.status === status ? null : { status }));
  }

  function editTags(ids: string[], add: boolean) {
    const names = (tags.data ?? []).filter((tag) => ids.includes(tag.id)).map((tag) => tag.name);
    apply((i) => {
      const current = i.tags.map((tag) => tag.name);
      const next = add ? [...new Set([...current, ...names])] : current.filter((n) => !names.includes(n));
      return next.length === current.length ? null : { tags: next };
    });
  }

  function editAssignees(ids: string[], add: boolean) {
    apply((i) => {
      const current = i.assignees.map((u) => u.id);
      const next = add ? [...new Set([...current, ...ids])] : current.filter((id) => !ids.includes(id));
      return next.length === current.length ? null : { assignees: next };
    });
  }

  const failed = bulk.data?.failed ?? [];

  return (
    <Stack spacing={1}>
      <Paper variant="outlined" sx={{ px: 1, py: 0.5 }}>
        <Stack direction="row" sx={{ alignItems: 'center', flexWrap: 'wrap', gap: 1.5, minHeight: '2.75rem' }}>
          <Checkbox
            checked={all}
            indeterminate={count > 0 && !all}
            onChange={(e) => onSelectAll(e.target.checked)}
            slotProps={{ input: { 'aria-label': t('bulk.selectAll') } }}
          />
          <Typography color="text.secondary">
            {count > 0 ? t('bulk.selected', { count }) : t('issues.count', { count: total })}
          </Typography>

          {count > 0 && (
            <>
              <FormControl size="small" sx={{ minWidth: '10rem' }} disabled={busy}>
                <InputLabel id={statusLabelId}>{t('bulk.setStatus')}</InputLabel>
                <Select
                  labelId={statusLabelId}
                  label={t('bulk.setStatus')}
                  value=""
                  onChange={(e) => setStatus(e.target.value as IssueStatus)}
                >
                  {issueStatuses.map((s) => (
                    <MenuItem key={s} value={s} dense>
                      {t(`status.${s}`)}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              {tags.data && tags.data.length > 0 && (
                <PickerAction label={t('issues.tags')} disabled={busy}>
                  {(value, setValue) => ({
                    picker: (
                      <TagPicker
                        id="bulk-tags"
                        size="small"
                        label={t('issues.tags')}
                        tags={tags.data}
                        value={value}
                        onChange={setValue}
                        sx={{ width: '20rem' }}
                      />
                    ),
                    onAdd: () => editTags(value, true),
                    onRemove: () => editTags(value, false),
                  })}
                </PickerAction>
              )}

              {users.data && users.data.length > 0 && (
                <PickerAction label={t('issues.assignees')} disabled={busy}>
                  {(value, setValue) => ({
                    picker: (
                      <UserPicker
                        id="bulk-assignees"
                        size="small"
                        label={t('issues.assignees')}
                        users={users.data}
                        value={value}
                        onChange={setValue}
                        sx={{ width: '20rem' }}
                      />
                    ),
                    onAdd: () => editAssignees(value, true),
                    onRemove: () => editAssignees(value, false),
                  })}
                </PickerAction>
              )}

              <Button size="small" onClick={() => onSelectAll(false)} disabled={busy}>
                {t('bulk.clear')}
              </Button>
              {busy && <Typography variant="body2" color="text.secondary">{t('app.saving')}</Typography>}
            </>
          )}
        </Stack>
      </Paper>

      {failed.length > 0 && (
        <Alert severity="warning" onClose={() => bulk.reset()}>
          {t('bulk.failed', { count: failed.length })}
          <ul style={{ margin: 0, paddingLeft: '1.25rem' }}>
            {failed.map((f) => (
              <li key={f.number}>
                {projectKey.toUpperCase()}-{f.number}: {errorText(t, f.error)}
              </li>
            ))}
          </ul>
        </Alert>
      )}
    </Stack>
  );
}

interface PickerActionProps {
  label: string;
  disabled: boolean;
  children: (
    value: string[],
    setValue: (v: string[]) => void,
  ) => { picker: ReactNode; onAdd: () => void; onRemove: () => void };
}

/** A button opening a popover with a picker and "Add" / "Remove" buttons. */
function PickerAction({ label, disabled, children }: PickerActionProps) {
  const { t } = useTranslation();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [value, setValue] = useState<string[]>([]);
  const { picker, onAdd, onRemove } = children(value, setValue);

  function close() {
    setAnchor(null);
    setValue([]);
  }

  return (
    <>
      <Button size="small" variant="outlined" disabled={disabled} onClick={(e) => setAnchor(e.currentTarget)}>
        {label}
      </Button>
      <Popover
        open={anchor !== null}
        anchorEl={anchor}
        onClose={close}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
      >
        <Stack spacing={1.5} sx={{ p: 2 }}>
          {picker}
          <Stack direction="row" spacing={1} sx={{ justifyContent: 'flex-end' }}>
            <Button
              size="small"
              disabled={value.length === 0}
              onClick={() => {
                onRemove();
                close();
              }}
            >
              {t('bulk.remove')}
            </Button>
            <Button
              size="small"
              variant="contained"
              disabled={value.length === 0}
              onClick={() => {
                onAdd();
                close();
              }}
            >
              {t('bulk.add')}
            </Button>
          </Stack>
        </Stack>
      </Popover>
    </>
  );
}
