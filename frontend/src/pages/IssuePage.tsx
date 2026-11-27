import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import IconButton from '@mui/material/IconButton';
import Popover from '@mui/material/Popover';
import EditIcon from '@mui/icons-material/Edit';
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined';
import { useState, type FormEvent, type ReactNode } from 'react';
import InventoryIcon from '@mui/icons-material/Inventory2Outlined';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useIssue, useTags, useUpdateIssue, useUsers } from '../api/hooks';
import { issueStatuses, type Issue, type IssueStatus } from '../api/types';
import { useParam } from '../app/params';
import { ProjectNav } from '../app/ProjectNav';
import { AttachmentSection } from '../components/AttachmentSection';
import { CommentSection } from '../components/CommentSection';
import { DateTime } from '../components/DateTime';
import { ErrorMessage } from '../components/ErrorMessage';
import { KeyText } from '../components/KeyText';
import { Loading } from '../components/Loading';
import { Markdown } from '../components/Markdown';
import { MarkdownEditor } from '../components/MarkdownEditor';
import { TagInput } from '../components/TagInput';
import { StorageItemPicker } from '../components/StorageItemPicker';
import { storageItemLabel } from '../lib/format';
import { UserPicker } from '../components/UserPicker';
import { fieldErrorText } from '../lib/errors';
import { tagChipSx } from '../lib/tagColor';

export function IssuePage() {
  const key = useParam('key');
  const number = Number(useParam('number'));
  const issue = useIssue(key, number);

  return (
    <>
      <ProjectNav projectKey={key} />
      <ErrorMessage error={issue.error} />
      {issue.isPending && <Loading />}
      {issue.data && (
        <>
          <IssueDetail key={issue.data.updatedAt} issue={issue.data} />
          {/* Outside the keyed IssueDetail so uploads and comment drafts survive issue edits. */}
          <Box sx={{ ...detailGrid, mt: 4 }}>
            <Stack spacing={4} sx={{ minWidth: 0 }}>
              <AttachmentSection issue={issue.data} />
              <CommentSection issue={issue.data} />
            </Stack>
          </Box>
        </>
      )}
    </>
  );
}

/** Main column plus metadata sidebar, shared by the issue and its attachments and comments. */
const detailGrid = { display: 'grid', gap: 4, gridTemplateColumns: { xs: '1fr', md: '1fr 16rem' } } as const;

function IssueDetail({ issue }: { issue: Issue }) {
  const { t } = useTranslation();
  const update = useUpdateIssue(issue.projectKey, issue.number);
  const archived = issue.status === 'archived';
  const fieldError = (field: string) => fieldErrorText(t, update.error, field);

  return (
    <Stack component="article" spacing={2}>
      {archived && (
        <Alert
          severity="warning"
          role="status"
          action={
            <Button color="inherit" size="small" onClick={() => update.mutate({ status: 'open' })} loading={update.isPending}>
              {t('issues.unarchive')}
            </Button>
          }
        >
          {t('issues.archivedBanner')}
        </Alert>
      )}

      <TitleEditor
        issue={issue}
        disabled={archived}
        pending={update.isPending}
        error={fieldError('title')}
        onSave={(title, done) => update.mutate({ title }, { onSuccess: done })}
      />

      <ErrorMessage error={update.error} />

      <Box sx={detailGrid}>
        <DescriptionEditor
          issue={issue}
          disabled={archived}
          pending={update.isPending}
          error={fieldError('description')}
          onSave={(description, done) => update.mutate({ description }, { onSuccess: done })}
        />

        <Stack component="aside" spacing={2.5}>
          <TextField
            select
            size="small"
            label={t('issues.status')}
            value={issue.status}
            disabled={update.isPending}
            onChange={(e) => update.mutate({ status: e.target.value as IssueStatus })}
          >
            {issueStatuses.map((s) => (
              <MenuItem key={s} value={s}>
                {t(`status.${s}`)}
              </MenuItem>
            ))}
          </TextField>

          <TagSection issue={issue} disabled={archived} error={fieldError('tags')} onSave={(names) => update.mutate({ tags: names })} />

          <AssigneeSection
            issue={issue}
            disabled={archived}
            error={fieldError('assignees')}
            onSave={(ids) => update.mutate({ assignees: ids })}
          />

          <StorageItemSection
            issue={issue}
            disabled={archived}
            error={fieldError('storageItems')}
            onSave={(ids) => update.mutate({ storageItems: ids })}
          />

          <Typography variant="body2" color="text.secondary">
            {t('issues.createdBy', { name: issue.createdBy.displayName || '?' })}
            <br />
            {t('issues.created')} <DateTime value={issue.createdAt} />
            <br />
            {t('issues.updated')} <DateTime value={issue.updatedAt} />
          </Typography>
        </Stack>
      </Box>
    </Stack>
  );
}

interface InlineEditorProps {
  issue: Issue;
  disabled: boolean;
  pending: boolean;
  error: string | undefined;
  /** Saves the value; `done` leaves edit mode once it succeeded. */
  onSave: (value: string, done: () => void) => void;
}

/** The title as a heading with an edit button that swaps it for a text field. */
function TitleEditor({ issue, disabled, pending, error, onSave }: InlineEditorProps) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(issue.title);

  function cancel() {
    setTitle(issue.title);
    setEditing(false);
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    const next = title.trim();
    if (next === issue.title) cancel();
    else onSave(next, () => setEditing(false));
  }

  if (editing) {
    return (
      <Stack component="form" direction="row" spacing={1} onSubmit={submit} sx={{ alignItems: 'flex-start' }}>
        <Typography variant="h5" component="span" sx={{ pt: 0.25 }}>
          <KeyText>{issue.key}</KeyText>
        </Typography>
        <TextField
          size="small"
          fullWidth
          autoFocus
          required
          aria-label={t('issues.title')}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => e.key === 'Escape' && cancel()}
          error={!!error}
          helperText={error}
          slotProps={{ htmlInput: { maxLength: 200 } }}
        />
        <Button type="submit" variant="contained" loading={pending} disabled={!title.trim()}>
          {t('app.save')}
        </Button>
        <Button onClick={cancel}>{t('app.cancel')}</Button>
      </Stack>
    );
  }

  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
      <Typography variant="h5" component="h2" sx={{ flex: 1 }}>
        <KeyText>{issue.key}</KeyText> {issue.title}
      </Typography>
      {!disabled && (
        <IconButton aria-label={t('issues.editTitle')} title={t('issues.editTitle')} onClick={() => setEditing(true)}>
          <EditIcon fontSize="small" />
        </IconButton>
      )}
    </Stack>
  );
}

/** The rendered description with an edit button that swaps it for the Markdown editor. */
function DescriptionEditor({ issue, disabled, pending, error, onSave }: InlineEditorProps) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [description, setDescription] = useState(issue.description);

  function cancel() {
    setDescription(issue.description);
    setEditing(false);
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (description === issue.description) cancel();
    else onSave(description, () => setEditing(false));
  }

  if (editing) {
    return (
      <Stack component="form" spacing={1.5} onSubmit={submit} sx={{ minWidth: 0 }}>
        <MarkdownEditor
          id="description"
          label={t('issues.description')}
          value={description}
          onChange={setDescription}
          error={error}
        />
        <Stack direction="row" spacing={1}>
          <Button type="submit" variant="contained" loading={pending}>
            {t('app.save')}
          </Button>
          <Button onClick={cancel}>{t('app.cancel')}</Button>
        </Stack>
      </Stack>
    );
  }

  return (
    <Box component="section" sx={{ minWidth: 0 }}>
      <MetaSection
        title={t('issues.description')}
        action={
          !disabled && (
            <IconButton size="small" aria-label={t('issues.editDescription')} title={t('issues.editDescription')} onClick={() => setEditing(true)}>
              <EditIcon fontSize="small" />
            </IconButton>
          )
        }
      >
        {issue.description ? <Markdown>{issue.description}</Markdown> : <Typography color="text.secondary">{t('issues.noDescription')}</Typography>}
      </MetaSection>
    </Box>
  );
}

interface SectionProps {
  issue: Issue;
  disabled: boolean;
  error: string | undefined;
  onSave: (value: string[]) => void;
}

/** The issue's tags as linked chips, GitHub-label style, edited in a popover. */
function TagSection({ issue, disabled, error, onSave }: SectionProps) {
  const { t } = useTranslation();
  const tags = useTags(issue.projectKey);

  return (
    <EditableSection
      title={t('issues.tags')}
      editLabel={t('tags.edit')}
      saved={issue.tags.map((tag) => tag.name)}
      disabled={disabled}
      error={error}
      onSave={onSave}
      editor={(value, onChange) => (
        <TagInput id="issue-tags" size="small" label={t('issues.tags')} value={value} onChange={onChange} tags={tags.data ?? []} autoFocus />
      )}
    >
      {issue.tags.length === 0 ? (
        <Typography color="text.secondary">—</Typography>
      ) : (
        <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5 }}>
          {issue.tags.map((tag) => (
            <Chip
              key={tag.id}
              size="small"
              label={tag.name}
              sx={tagChipSx(tag.color)}
              component={Link}
              to={`/p/${issue.projectKey}?tag=${tag.id}`}
              clickable
            />
          ))}
        </Stack>
      )}
    </EditableSection>
  );
}

/** The issue's assignees, one per line, edited in a popover. */
function AssigneeSection({ issue, disabled, error, onSave }: SectionProps) {
  const { t } = useTranslation();
  const users = useUsers();

  return (
    <EditableSection
      title={t('issues.assignees')}
      editLabel={t('issues.editAssignees')}
      saved={issue.assignees.map((a) => a.id)}
      disabled={disabled}
      error={error}
      onSave={onSave}
      editor={(value, onChange) =>
        users.data ? (
          <UserPicker
            id="issue-assignees"
            size="small"
            label={t('issues.assignees')}
            users={users.data}
            value={value}
            onChange={onChange}
            autoFocus
          />
        ) : (
          <Loading />
        )
      }
    >
      {issue.assignees.length === 0 ? (
        <Typography color="text.secondary">{t('issues.nobody')}</Typography>
      ) : (
        <Stack spacing={0.5}>
          {issue.assignees.map((a) => (
            <Typography key={a.id} title={a.email}>
              {a.displayName}
            </Typography>
          ))}
        </Stack>
      )}
    </EditableSection>
  );
}

/** The storage items the issue concerns, linking to their storage, edited in a popover. */
function StorageItemSection({ issue, disabled, error, onSave }: SectionProps) {
  const { t } = useTranslation();

  return (
    <EditableSection
      title={t('issues.storageItems')}
      editLabel={t('issues.editStorageItems')}
      saved={issue.storageItems.map((i) => i.id)}
      disabled={disabled}
      error={error}
      onSave={onSave}
      editor={(value, onChange) => (
        <StorageItemPicker
          id="issue-storage-items"
          size="small"
          label={t('issues.storageItems')}
          value={value}
          onChange={onChange}
          known={issue.storageItems}
          autoFocus
        />
      )}
    >
      {issue.storageItems.length === 0 ? (
        <Typography color="text.secondary">—</Typography>
      ) : (
        <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5 }}>
          {issue.storageItems.map((item) => (
            <Chip
              key={item.id}
              size="small"
              variant="outlined"
              icon={<InventoryIcon fontSize="small" />}
              label={storageItemLabel(item)}
              title={item.deleted ? t('storageItems.deleted') : undefined}
              sx={item.deleted ? { textDecoration: 'line-through', opacity: 0.7 } : undefined}
              component={Link}
              to={`/storages?storage=${encodeURIComponent(item.storageId)}`}
              clickable
            />
          ))}
        </Stack>
      )}
    </EditableSection>
  );
}

interface EditableSectionProps {
  title: string;
  editLabel: string;
  /** The saved value, a set of IDs or names. */
  saved: string[];
  disabled: boolean;
  error: string | undefined;
  onSave: (value: string[]) => void;
  /** The picker shown in the popover. */
  editor: (value: string[], onChange: (value: string[]) => void) => ReactNode;
  /** The read-only display of the saved value. */
  children: ReactNode;
}

/**
 * A sidebar section like GitHub's: a heading with a gear that opens a popover
 * to edit the value, saved once when the popover closes (if it changed).
 */
function EditableSection({ title, editLabel, saved, disabled, error, onSave, editor, children }: EditableSectionProps) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [draft, setDraft] = useState(saved);

  function close() {
    setAnchor(null);
    if (!sameSet(draft, saved)) onSave(draft);
  }

  return (
    <MetaSection
      title={title}
      action={
        !disabled && (
          <IconButton size="small" aria-label={editLabel} title={editLabel} onClick={(e) => setAnchor(e.currentTarget)}>
            <SettingsOutlinedIcon fontSize="small" />
          </IconButton>
        )
      }
    >
      {children}
      {error && (
        <Typography variant="body2" color="error" sx={{ mt: 0.5 }}>
          {error}
        </Typography>
      )}

      <Popover
        open={anchor !== null}
        anchorEl={anchor}
        onClose={close}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Box sx={{ p: 2, width: '20rem' }}>{editor(draft, setDraft)}</Box>
      </Popover>
    </MetaSection>
  );
}

function MetaSection({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <Box>
      <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="overline" component="h3" color="text.secondary" sx={{ lineHeight: 2 }}>
          {title}
        </Typography>
        {action}
      </Stack>
      {children}
    </Box>
  );
}

function sameSet(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((x) => b.includes(x));
}
