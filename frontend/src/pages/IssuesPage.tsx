import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import Chip from '@mui/material/Chip';
import FormControl from '@mui/material/FormControl';
import InputAdornment from '@mui/material/InputAdornment';
import InputLabel from '@mui/material/InputLabel';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import MenuItem from '@mui/material/MenuItem';
import OutlinedInput from '@mui/material/OutlinedInput';
import Paper from '@mui/material/Paper';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import ToggleButton from '@mui/material/ToggleButton';
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup';
import Typography from '@mui/material/Typography';
import AddIcon from '@mui/icons-material/Add';
import SearchIcon from '@mui/icons-material/Search';
import { useEffect, useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import { useIssues, useTags, useUsers } from '../api/hooks';
import { issueStatuses, type Issue, type MatchMode, type StorageItemRef } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { StatusBadge } from '../components/StatusBadge';
import { StorageItemPicker } from '../components/StorageItemPicker';
import { TagPicker } from '../components/TagPicker';
import { UserPicker } from '../components/UserPicker';
import { tagChipSx } from '../lib/tagColor';
import { IssueBulkBar } from './IssueBulkBar';

// The page's URL query uses the API's parameter names, so a filtered view
// is a bookmarkable URL and maps 1:1 onto GET /api/projects/{key}/issues.
const filterParams = ['status', 'tag', 'tagMode', 'assignee', 'assigneeMode', 'storageItem', 'storageItemMode', 'q'] as const;

export function IssuesPage() {
  const { t } = useTranslation();
  const key = useParam('key');
  const [params] = useSearchParams();

  const filterQuery = new URLSearchParams(
    [...params].filter(([name]) => (filterParams as readonly string[]).includes(name)),
  ).toString();

  const issues = useIssues(key, filterQuery);
  const items = issues.data?.pages.flatMap((p) => p.items) ?? [];
  const total = issues.data?.pages[0]?.total ?? 0;

  // Selected issue IDs. Only listed issues count as selected, so changing
  // the filters drops whatever is no longer shown.
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(new Set());
  const selected = items.filter((i) => selectedIds.has(i.id));

  return (
    <>
      <ProjectNav projectKey={key} />

      <Stack direction="row" spacing={2} sx={{ justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
        {/* Listed issues name their linked items, so a filtered item has a name to show. */}
        <FilterBar projectKey={key} knownItems={items.flatMap((i) => i.storageItems)} />
        <Button variant="contained" href={`/p/${key}/issues/new`} startIcon={<AddIcon />} sx={{ flexShrink: 0 }}>
          {t('issues.new')}
        </Button>
      </Stack>

      <ErrorMessage error={issues.error} />
      {issues.isPending && <Loading />}
      {issues.isSuccess && (
        <Stack spacing={2}>
          <IssueBulkBar
            projectKey={key}
            issues={items}
            selected={selected}
            total={total}
            onSelectAll={(all) => setSelectedIds(new Set(all ? items.map((i) => i.id) : []))}
          />
          {items.length === 0 ? (
            <Typography>{t('issues.empty')}</Typography>
          ) : (
            <IssueTable issues={items} selectedIds={selectedIds} onSelectedChange={setSelectedIds} />
          )}
          {issues.hasNextPage && (
            <Box>
              <Button variant="outlined" onClick={() => void issues.fetchNextPage()} loading={issues.isFetchingNextPage}>
                {t('app.loadMore')}
              </Button>
            </Box>
          )}
        </Stack>
      )}
    </>
  );
}

// Columns beyond key, title and status are hidden on narrow screens.
const wide = { display: { xs: 'none', md: 'flex' } } as const;

interface IssueTableProps {
  issues: Issue[];
  selectedIds: ReadonlySet<string>;
  onSelectedChange: (ids: ReadonlySet<string>) => void;
}

function IssueTable({ issues, selectedIds, onSelectedChange }: IssueTableProps) {
  const { t } = useTranslation();
  // Row last toggled without Shift, the anchor of Shift+click ranges.
  const anchor = useRef<number | null>(null);

  function toggle(index: number, checked: boolean, range: boolean) {
    const next = new Set(selectedIds);
    const from = range && anchor.current !== null ? Math.min(anchor.current, index) : index;
    const to = range && anchor.current !== null ? Math.max(anchor.current, index) : index;
    for (const issue of issues.slice(from, to + 1)) {
      if (checked) next.add(issue.id);
      else next.delete(issue.id);
    }
    if (!range) anchor.current = index;
    onSelectedChange(next);
  }

  return (
    <Paper variant="outlined">
      <List disablePadding>
        {issues.map((i, index) => (
          <ListItem key={i.id} disablePadding divider={index < issues.length - 1}>
            <Checkbox
              checked={selectedIds.has(i.id)}
              onClick={(e) => toggle(index, (e.target as HTMLInputElement).checked, e.shiftKey)}
              sx={{ ml: 1 }}
              slotProps={{ input: { 'aria-label': t('bulk.select', { key: i.key }) } }}
            />
            <ListItemButton
              selected={selectedIds.has(i.id)}
              href={`/p/${i.projectKey}/issues/${i.number}`}
              sx={{
                display: 'grid',
                gap: 1.5,
                alignItems: 'center',
                gridTemplateColumns: { xs: '6rem 1fr auto', md: '6rem 1fr auto auto 9rem auto' },
              }}
            >
              <KeyText>{i.key}</KeyText>
              <Typography sx={{ fontWeight: 500 }}>{i.title}</Typography>
              <StatusBadge status={i.status} />
              <Stack direction="row" sx={{ ...wide, flexWrap: 'wrap', gap: 0.5 }}>
                {i.tags.map((tag) => (
                  <Chip key={tag.id} size="small" label={tag.name} sx={tagChipSx(tag.color)} />
                ))}
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={wide}>
                {i.assignees.map((a) => a.displayName).join(', ') || t('issues.nobody')}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ ...wide, whiteSpace: 'nowrap' }}>
                <DateTime value={i.updatedAt} />
              </Typography>
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Paper>
  );
}

function FilterBar({ projectKey, knownItems }: { projectKey: string; knownItems: StorageItemRef[] }) {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const tags = useTags(projectKey);
  const users = useUsers();

  const [q, setQ] = useState(params.get('q') ?? '');

  function update(mutate: (p: URLSearchParams) => void) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        mutate(next);
        return next;
      },
      { replace: true },
    );
  }

  function setAll(name: string, values: string[]) {
    update((p) => {
      p.delete(name);
      for (const v of values) p.append(name, v);
    });
  }

  // Debounce the search box before touching the URL (and the API).
  useEffect(() => {
    const timer = setTimeout(() => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (q.trim()) next.set('q', q.trim());
          else next.delete('q');
          return next;
        },
        { replace: true },
      );
    }, 300);

    return () => clearTimeout(timer);
  }, [q, setParams]);

  const hasFilters = filterParams.some((name) => params.has(name));

  return (
    <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1.5, alignItems: 'center' }}>
      <TextField
        type="search"
        size="small"
        placeholder={t('filters.search')}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        sx={{ minWidth: '18rem' }}
        slotProps={{
          htmlInput: { maxLength: 200, 'aria-label': t('filters.search') },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          },
        }}
      />

      <MultiFilter
        label={t('filters.statuses')}
        title={t('filters.statusHint')}
        options={issueStatuses.map((s) => ({ value: s, label: t(`status.${s}`) }))}
        value={params.getAll('status')}
        onChange={(v) => setAll('status', v)}
      />

      {tags.data && tags.data.length > 0 && (
        <>
          <TagPicker
            id="tag-filter"
            size="small"
            label={t('filters.tags')}
            tags={tags.data}
            value={params.getAll('tag')}
            onChange={(v) => setAll('tag', v)}
            sx={{ minWidth: '14rem', maxWidth: '24rem' }}
          />
          <MatchToggle label={t('filters.tags')} value={params.get('tagMode')} onChange={(m) => update((p) => p.set('tagMode', m))} />
        </>
      )}

      {users.data && users.data.length > 0 && (
        <>
          <UserPicker
            id="assignee-filter"
            size="small"
            label={t('filters.assignees')}
            users={users.data}
            value={params.getAll('assignee')}
            onChange={(v) => setAll('assignee', v)}
            sx={{ minWidth: '14rem', maxWidth: '24rem' }}
          />
          <MatchToggle
            label={t('filters.assignees')}
            value={params.get('assigneeMode')}
            onChange={(m) => update((p) => p.set('assigneeMode', m))}
          />
        </>
      )}

      <StorageItemPicker
        id="storage-item-filter"
        size="small"
        label={t('filters.storageItems')}
        value={params.getAll('storageItem')}
        onChange={(v) => setAll('storageItem', v)}
        known={knownItems}
        sx={{ minWidth: '14rem', maxWidth: '24rem' }}
      />
      {params.getAll('storageItem').length > 1 && (
        <MatchToggle
          label={t('filters.storageItems')}
          value={params.get('storageItemMode')}
          onChange={(m) => update((p) => p.set('storageItemMode', m))}
        />
      )}

      {hasFilters && (
        <Button
          size="small"
          onClick={() => {
            setQ('');
            update((p) => filterParams.forEach((name) => p.delete(name)));
          }}
        >
          {t('filters.clear')}
        </Button>
      )}
    </Stack>
  );
}

interface MultiFilterProps {
  label: string;
  title?: string;
  options: { value: string; label: string }[];
  value: string[];
  onChange: (value: string[]) => void;
}

/** A compact multi-select with a checkbox per option. */
function MultiFilter({ label, title, options, value, onChange }: MultiFilterProps) {
  const labelId = useId();
  const labels = new Map(options.map((o) => [o.value, o.label]));

  return (
    <FormControl size="small" sx={{ minWidth: '10rem', maxWidth: '16rem' }} title={title}>
      <InputLabel id={labelId}>{label}</InputLabel>
      <Select
        multiple
        labelId={labelId}
        value={value}
        onChange={(e) => onChange(typeof e.target.value === 'string' ? e.target.value.split(',') : e.target.value)}
        input={<OutlinedInput label={label} />}
        renderValue={(selected) => selected.map((v) => labels.get(v) ?? v).join(', ')}
      >
        {options.map((o) => (
          <MenuItem key={o.value} value={o.value} dense>
            <Checkbox size="small" checked={value.includes(o.value)} sx={{ py: 0 }} />
            <ListItemText primary={o.label} />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

/** Whether all or any of the selected values must match. */
function MatchToggle({ label, value, onChange }: { label: string; value: string | null; onChange: (m: MatchMode) => void }) {
  const { t } = useTranslation();

  return (
    <ToggleButtonGroup
      size="small"
      exclusive
      value={value === 'any' ? 'any' : 'all'}
      onChange={(_, m: MatchMode | null) => m && onChange(m)}
      aria-label={`${label}: ${t('filters.match')}`}
    >
      <ToggleButton value="all" sx={{ py: 0.5 }}>
        {t('filters.matchAll')}
      </ToggleButton>
      <ToggleButton value="any" sx={{ py: 0.5 }}>
        {t('filters.matchAny')}
      </ToggleButton>
    </ToggleButtonGroup>
  );
}
